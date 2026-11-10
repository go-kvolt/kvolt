package context

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_IgnoresSpoofedLeftmostWhenProxyAppends(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:443"
	// Client sent "evil"; trusted proxy appended real client 203.0.113.10
	req.Header.Set("X-Forwarded-For", "evil, 203.0.113.10")
	c := New(httptest.NewRecorder(), req)
	c.TrustedProxies = []string{"10.0.0.0/8"}
	if got := c.ClientIP(); got != "203.0.113.10" {
		t.Fatalf("got %q want 203.0.113.10", got)
	}
}

func TestClientIP_UntrustedPeerIgnoresXFF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	c := New(httptest.NewRecorder(), req)
	if got := c.ClientIP(); got != "203.0.113.9" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIP_RejectsTrustedXRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:443"
	req.Header.Set("X-Real-IP", "10.0.0.9") // another trusted hop — not a client
	c := New(httptest.NewRecorder(), req)
	c.TrustedProxies = []string{"10.0.0.0/8"}
	if got := c.ClientIP(); got != "10.0.0.5" {
		t.Fatalf("got %q want peer 10.0.0.5", got)
	}
}
