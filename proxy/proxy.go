package proxy

import (
	"bytes"
	"context"
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

type proxyErrorKey struct{}

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
	ReqBody           []byte
	ReqBodyTruncated  bool
	Status            int
	RespBody          []byte
	RespBodyTruncated bool
	Error             string
	Latency           time.Duration
	IsWebhook         bool
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
			ResponseHeader:  http.Header{},
			StripeRequestID: "",
		}

		var reqCapture *captureReadCloser
		if r.Body != nil {
			reqCapture = &captureReadCloser{ReadCloser: r.Body, maxBodyBytes: cfg.maxBodyBytes}
			r.Body = reqCapture
		}
		var proxyErr string
		r = r.WithContext(context.WithValue(r.Context(), proxyErrorKey{}, &proxyErr))

		rw := &responseWriter{
			ResponseWriter: w,
			buf:            &bytes.Buffer{},
			maxBodyBytes:   cfg.maxBodyBytes,
		}
		rp.ServeHTTP(rw, r)

		if reqCapture != nil {
			call.ReqBody = reqCapture.buf.Bytes()
			call.ReqBodyTruncated = reqCapture.truncated
			if reqCapture.err != nil {
				call.Error = reqCapture.err.Error()
			}
		}
		call.Status = rw.status
		call.RespBody = rw.buf.Bytes()
		call.RespBodyTruncated = rw.truncated
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

func redactedHeader(h http.Header) http.Header {
	out := h.Clone()
	for k := range out {
		switch strings.ToLower(k) {
		case "authorization", "cookie", "set-cookie", "x-stripe-client-secret":
			out[k] = []string{"[redacted]"}
		}
	}
	return out
}

type captureReadCloser struct {
	io.ReadCloser
	buf          bytes.Buffer
	maxBodyBytes int64
	truncated    bool
	err          error
}

func (c *captureReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if n > 0 {
		remaining := c.maxBodyBytes - int64(c.buf.Len())
		switch {
		case remaining <= 0:
			c.truncated = true
		case int64(n) > remaining:
			c.buf.Write(p[:remaining])
			c.truncated = true
		default:
			c.buf.Write(p[:n])
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		c.err = err
	}
	return n, err
}

type responseWriter struct {
	http.ResponseWriter
	status       int
	buf          *bytes.Buffer
	maxBodyBytes int64
	truncated    bool
	err          string
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	remaining := rw.maxBodyBytes - int64(rw.buf.Len())
	switch {
	case remaining <= 0:
		rw.truncated = true
	case int64(len(b)) > remaining:
		rw.buf.Write(b[:remaining])
		rw.truncated = true
	default:
		rw.buf.Write(b)
	}
	return rw.ResponseWriter.Write(b)
}
