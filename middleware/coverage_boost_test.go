package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/golang-jwt/jwt/v5"
)

// flushHijackWriter implements Flusher + Hijacker for wrapper tests.
type flushHijackWriter struct {
	http.ResponseWriter
	flushed  bool
	hijacked bool
}

func (w *flushHijackWriter) Flush() { w.flushed = true }

func (w *flushHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	c1, c2 := net.Pipe()
	_ = c2.Close()
	return c1, bufio.NewReadWriter(bufio.NewReader(c1), bufio.NewWriter(c1)), nil
}

func TestTimeout_FlushHijackUnwrap(t *testing.T) {
	base := &flushHijackWriter{ResponseWriter: httptest.NewRecorder()}
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(base, r)

	var tw http.ResponseWriter
	c.Handlers = []context.HandlerFunc{
		Timeout(time.Second),
		func(c *context.Context) error {
			tw = c.Writer
			if u, ok := c.Writer.(interface{ Unwrap() http.ResponseWriter }); ok {
				if u.Unwrap() != base {
					t.Fatal("Unwrap want underlying writer")
				}
			} else {
				t.Fatal("timeout writer missing Unwrap")
			}
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
			_, _ = c.Writer.Write([]byte("ok"))
			if h, ok := c.Writer.(http.Hijacker); ok {
				conn, _, err := h.Hijack()
				if err != nil {
					t.Fatalf("Hijack: %v", err)
				}
				_ = conn.Close()
			}
			return nil
		},
	}
	c.Next()
	if tw == nil {
		t.Fatal("handler not reached")
	}
	if !base.flushed {
		t.Fatal("expected Flush to reach underlying writer")
	}
	if !base.hijacked {
		t.Fatal("expected Hijack to reach underlying writer")
	}
}

func TestTimeout_HijackUnsupported(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Timeout(time.Second),
		func(c *context.Context) error {
			h, ok := c.Writer.(http.Hijacker)
			if !ok {
				t.Fatal("want Hijacker")
			}
			_, _, err := h.Hijack()
			if err == nil || !strings.Contains(err.Error(), "not supported") {
				t.Fatalf("want unsupported error, got %v", err)
			}
			return nil
		},
	}
	c.Next()
}

func TestTimeout_FlushAfterTimedOut(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	started := make(chan struct{})
	c.Handlers = []context.HandlerFunc{
		Timeout(30 * time.Millisecond),
		func(c *context.Context) error {
			close(started)
			time.Sleep(80 * time.Millisecond)
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush() // timedOut path
			}
			_, _ = c.Writer.Write([]byte("late"))
			return nil
		},
	}
	c.Next()
	<-started
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("want 504 got %d", w.Code)
	}
}

func TestLimiter_ClientIPFallback(t *testing.T) {
	// Empty KeyFunc forces clientIP(remoteAddr).
	lim := LimiterWithConfig(LimiterConfig{
		RPS:   10,
		Burst: 10,
		KeyFunc: func(c *context.Context) string {
			return ""
		},
	})
	for _, addr := range []string{"9.9.9.9:1234", "bare-host-no-port"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addr
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
		if err := lim(c); err != nil {
			t.Fatalf("addr %s: %v", addr, err)
		}
		if w.Code != 200 {
			t.Fatalf("addr %s: code %d", addr, w.Code)
		}
	}
}

func signHS256(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJWT_CookieAndQueryExtractors(t *testing.T) {
	token := signHS256(t, jwt.MapClaims{
		"sub": "u1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	t.Run("cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			JWT(JWTConfig{SigningKey: "secret", TokenLookup: "cookie:access_token"}),
			func(c *context.Context) error { return c.String(200, "ok") },
		}
		c.Next()
		if w.Code != 200 {
			t.Fatalf("cookie: code=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("query", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/?token="+token, nil)
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			JWT(JWTConfig{SigningKey: "secret", TokenLookup: "query:token"}),
			func(c *context.Context) error { return c.String(200, "ok") },
		}
		c.Next()
		if w.Code != 200 {
			t.Fatalf("query: code=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("missing cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			JWT(JWTConfig{SigningKey: "secret", TokenLookup: "cookie:access_token"}),
			func(c *context.Context) error { return c.String(200, "ok") },
		}
		c.Next()
		if w.Code == 200 {
			t.Fatal("missing cookie should 401")
		}
	})

	t.Run("missing query", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			JWT(JWTConfig{SigningKey: "secret", TokenLookup: "query:token"}),
			func(c *context.Context) error { return c.String(200, "ok") },
		}
		c.Next()
		if w.Code == 200 {
			t.Fatal("missing query should 401")
		}
	})

	t.Run("invalid lookup", func(t *testing.T) {
		ext := buildJWTExtractor("bogus", "Bearer")
		if _, err := ext(nil); err == nil {
			t.Fatal("want invalid lookup error")
		}
		ext2 := buildJWTExtractor("no-colon", "Bearer")
		if _, err := ext2(nil); err == nil {
			t.Fatal("want invalid lookup config")
		}
	})
}

func TestGzip_FlushHijackUnwrapAndEdges(t *testing.T) {
	base := &flushHijackWriter{ResponseWriter: httptest.NewRecorder()}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	c := context.New(base, r)
	payload := strings.Repeat("x", 2048)
	c.Handlers = []context.HandlerFunc{
		GzipWithConfig(GzipConfig{MinSize: 100}),
		func(c *context.Context) error {
			c.Writer.Header().Set("Content-Type", "text/plain")
			if u, ok := c.Writer.(interface{ Unwrap() http.ResponseWriter }); ok {
				if u.Unwrap() != base {
					t.Fatal("Unwrap mismatch")
				}
			}
			_, _ = c.Writer.Write([]byte("hi")) // buffer below MinSize
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush() // flushPlain path (buf non-empty, not yet gzip)
			}
			_, _ = c.Writer.Write([]byte(payload))
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush() // gzip flush path
			}
			return nil
		},
	}
	c.Next()
	if !base.flushed {
		t.Fatal("expected Flush")
	}

	base2 := &flushHijackWriter{ResponseWriter: httptest.NewRecorder()}
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Accept-Encoding", "gzip")
	c2 := context.New(base2, r2)
	c2.Handlers = []context.HandlerFunc{
		GzipWithConfig(GzipConfig{MinSize: 1}),
		func(c *context.Context) error {
			if h, ok := c.Writer.(http.Hijacker); ok {
				conn, _, err := h.Hijack()
				if err != nil {
					t.Fatalf("Hijack: %v", err)
				}
				_ = conn.Close()
			}
			return nil
		},
	}
	c2.Next()
	if !base2.hijacked {
		t.Fatal("expected Hijack")
	}
}

func TestGzip_HijackUnsupportedAndWebsocketSkip(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Gzip(),
		func(c *context.Context) error {
			h, ok := c.Writer.(http.Hijacker)
			if !ok {
				t.Fatal("want Hijacker")
			}
			_, _, err := h.Hijack()
			if err == nil {
				t.Fatal("want hijack error")
			}
			return c.String(200, "tiny")
		},
	}
	c.Next()

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Upgrade", "websocket")
	r2.Header.Set("Accept-Encoding", "gzip")
	c2 := context.New(w2, r2)
	c2.Handlers = []context.HandlerFunc{
		Gzip(),
		func(c *context.Context) error { return c.String(200, "ws") },
	}
	c2.Next()
	if w2.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("websocket should skip gzip")
	}
}

func TestGzip_SkippedContentTypeWritePath(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Gzip(),
		func(c *context.Context) error {
			c.Writer.Header().Set("Content-Type", "application/json")
			_, _ = c.Writer.Write([]byte(`{"a":1}`))
			_, _ = c.Writer.Write([]byte(`{"b":2}`)) // skipped Write path
			return nil
		},
	}
	c.Next()
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("json should skip gzip")
	}
}

func TestFlushLogger(t *testing.T) {
	FlushLogger()
	// Second call after worker started.
	FlushLogger()
}

func TestLogger_EnqueuePath(t *testing.T) {
	// Exercise real enqueueLog (not swapped emitLog).
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/enq", nil)
	r.RemoteAddr = "127.0.0.1:9"
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		LoggerWithConfig(LoggerConfig{Format: "json"}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	FlushLogger()
}

func TestRecovery_ErrAbortHandlerRepanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected re-panic")
		}
		err, ok := r.(error)
		if !ok || err != http.ErrAbortHandler {
			t.Fatalf("want ErrAbortHandler, got %#v", r)
		}
	}()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Recovery(),
		func(c *context.Context) error {
			panic(http.ErrAbortHandler)
		},
	}
	c.Next()
	t.Fatal("should not reach")
}

func TestRecovery_HeaderAlreadyWritten(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		RecoveryWithConfig(RecoveryConfig{LogStackTrace: false, StackSize: 0}),
		func(c *context.Context) error {
			_ = c.String(200, "partial")
			panic("after write")
		},
	}
	c.Next()
	if w.Code != 200 {
		t.Fatalf("want keep 200, got %d", w.Code)
	}
}

func TestLiveReadyMetricsRBACIdempotencyExtras(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/live", nil)
	c := context.New(w, r)
	_ = Live()(c)
	if w.Code != 200 {
		t.Fatalf("Live code=%d", w.Code)
	}

	ready := &atomic.Bool{}
	ready.Store(true)
	w2 := httptest.NewRecorder()
	c2 := context.New(w2, httptest.NewRequest("GET", "/ready", nil))
	_ = Ready(ready, nil, func() error { return nil })(c2)
	if w2.Code != 200 {
		t.Fatalf("Ready code=%d", w2.Code)
	}
	ready.Store(false)
	w3 := httptest.NewRecorder()
	c3 := context.New(w3, httptest.NewRequest("GET", "/ready", nil))
	_ = Ready(ready)(c3)
	if w3.Code != 503 {
		t.Fatalf("Ready not ready code=%d", w3.Code)
	}

	store := &MetricsStore{}
	if store.AvgLatency() != 0 {
		t.Fatal("empty avg")
	}
	hooked := false
	w4 := httptest.NewRecorder()
	r4 := httptest.NewRequest("GET", "/m", nil)
	c4 := context.New(w4, r4)
	c4.Handlers = []context.HandlerFunc{
		MetricsWithConfig(MetricsConfig{Store: store, OnRequest: func(method, path string, status int, d time.Duration) {
			hooked = true
		}}),
		func(c *context.Context) error { return c.String(404, "no") },
	}
	c4.Next()
	snap := store.Snapshot()
	if !hooked || snap.Requests != 1 || snap.Errors4xx != 1 {
		t.Fatalf("hooked=%v snap=%+v", hooked, snap)
	}
	_ = store.AvgLatency() // may be 0 on ultra-fast paths
	_ = Metrics(nil)       // nil store allocates

	check := func(c *context.Context, p string) (bool, error) { return p == "a" || p == "b", nil }
	w5 := httptest.NewRecorder()
	c5 := context.New(w5, httptest.NewRequest("GET", "/", nil))
	c5.Handlers = []context.HandlerFunc{
		RequireAllPermissions(check, "a", "b"),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c5.Next()
	if w5.Code != 200 {
		t.Fatalf("rbac all code=%d", w5.Code)
	}

	mem := NewMemoryIdempotencyStore().WithMaxKeys(2)
	base := &flushHijackWriter{ResponseWriter: httptest.NewRecorder()}
	r6 := httptest.NewRequest("POST", "/idem", strings.NewReader(`{}`))
	r6.Header.Set(IdempotencyKeyHeader, "k1")
	c6 := context.New(base, r6)
	c6.Handlers = []context.HandlerFunc{
		Idempotency(IdempotencyConfig{Store: mem, TTL: time.Minute}),
		func(c *context.Context) error {
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
			if u, ok := c.Writer.(interface{ Unwrap() http.ResponseWriter }); ok {
				_ = u.Unwrap()
			}
			if h, ok := c.Writer.(http.Hijacker); ok {
				conn, _, err := h.Hijack()
				if err == nil {
					_ = conn.Close()
				}
			}
			return c.JSON(200, map[string]string{"ok": "1"})
		},
	}
	c6.Next()
}

func TestCORS_CredentialsWildcardAndPreflight(t *testing.T) {
	mw := CORSWithConfig(CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true, // forced off
		ExposeHeaders:    []string{"X-A"},
		MaxAge:           60,
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/", nil)
	r.Header.Set("Origin", "https://ex.com")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "x") }}
	c.Next()

	mw2 := CORSWithConfig(CORSConfig{AllowOrigins: []string{"https://ex.com"}, AllowCredentials: true})
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Origin", "https://ex.com")
	c2 := context.New(w2, r2)
	c2.Handlers = []context.HandlerFunc{mw2, func(c *context.Context) error { return c.String(200, "x") }}
	c2.Next()
	if w2.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("want credentials")
	}

	mw3 := CORSWithConfig(CORSConfig{AllowOrigins: []string{"https://other.com", "*"}, AllowCredentials: true})
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest("GET", "/", nil)
	r3.Header.Set("Origin", "https://any.com")
	c3 := context.New(w3, r3)
	c3.Handlers = []context.HandlerFunc{mw3, func(c *context.Context) error { return c.String(200, "x") }}
	c3.Next()
	if w3.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("want * got %q", w3.Header().Get("Access-Control-Allow-Origin"))
	}
	if w3.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("credentials should be disabled when * is present")
	}
}

func TestJWT_RequireExpFalseAndHeaderNoScheme(t *testing.T) {
	f := false
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u"})
	s, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	// normalizeJWTConfig forces Bearer for header lookups; exercise empty-scheme extractor directly.
	ext := buildJWTExtractor("header:X-Token", "")
	w0 := httptest.NewRecorder()
	r0 := httptest.NewRequest("GET", "/", nil)
	r0.Header.Set("X-Token", s)
	got, err := ext(context.New(w0, r0))
	if err != nil || got != s {
		t.Fatalf("raw header extract: got=%q err=%v", got, err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "t", Value: s})
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		JWT(JWTConfig{SigningKey: "secret", TokenLookup: "cookie:t", RequireExp: &f}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestTimeout_ZeroDurationAndHeaderAfterWrite(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Timeout(0),
		func(c *context.Context) error { return c.String(200, "fast") },
	}
	c.Next()
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}

	w2 := httptest.NewRecorder()
	c2 := context.New(w2, httptest.NewRequest("GET", "/", nil))
	c2.Handlers = []context.HandlerFunc{
		Timeout(time.Second),
		func(c *context.Context) error {
			_, _ = c.Writer.Write([]byte("x"))
			_ = c.Writer.Header()     // wroteHeader path
			c.Writer.WriteHeader(201) // ignored after write
			return nil
		},
	}
	c2.Next()
}

func TestAudit_PanicErrorPath(t *testing.T) {
	var ev AuditEvent
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/a", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Recovery(),
		Audit(AuditConfig{
			Sync:      true,
			OnAudit:   func(e AuditEvent) { ev = e },
			ActorFunc: func(c *context.Context) string { return "actor" },
		}),
		func(c *context.Context) error { panic(errors.New("audit-panic")) },
	}
	c.Next()
	if ev.Err == nil || ev.Actor != "actor" {
		t.Fatalf("ev=%+v", ev)
	}
	// non-error panic → panicError.Error
	w2 := httptest.NewRecorder()
	c2 := context.New(w2, httptest.NewRequest("GET", "/a", nil))
	c2.Handlers = []context.HandlerFunc{
		Recovery(),
		Audit(AuditConfig{Sync: true, OnAudit: func(e AuditEvent) { ev = e }}),
		func(c *context.Context) error { panic("string-panic") },
	}
	c2.Next()
	if ev.Err == nil || ev.Err.Error() != "panic" {
		t.Fatalf("panicError: %+v", ev.Err)
	}
}

func TestCSRFTokenMatchAndIdempotencySetEvict(t *testing.T) {
	if csrfTokenMatch("", "a") || csrfTokenMatch("a", "") {
		t.Fatal("empty")
	}
	if csrfTokenMatch("ab", "abc") {
		t.Fatal("len mismatch")
	}
	if !csrfTokenMatch("same", "same") {
		t.Fatal("equal")
	}

	s := NewMemoryIdempotencyStore().WithMaxKeys(2)
	rec := IdempotencyRecord{Status: 200, Body: []byte("x")}
	_ = s.Set("a", rec, time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	_ = s.Set("b", rec, time.Hour)
	_ = s.Set("c", rec, time.Hour) // evicts expired + oldest
	if _, ok, _ := s.Get("a"); ok {
		t.Fatal("a should be gone")
	}
	_ = s.Set("d", rec, 0)
	_ = s.Set("e", rec, 0) // eviction with zero TTL items

	falseBody := false
	_ = idempotencyIncludeBody(IdempotencyConfig{IncludeBodyHash: &falseBody})
	_ = idempotencyIncludeBody(IdempotencyConfig{})

	w := httptest.NewRecorder()
	c := context.New(w, httptest.NewRequest("GET", "/", nil))
	_ = writeMiddlewareError(c, nil)
	_ = c.String(200, "x")
	_ = writeMiddlewareError(c, httperr.BadRequest("bad", "no")) // header already written
	w2 := httptest.NewRecorder()
	c2 := context.New(w2, httptest.NewRequest("GET", "/", nil))
	_ = writeMiddlewareError(c2, errors.New("plain"))
	if w2.Code != 500 {
		t.Fatal(w2.Code)
	}
}

func TestEnqueueLogChannelFull(t *testing.T) {
	// Fill buffer then force synchronous fallback path.
	for i := 0; i < 10001; i++ {
		enqueueLog(".")
	}
	FlushLogger()
}
