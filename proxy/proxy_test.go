package proxy

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerCapturesMetadataAndCapsBodiesWithoutTruncatingUpstream(t *testing.T) {
	var upstreamBody string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		upstreamBody = string(b)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Request-Id": []string{"req_123"},
				"Set-Cookie": []string{"session=secret"},
			},
			Body:    io.NopCloser(strings.NewReader("response-body")),
			Request: r,
		}, nil
	})

	calls := make(chan Call, 1)
	handler := Handler(calls, WithTarget("https://api.stripe.test"), WithMaxBodyBytes(4), WithTransport(rt))

	req := httptest.NewRequest(http.MethodPost, "http://stripeek.test/v1/customers?limit=3", strings.NewReader("abcdefghij"))
	req.Header.Set("Authorization", "Bearer sk_test_secret")
	req.Header.Set("Proxy-Authorization", "Basic secret")
	req.Header.Set("Idempotency-Key", "idem_123")
	req.Header.Set("Stripe-Account", "acct_123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if upstreamBody != "abcdefghij" {
		t.Fatalf("upstream body = %q, want full original body", upstreamBody)
	}

	call := <-calls
	if call.RequestURI != "/v1/customers?limit=3" {
		t.Errorf("RequestURI = %q", call.RequestURI)
	}
	if call.IdempotencyKey != "idem_123" {
		t.Errorf("IdempotencyKey = %q", call.IdempotencyKey)
	}
	if call.StripeAccount != "acct_123" {
		t.Errorf("StripeAccount = %q", call.StripeAccount)
	}
	if call.StripeRequestID != "req_123" {
		t.Errorf("StripeRequestID = %q", call.StripeRequestID)
	}
	if got := call.RequestHeader.Get("Authorization"); got != "[redacted]" {
		t.Errorf("captured Authorization = %q", got)
	}
	if got := call.RequestHeader.Get("Proxy-Authorization"); got != "[redacted]" {
		t.Errorf("captured Proxy-Authorization = %q", got)
	}
	if got := call.ResponseHeader.Get("Set-Cookie"); got != "[redacted]" {
		t.Errorf("captured Set-Cookie = %q", got)
	}
	if string(call.ReqBody) != "abcd" || !call.ReqBodyTruncated {
		t.Errorf("request capture = %q truncated=%v", call.ReqBody, call.ReqBodyTruncated)
	}
	if string(call.RespBody) != "resp" || !call.RespBodyTruncated {
		t.Errorf("response capture = %q truncated=%v", call.RespBody, call.RespBodyTruncated)
	}
}

func TestHandlerDoesNotBlockWhenCallChannelIsFull(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    r,
		}, nil
	})

	calls := make(chan Call)
	handler := Handler(calls, WithTarget("https://api.stripe.test"), WithTransport(rt))

	req := httptest.NewRequest(http.MethodGet, "http://stripeek.test/v1/customers", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandlerCapturesUpstreamErrors(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("upstream unavailable")
	})

	calls := make(chan Call, 1)
	handler := Handler(calls, WithTarget("https://api.stripe.test"), WithTransport(rt))

	req := httptest.NewRequest(http.MethodGet, "http://stripeek.test/v1/customers", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
	call := <-calls
	if call.Status != http.StatusBadGateway {
		t.Fatalf("call status = %d, want %d", call.Status, http.StatusBadGateway)
	}
	if call.Error != "upstream unavailable" {
		t.Fatalf("call error = %q", call.Error)
	}
	if string(call.RespBody) != "upstream unavailable" {
		t.Fatalf("captured response body = %q", call.RespBody)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestHandlerInfersKeyMode(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
	})

	cases := []struct {
		name string
		auth string
		want string
	}{
		{"bearer test", "Bearer sk_test_abc123", "test"},
		{"bearer live", "Bearer sk_live_abc123", "live"},
		{"restricted live", "Bearer rk_live_abc123", "live"},
		{"basic test", "Basic " + base64.StdEncoding.EncodeToString([]byte("sk_test_abc:")), "test"},
		{"unknown", "Bearer something_else", ""},
		{"absent", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := make(chan Call, 1)
			handler := Handler(calls, WithTarget("https://api.stripe.test"), WithTransport(rt))
			req := httptest.NewRequest(http.MethodGet, "http://stripeek.test/v1/customers", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)
			if got := (<-calls).KeyMode; got != tc.want {
				t.Fatalf("KeyMode = %q, want %q", got, tc.want)
			}
		})
	}
}
