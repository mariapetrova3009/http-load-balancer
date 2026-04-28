package balancer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return u
}

func newBackendServer(name string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(name))
	}))
}

func doRequest(t *testing.T, h http.Handler) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://lb.local/api/test", nil)
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestLoadBalancer_RoundRobin(t *testing.T) {
	s1 := newBackendServer("b1")
	t.Cleanup(s1.Close)
	s2 := newBackendServer("b2")
	t.Cleanup(s2.Close)
	s3 := newBackendServer("b3")
	t.Cleanup(s3.Close)

	lb := New([]*url.URL{
		mustParseURL(t, s1.URL),
		mustParseURL(t, s2.URL),
		mustParseURL(t, s3.URL),
	})
	lb.SetMaxRetries(0)

	_, b := doRequest(t, lb)
	if b != "b1" {
		t.Fatalf("expected b1, got %q", b)
	}
	_, b = doRequest(t, lb)
	if b != "b2" {
		t.Fatalf("expected b2, got %q", b)
	}
	_, b = doRequest(t, lb)
	if b != "b3" {
		t.Fatalf("expected b3, got %q", b)
	}
}

func TestLoadBalancer_SkipDeadBackend(t *testing.T) {
	s1 := newBackendServer("b1")
	t.Cleanup(s1.Close)
	s2 := newBackendServer("b2")
	t.Cleanup(s2.Close)

	lb := New([]*url.URL{
		mustParseURL(t, s1.URL),
		mustParseURL(t, s2.URL),
	})
	lb.SetMaxRetries(0)


	lb.Backends()[0].SetAlive(false, "dead for test")

	_, b := doRequest(t, lb)
	if b != "b2" {
		t.Fatalf("expected b2, got %q", b)
	}
}

func TestLoadBalancer_RetryFailoverOnProxyError_GET(t *testing.T) {
	s1 := newBackendServer("b1")
	s2 := newBackendServer("b2")
	t.Cleanup(s2.Close)

	// Kill the first backend (but keep it "alive" initially) so the balancer
	// hits it first and then fails over on proxy error.
	s1.Close()

	lb := New([]*url.URL{
		mustParseURL(t, s1.URL),
		mustParseURL(t, s2.URL),
	})
	lb.SetMaxRetries(1)

	code, body := doRequest(t, lb)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", code, body)
	}
	if body != "b2" {
		t.Fatalf("expected b2 after failover, got %q", body)
	}

	if lb.Backends()[0].IsAlive() {
		t.Fatalf("expected backend-1 to be marked dead after proxy error")
	}
}

