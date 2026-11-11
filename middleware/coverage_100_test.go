package middleware

import (
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/golang-jwt/jwt/v5"
)

func TestCoverage100_MiddlewareGaps(t *testing.T) {
	t.Run("flush_logger_empty_and_sleep", func(t *testing.T) {
		logMu.Lock()
		old := logChan
		// Empty channel the worker is not draining → immediate empty return.
		logChan = make(chan string, 2)
		logMu.Unlock()

		FlushLogger()

		// Non-empty → sleep-until-deadline path.
		logMu.Lock()
		logChan <- "stuck"
		logMu.Unlock()

		FlushLogger()

		logMu.Lock()
		logChan = old
		logMu.Unlock()
	})

	t.Run("audit_panic_nil_OnAudit", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("want panic")
			}
		}()
		_ = Audit(AuditConfig{})
	})

	t.Run("basic_auth_bad_base64", func(t *testing.T) {
		mw := BasicAuth(map[string]string{"u": "p"})
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Basic !!!not-base64!!!")
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
		c.Next()
		if w.Code == 200 {
			t.Fatal("want reject")
		}
	})

	t.Run("api_key_empty", func(t *testing.T) {
		mw := APIKey("secret")
		w := httptest.NewRecorder()
		c := context.New(w, httptest.NewRequest("GET", "/", nil))
		c.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
		c.Next()
		if w.Code == 200 {
			t.Fatal("want unauthorized")
		}
	})

	t.Run("cors_empty_and_disallowed_origin", func(t *testing.T) {
		mw := CORSWithConfig(CORSConfig{AllowOrigins: []string{"https://ok.example"}})
		w := httptest.NewRecorder()
		c := context.New(w, httptest.NewRequest("GET", "/", nil)) // no Origin
		c.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
		c.Next()
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("empty origin must not set ACAO")
		}
		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.Header.Set("Origin", "https://evil.example")
		c2 := context.New(w2, r2)
		c2.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
		c2.Next()
		if w2.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("disallowed origin")
		}
	})

	t.Run("gzip_startGzip_error_via_Write", func(t *testing.T) {
		errStartGzip = errors.New("forced startGzip fail")
		defer func() { errStartGzip = nil }()
		rec := httptest.NewRecorder()
		gw := &gzipResponseWriter{ResponseWriter: rec, cfg: GzipConfig{MinSize: 4}}
		gw.buf = []byte("abcd")
		if _, err := gw.Write([]byte("x")); err == nil {
			t.Fatal("want startGzip error from Write")
		}
	})

	t.Run("idempotency_panic_nil_store", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("want panic")
			}
		}()
		_ = Idempotency(IdempotencyConfig{})
	})

	t.Run("idempotency_empty_key_optional", func(t *testing.T) {
		mem := NewMemoryIdempotencyStore()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			Idempotency(IdempotencyConfig{Store: mem, Required: false}),
			func(c *context.Context) error { return c.String(200, "no-key") },
		}
		c.Next()
		if w.Body.String() != "no-key" {
			t.Fatalf("body=%q", w.Body.String())
		}
	})

	t.Run("idempotency_set_oldestKey_empty_break", func(t *testing.T) {
		s := NewMemoryIdempotencyStore()
		s.maxKeys = 0 // force len>=maxKeys with empty map → oldestKey ""
		if err := s.Set("a", IdempotencyRecord{Status: 200, Body: []byte("x")}, time.Hour); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("idempotency_Write_implicit_200", func(t *testing.T) {
		base := httptest.NewRecorder()
		cw := &captureResponseWriter{ResponseWriter: base}
		n, err := cw.Write([]byte("hi"))
		if err != nil || n != 2 {
			t.Fatalf("n=%d err=%v", n, err)
		}
		if base.Code != 200 {
			t.Fatalf("code=%d", base.Code)
		}
	})

	t.Run("idempotency_inflight_Get_error", func(t *testing.T) {
		store := &countGetStore{}
		mw := Idempotency(IdempotencyConfig{Store: store, TTL: time.Minute})
		var wg sync.WaitGroup
		wg.Add(2)
		start := make(chan struct{})
		run := func() {
			defer wg.Done()
			<-start
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/idem", strings.NewReader("{}"))
			r.Header.Set(IdempotencyKeyHeader, "same-key")
			c := context.New(w, r)
			c.Handlers = []context.HandlerFunc{
				mw,
				func(c *context.Context) error {
					time.Sleep(50 * time.Millisecond)
					return c.String(500, "fail") // 5xx not cached
				},
			}
			c.Next()
		}
		go run()
		go run()
		close(start)
		wg.Wait()
	})

	t.Run("jwt_EqualFold_scheme_mismatch", func(t *testing.T) {
		ext := buildJWTExtractor("header:Authorization", "Bearer")
		r := httptest.NewRequest("GET", "/", nil)
		// len("Bearer")==6 and auth[6]==' ', but prefix is not Bearer
		r.Header.Set("Authorization", "Bearxx tokenvalue")
		if _, err := ext(context.New(httptest.NewRecorder(), r)); err == nil {
			t.Fatal("want scheme mismatch")
		}
	})

	t.Run("jwt_hmac_and_wrap_keyfunc_edges", func(t *testing.T) {
		fn := jwtHMACKeyFunc("HS256", []byte("sec"))
		if _, err := fn(&jwt.Token{}); err == nil {
			t.Fatal("nil method")
		}
		if _, err := fn(&jwt.Token{Method: jwt.SigningMethodHS384}); err == nil {
			t.Fatal("alg mismatch")
		}
		if _, err := fn(&jwt.Token{Method: jwt.SigningMethodRS256}); err == nil {
			t.Fatal("non-hmac / alg")
		}
		// Alg matches HS256 but type is not *SigningMethodHMAC
		if _, err := fn(&jwt.Token{Method: algHS256NotHMAC{}}); err == nil {
			t.Fatal("want non-HMAC reject")
		}
		wrap := jwtWrapKeyFunc("HS256", func(t *jwt.Token) (interface{}, error) { return []byte("sec"), nil })
		if _, err := wrap(&jwt.Token{Method: jwt.SigningMethodHS384}); err == nil {
			t.Fatal("wrap alg")
		}
		if _, err := wrap(&jwt.Token{}); err == nil {
			t.Fatal("wrap nil method")
		}
	})

	t.Run("jwt_panic_and_invalid_claims", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("JWT panic")
			}
		}()
		_ = JWT(JWTConfig{})
	})

	t.Run("jwt_invalid_claims_branch", func(t *testing.T) {
		old := jwtMapClaims
		jwtMapClaims = func(token *jwt.Token) (jwt.MapClaims, bool) { return nil, false }
		defer func() { jwtMapClaims = old }()

		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u", "exp": time.Now().Add(time.Hour).Unix()})
		s, err := tok.SignedString([]byte("secret"))
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
		if w.Code == 200 {
			t.Fatal("want claims failure")
		}
	})

	t.Run("jwt_optional_invalid_claims", func(t *testing.T) {
		old := jwtMapClaims
		jwtMapClaims = func(token *jwt.Token) (jwt.MapClaims, bool) { return nil, false }
		defer func() { jwtMapClaims = old }()

		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u", "exp": time.Now().Add(time.Hour).Unix()})
		s, _ := tok.SignedString([]byte("secret"))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+s)
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			JWTOptional(JWTConfig{SigningKey: "secret", SkipOnInvalid: true}),
			func(c *context.Context) error { return c.String(200, "anon") },
		}
		c.Next()
		if w.Code != 200 {
			t.Fatalf("skip claims code=%d", w.Code)
		}

		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.Header.Set("Authorization", "Bearer "+s)
		c2 := context.New(w2, r2)
		c2.Handlers = []context.HandlerFunc{
			JWTOptional(JWTConfig{SigningKey: "secret"}),
			func(c *context.Context) error { return c.String(200, "ok") },
		}
		c2.Next()
		if w2.Code == 200 {
			t.Fatal("want 401 on invalid claims")
		}
	})

	t.Run("limiter_default_keyfunc_empty_clientip", func(t *testing.T) {
		mw := LimiterWithConfig(LimiterConfig{RPS: 10, Burst: 10}) // default KeyFunc
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "" // ClientIP "" → clientIP(RemoteAddr)
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
		c.Next()
	})

	t.Run("limiter_stale_eviction_and_errorhandler_return", func(t *testing.T) {
		base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		n := 0
		old := limiterNow
		limiterNow = func() time.Time {
			n++
			if n <= 2 {
				return base
			}
			return base.Add(2 * time.Minute)
		}
		defer func() { limiterNow = old }()

		mw := LimiterWithConfig(LimiterConfig{
			RPS: 1, Burst: 1, MaxKeys: 1,
			KeyFunc: func(c *context.Context) string { return c.Request.Header.Get("K") },
			ErrorHandler: func(c *context.Context) error {
				_ = c.String(429, "no")
				return nil // header written → return err (nil) at end
			},
		})
		hit := func(k string) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("K", k)
			c := context.New(w, r)
			c.Handlers = []context.HandlerFunc{mw, func(c *context.Context) error { return c.String(200, "ok") }}
			c.Next()
		}
		hit("old")
		hit("old") // 429 + ErrorHandler nil return
		hit("new") // stale eviction of "old"
	})

	t.Run("logger_sync_fallback_and_empty_ip", func(t *testing.T) {
		old := logChan
		logChan = make(chan string, 1)
		logChan <- "fill"
		enqueueLog("sync-fallback")
		logChan = old

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/lip", nil)
		r.RemoteAddr = ""
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			LoggerWithConfig(LoggerConfig{Format: "text"}),
			func(c *context.Context) error { return c.String(200, "ok") },
		}
		c.Next()
		FlushLogger()
	})

	t.Run("metrics_5xx", func(t *testing.T) {
		store := &MetricsStore{}
		w := httptest.NewRecorder()
		c := context.New(w, httptest.NewRequest("GET", "/", nil))
		c.Handlers = []context.HandlerFunc{
			MetricsWithConfig(MetricsConfig{Store: store}),
			func(c *context.Context) error { return c.String(500, "err") },
		}
		c.Next()
		if store.Snapshot().Errors5xx != 1 {
			t.Fatal("want 5xx")
		}
	})

	t.Run("rbac_panics", func(t *testing.T) {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("nil checker")
				}
			}()
			_ = RequireAllPermissions(nil, "a")
		}()
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("empty perms")
				}
			}()
			_ = RequireAllPermissions(func(c *context.Context, p string) (bool, error) { return true, nil })
		}()
	})

	t.Run("recovery_custom_handler", func(t *testing.T) {
		called := false
		w := httptest.NewRecorder()
		c := context.New(w, httptest.NewRequest("GET", "/", nil))
		c.Handlers = []context.HandlerFunc{
			RecoveryWithConfig(RecoveryConfig{
				LogStackTrace: false,
				RecoverHandler: func(c *context.Context, recovered interface{}) {
					called = true
					_ = c.String(503, "handled")
				},
			}),
			func(c *context.Context) error { panic("boom") },
		}
		c.Next()
		if !called || w.Code != 503 {
			t.Fatalf("called=%v code=%d", called, w.Code)
		}
	})

	t.Run("timeout_status_without_wroteHeader", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := context.New(w, httptest.NewRequest("GET", "/", nil))
		c.Handlers = []context.HandlerFunc{
			Timeout(time.Second),
			func(c *context.Context) error {
				tw, ok := c.Writer.(*timeoutWriter)
				if !ok {
					t.Fatal("want *timeoutWriter")
				}
				tw.status = 201
				tw.wroteHeader = false
				return nil
			},
		}
		c.Next()
		if w.Code != 201 {
			t.Fatalf("want 201 got %d", w.Code)
		}
	})
}

type countGetStore struct {
	mu   sync.Mutex
	gets int
}

func (s *countGetStore) Get(key string) (IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets++
	if s.gets >= 3 {
		return IdempotencyRecord{}, false, errors.New("store down after wait")
	}
	return IdempotencyRecord{}, false, nil
}

func (s *countGetStore) Set(key string, rec IdempotencyRecord, ttl time.Duration) error {
	return nil
}

type algHS256NotHMAC struct{}

func (algHS256NotHMAC) Alg() string                              { return "HS256" }
func (algHS256NotHMAC) Verify(string, []byte, interface{}) error { return nil }
func (algHS256NotHMAC) Sign(string, interface{}) ([]byte, error) { return nil, nil }
