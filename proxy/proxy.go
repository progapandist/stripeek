package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const StripeAPI = "https://api.stripe.com"

const DefaultMaxBodyBytes int64 = 2 << 20

var sensitiveHeaders = map[string]struct{}{
	"authorization":          {},
	"cookie":                 {},
	"proxy-authorization":    {},
	"set-cookie":             {},
	"stripe-signature":       {},
	"x-stripe-client-secret": {},
}

type proxyErrorKey struct{}

// Group identifies a user-created debugging group assigned to captured calls.
type Group struct {
	ID        string
	Name      string
	Color     string
	LightHex  string
	DarkHex   string
	StartedAt time.Time
}

// Call is a captured request/response pair.
type Call struct {
	Time              time.Time
	Method            string
	Path              string
	RequestURI        string
	RequestHeader     http.Header
	ResponseHeader    http.Header
	StripeRequestID   string
	IdempotencyKey    string
	StripeAccount     string
	KeyMode           string // "test" or "live", inferred from the API key prefix
	ReqBody           []byte
	ReqBodyTruncated  bool
	Status            int
	RespBody          []byte
	RespBodyTruncated bool
	Error             string
	Latency           time.Duration
	IsWebhook         bool
	Group             *Group `json:",omitempty"`
}

type config struct {
	target       *url.URL
	maxBodyBytes int64
	transport    http.RoundTripper
}

// Option configures the proxy handler.
type Option func(*config)

// WithTarget forwards captured traffic to target instead of StripeAPI.
func WithTarget(target string) Option {
	return func(c *config) {
		u, err := url.Parse(target)
		if err == nil && u.Scheme != "" && u.Host != "" {
			c.target = u
		}
	}
}

// WithMaxBodyBytes caps the number of bytes captured for each body.
func WithMaxBodyBytes(n int64) Option {
	return func(c *config) {
		if n > 0 {
			c.maxBodyBytes = n
		}
	}
}

// WithTransport sets the upstream HTTP transport. It is mainly useful for tests.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *config) {
		if rt != nil {
			c.transport = rt
		}
	}
}

// Handler returns an http.Handler that proxies to Stripe and sends each
// captured Call to the provided channel.
func Handler(calls chan<- Call, opts ...Option) http.Handler {
	target, _ := url.Parse(StripeAPI)
	cfg := config{
		target:       target,
		maxBodyBytes: DefaultMaxBodyBytes,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	rp := httputil.NewSingleHostReverseProxy(cfg.target)
	if cfg.transport != nil {
		rp.Transport = cfg.transport
	}

	// Rewrite the Host header so Stripe accepts the request.
	originalDirector := rp.Director
	rp.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = cfg.target.Host
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if p, ok := r.Context().Value(proxyErrorKey{}).(*string); ok {
			*p = err.Error()
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(err.Error()))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		call := Call{
			Time:            start,
			Method:          r.Method,
			Path:            r.URL.Path,
			RequestURI:      r.URL.RequestURI(),
			RequestHeader:   redactedHeader(r.Header),
			IdempotencyKey:  r.Header.Get("Idempotency-Key"),
			StripeAccount:   r.Header.Get("Stripe-Account"),
			KeyMode:         keyMode(r.Header.Get("Authorization")),
			ResponseHeader:  http.Header{},
			StripeRequestID: "",
		}

		var reqCapture *captureReadCloser
		if r.Body != nil {
			reqCapture = &captureReadCloser{
				ReadCloser: r.Body,
				capture:    newBodyCapture(cfg.maxBodyBytes),
			}
			r.Body = reqCapture
		}
		var proxyErr string
		r = r.WithContext(context.WithValue(r.Context(), proxyErrorKey{}, &proxyErr))

		rw := &responseWriter{
			ResponseWriter: w,
			capture:        newBodyCapture(cfg.maxBodyBytes),
		}
		rp.ServeHTTP(rw, r)

		if reqCapture != nil {
			call.ReqBody = reqCapture.capture.Bytes()
			call.ReqBodyTruncated = reqCapture.capture.truncated
			if reqCapture.err != nil {
				call.Error = reqCapture.err.Error()
			}
		}
		call.Status = rw.status
		call.RespBody = rw.capture.Bytes()
		call.RespBodyTruncated = rw.capture.truncated
		call.ResponseHeader = redactedHeader(rw.Header())
		call.StripeRequestID = rw.Header().Get("Request-Id")
		call.Latency = time.Since(start)
		if rw.err != "" {
			call.Error = rw.err
		}
		if proxyErr != "" {
			call.Error = proxyErr
		}

		select {
		case calls <- call:
		default:
		}
	})
}

// keyMode infers "test" or "live" from a Stripe API key in the Authorization
// header without retaining the secret itself. Stripe accepts the key either as
// a bearer token ("Bearer sk_test_…") or as the HTTP Basic username
// ("Basic base64(sk_live_…:)"); the mode is the segment after the key kind
// (sk_<mode>_…, rk_<mode>_…). Returns "" when the mode can't be determined.
func keyMode(auth string) string {
	key := ""
	if rest, ok := strings.CutPrefix(auth, "Bearer "); ok {
		key = strings.TrimSpace(rest)
	} else if rest, ok := strings.CutPrefix(auth, "Basic "); ok {
		if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rest)); err == nil {
			key, _, _ = strings.Cut(string(dec), ":")
		}
	}
	parts := strings.SplitN(key, "_", 3)
	if len(parts) >= 2 && (parts[1] == "test" || parts[1] == "live") {
		return parts[1]
	}
	return ""
}

func redactedHeader(h http.Header) http.Header {
	out := h.Clone()
	for k := range out {
		if _, ok := sensitiveHeaders[strings.ToLower(k)]; ok {
			out[k] = []string{"[redacted]"}
		}
	}
	return out
}

type bodyCapture struct {
	buf          bytes.Buffer
	maxBodyBytes int64
	truncated    bool
}

func newBodyCapture(maxBodyBytes int64) bodyCapture {
	return bodyCapture{maxBodyBytes: maxBodyBytes}
}

func (c *bodyCapture) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	remaining := c.maxBodyBytes - int64(c.buf.Len())
	switch {
	case remaining <= 0:
		c.truncated = true
	case int64(len(p)) > remaining:
		c.buf.Write(p[:int(remaining)])
		c.truncated = true
	default:
		c.buf.Write(p)
	}
}

func (c *bodyCapture) Bytes() []byte {
	return c.buf.Bytes()
}

type captureReadCloser struct {
	io.ReadCloser
	capture bodyCapture
	err     error
}

func (c *captureReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if n > 0 {
		c.capture.Write(p[:n])
	}
	if err != nil && !errors.Is(err, io.EOF) {
		c.err = err
	}
	return n, err
}

type responseWriter struct {
	http.ResponseWriter
	status  int
	capture bodyCapture
	err     string
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	rw.capture.Write(b)
	n, err := rw.ResponseWriter.Write(b)
	if err != nil {
		rw.err = err.Error()
	}
	return n, err
}
