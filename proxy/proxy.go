package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const StripeAPI = "https://api.stripe.com"

// Call is a captured request/response pair.
type Call struct {
	Time      time.Time
	Method    string
	Path      string
	ReqBody   []byte
	Status    int
	RespBody  []byte
	Latency   time.Duration
	IsWebhook bool
}

// Handler returns an http.Handler that proxies to Stripe and sends each
// captured Call to the provided channel.
func Handler(calls chan<- Call) http.Handler {
	target, _ := url.Parse(StripeAPI)

	rp := httputil.NewSingleHostReverseProxy(target)

	// Rewrite the Host header so Stripe accepts the request.
	originalDirector := rp.Director
	rp.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Buffer the request body so we can log it and still forward it.
		var reqBody []byte
		if r.Body != nil {
			reqBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
			r.ContentLength = int64(len(reqBody))
		}

		rw := &responseWriter{ResponseWriter: w, buf: &bytes.Buffer{}}
		rp.ServeHTTP(rw, r)

		calls <- Call{
			Time:     start,
			Method:   r.Method,
			Path:     r.URL.Path,
			ReqBody:  reqBody,
			Status:   rw.status,
			RespBody: rw.buf.Bytes(),
			Latency:  time.Since(start),
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.buf.Write(b)
	return rw.ResponseWriter.Write(b)
}
