package context

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP_TrustedProxyPaths(t *testing.T) {
	c := New(httptest.NewRecorder(), nil)
	if c.ClientIP() != "" {
		t.Fatal("nil request")
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "9.9.9.9" // no port
	c = New(httptest.NewRecorder(), r)
	if c.ClientIP() != "9.9.9.9" {
		t.Fatal(c.ClientIP())
	}

	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "10.0.0.1:1234"
	r2.Header.Set("X-Forwarded-For", "1.1.1.1, 10.0.0.1")
	r2.Header.Set("X-Real-IP", "10.0.0.1")
	c2 := New(httptest.NewRecorder(), r2)
	c2.TrustedProxies = []string{"10.0.0.0/8", "", "not-an-ip", "10.0.0.1"}
	if ip := c2.ClientIP(); ip != "1.1.1.1" {
		t.Fatalf("xff walk want 1.1.1.1 got %s", ip)
	}

	r3 := httptest.NewRequest("GET", "/", nil)
	r3.RemoteAddr = "10.0.0.1:1"
	r3.Header.Set("X-Real-IP", "8.8.8.8")
	c3 := New(httptest.NewRecorder(), r3)
	c3.TrustedProxies = []string{"10.0.0.1"}
	if ip := c3.ClientIP(); ip != "8.8.8.8" {
		t.Fatalf("xri want 8.8.8.8 got %s", ip)
	}

	r4 := httptest.NewRequest("GET", "/", nil)
	r4.RemoteAddr = "10.0.0.1:1"
	r4.Header.Set("X-Forwarded-For", "10.0.0.2, 10.0.0.1")
	r4.Header.Set("X-Real-IP", "10.0.0.2")
	c4 := New(httptest.NewRecorder(), r4)
	c4.TrustedProxies = []string{"10.0.0.0/8"}
	if ip := c4.ClientIP(); ip != "10.0.0.1" {
		// all hops trusted → peer
		t.Fatalf("want peer got %s", ip)
	}

	_ = clientIPFromXFF(" , , ", []string{"1.1.1.1"})
	_ = clientIPFromXFF("1.1.1.1:80", nil)
	_ = isTrustedProxy("nope", []string{"1.1.1.1"})
	_ = isTrustedProxy("1.1.1.1", nil)
}
