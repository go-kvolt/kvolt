package middleware

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestRequestID_SanitizesInjectedHeader(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "abc\r\nX-Injected: evil")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "OK") }}
	RequestID()(c)
	got := w.Header().Get("X-Request-ID")
	if got == "" || strings.ContainsAny(got, "\r\n ") || strings.Contains(got, "Injected") {
		t.Fatalf("unsanitized request id: %q", got)
	}
}

func TestSecure_HSTSRequiresTLSOrTrustedProxy(t *testing.T) {
	cfg := SecureConfig{HSTSMaxAge: 31536000, HSTSIfXForwardedProto: true}

	// Plain HTTP + spoofed XFP without trusted proxy → no HSTS
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.RemoteAddr = "203.0.113.9:1234"
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return nil }}
	SecureWithConfig(cfg)(c)
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS must not set from untrusted X-Forwarded-Proto")
	}

	// Trusted proxy + XFP https → HSTS
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("X-Forwarded-Proto", "https")
	r2.RemoteAddr = "10.0.0.1:9999"
	c2 := context.New(w2, r2)
	c2.TrustedProxies = []string{"10.0.0.0/8"}
	c2.Handlers = []context.HandlerFunc{func(c *context.Context) error { return nil }}
	SecureWithConfig(cfg)(c2)
	if !strings.Contains(w2.Header().Get("Strict-Transport-Security"), "max-age=") {
		t.Fatal("expected HSTS behind trusted proxy")
	}

	// Direct TLS → HSTS
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest("GET", "/", nil)
	r3.TLS = &tls.ConnectionState{}
	c3 := context.New(w3, r3)
	c3.Handlers = []context.HandlerFunc{func(c *context.Context) error { return nil }}
	SecureWithConfig(cfg)(c3)
	if !strings.Contains(w3.Header().Get("Strict-Transport-Security"), "max-age=") {
		t.Fatal("expected HSTS on TLS")
	}
}

func TestBasicAuth_AcceptsValidRejectsInvalid(t *testing.T) {
	mw := BasicAuth(map[string]string{"alice": "s3cret"})

	okReq := httptest.NewRequest("GET", "/", nil)
	okReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:s3cret")))
	wok := httptest.NewRecorder()
	cok := context.New(wok, okReq)
	cok.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
	cok.Next()
	if wok.Code != 200 {
		t.Fatalf("want 200, got %d", wok.Code)
	}

	badReq := httptest.NewRequest("GET", "/", nil)
	badReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:wrong")))
	wbad := httptest.NewRecorder()
	cbad := context.New(wbad, badReq)
	cbad.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
	cbad.Next()
	if wbad.Code == 200 {
		t.Fatal("expected auth failure")
	}
}

func TestAPIKey_ConstantTimePath(t *testing.T) {
	mw := APIKey("good-key")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "good-key")
	w := httptest.NewRecorder()
	c := context.New(w, req)
	c.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
	c.Next()
	if w.Code != 200 {
		t.Fatalf("want 200 got %d", w.Code)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-API-Key", "bad-key")
	w2 := httptest.NewRecorder()
	c2 := context.New(w2, req2)
	c2.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
	c2.Next()
	if w2.Code == 200 {
		t.Fatal("expected rejection")
	}
}

func TestCSRF_SecureAutoOnHTTPS(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.TLS = &tls.ConnectionState{}
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{CSRF(), func(c *context.Context) error { return nil }}
	c.Next()
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf cookie")
	}
	if !cookies[0].Secure {
		t.Fatal("csrf cookie should be Secure on TLS")
	}
}
