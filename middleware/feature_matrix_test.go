package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/cache"
	"github.com/go-kvolt/kvolt/v2/pkg/session"
	"github.com/golang-jwt/jwt/v5"
)

func TestTimeout_Returns504WhenHandlerIgnoresDeadline(t *testing.T) {
	mw := Timeout(40 * time.Millisecond)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		mw,
		func(c *context.Context) error {
			time.Sleep(300 * time.Millisecond)
			_ = c.String(200, "late")
			return nil
		},
	}
	start := time.Now()
	c.Next()
	elapsed := time.Since(start)
	if w.Code != 504 {
		t.Fatalf("want 504 got %d body=%s", w.Code, w.Body.String())
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("timeout did not bound wait: elapsed=%v", elapsed)
	}
}

func TestTimeout_AllowsFastHandler(t *testing.T) {
	mw := Timeout(time.Second)
	w := httptest.NewRecorder()
	c := context.New(w, httptest.NewRequest("GET", "/", nil))
	c.Handlers = []context.HandlerFunc{
		mw,
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if w.Code != 200 || w.Body.String() != "ok" {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestSession_CookieValidAndMissing(t *testing.T) {
	store := cache.NewMemoryStore(time.Minute)
	mgr := session.New(store, time.Hour)
	token, err := mgr.Create(map[string]string{"u": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	mw := Session(SessionConfig{Manager: mgr})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "session_id", Value: token})
	c := context.New(w, r)
	hit := false
	c.Handlers = []context.HandlerFunc{
		mw,
		func(c *context.Context) error {
			hit = true
			v, _ := c.Get("session")
			m := v.(map[string]string)
			if m["u"] != "alice" {
				t.Fatalf("session=%v", v)
			}
			return c.String(200, "ok")
		},
	}
	c.Next()
	if !hit || w.Code != 200 {
		t.Fatalf("hit=%v code=%d", hit, w.Code)
	}

	w2 := httptest.NewRecorder()
	c2 := context.New(w2, httptest.NewRequest("GET", "/", nil))
	c2.Handlers = []context.HandlerFunc{
		mw,
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c2.Next()
	if w2.Code != 401 {
		t.Fatalf("want 401 got %d", w2.Code)
	}
}

func TestCORS_PreflightShortCircuit_NotPlainOPTIONS(t *testing.T) {
	mw := CORS()

	// Preflight
	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/x", nil)
	r.Header.Set("Origin", "https://a.example")
	r.Header.Set("Access-Control-Request-Method", "POST")
	c := context.New(w, r)
	hit := false
	c.Handlers = []context.HandlerFunc{
		mw,
		func(c *context.Context) error {
			hit = true
			return c.String(200, "handler")
		},
	}
	c.Next()
	if hit {
		t.Fatal("preflight should not reach handler")
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight want 204 got %d", w.Code)
	}

	// Plain OPTIONS (app handler)
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("OPTIONS", "/x", nil)
	r2.Header.Set("Origin", "https://a.example")
	c2 := context.New(w2, r2)
	hit2 := false
	c2.Handlers = []context.HandlerFunc{
		mw,
		func(c *context.Context) error {
			hit2 = true
			return c.String(200, "custom-options")
		},
	}
	c2.Next()
	if !hit2 || w2.Body.String() != "custom-options" {
		t.Fatalf("plain OPTIONS must reach handler hit=%v body=%q", hit2, w2.Body.String())
	}
}

func TestJWT_RequireExpAndIssuer(t *testing.T) {
	noExp := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u1"})
	s, err := noExp.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+s)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		JWT(JWTConfig{SigningKey: "secret"}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if w.Code != 401 {
		t.Fatalf("missing exp: want 401 got %d", w.Code)
	}

	badIss := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u1",
		"iss": "other",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s2, _ := badIss.SignedString([]byte("secret"))
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Authorization", "Bearer "+s2)
	c2 := context.New(w2, r2)
	c2.Handlers = []context.HandlerFunc{
		JWT(JWTConfig{SigningKey: "secret", Issuer: "kvolt"}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c2.Next()
	if w2.Code != 401 {
		t.Fatalf("bad iss: want 401 got %d", w2.Code)
	}

	okTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u1",
		"iss": "kvolt",
		"aud": "api",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s3, _ := okTok.SignedString([]byte("secret"))
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest("GET", "/", nil)
	r3.Header.Set("Authorization", "Bearer "+s3)
	c3 := context.New(w3, r3)
	c3.Handlers = []context.HandlerFunc{
		JWT(JWTConfig{SigningKey: "secret", Issuer: "kvolt", Audience: "api"}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c3.Next()
	if w3.Code != 200 {
		t.Fatalf("valid token: want 200 got %d body=%s", w3.Code, w3.Body.String())
	}
}

func TestAudit_EmitsOnPanic(t *testing.T) {
	var (
		mu sync.Mutex
		ev AuditEvent
	)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/boom", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Recovery(),
		Audit(AuditConfig{
			Sync: true,
			OnAudit: func(e AuditEvent) {
				mu.Lock()
				ev = e
				mu.Unlock()
			},
		}),
		func(c *context.Context) error {
			panic("audit-me")
		},
	}
	c.Next()
	mu.Lock()
	defer mu.Unlock()
	if ev.Path != "/boom" {
		t.Fatalf("audit path=%q", ev.Path)
	}
	if ev.Err == nil {
		t.Fatal("expected Err set on panic")
	}
	if w.Code != 500 {
		t.Fatalf("recovery want 500 got %d", w.Code)
	}
}

func TestGzip_CompressesLargeText(t *testing.T) {
	body := strings.Repeat("abcdefghij", 200) // 2KB
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Gzip(),
		func(c *context.Context) error {
			c.Writer.Header().Set("Content-Type", "text/plain")
			return c.String(200, body)
		},
	}
	c.Next()
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("want gzip encoding, got %q len=%d", w.Header().Get("Content-Encoding"), w.Body.Len())
	}
	if w.Body.Len() >= len(body) {
		t.Fatalf("expected compressed body smaller than %d got %d", len(body), w.Body.Len())
	}
}

func TestIdempotency_RejectsOversizedKey(t *testing.T) {
	mw := Idempotency(IdempotencyConfig{Store: NewMemoryIdempotencyStore(), Required: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set(IdempotencyKeyHeader, strings.Repeat("a", 200))
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		mw,
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if w.Code != 400 {
		t.Fatalf("want 400 got %d", w.Code)
	}
}
