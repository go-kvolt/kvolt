package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// runChain executes handlers against a fresh recorder and returns it.
func runChain(r *http.Request, handlers ...context.HandlerFunc) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c := context.New(rec, r)
	c.Handlers = handlers
	c.Next()
	c.FlushHeaders()
	return rec
}

// Headers set by middleware that runs before Idempotency (CORS, RequestID) are set again on
// every request. A replay must not duplicate them: two Access-Control-Allow-Origin values
// make browsers reject the response.
func TestIdempotency_ReplayDoesNotDuplicateUpstreamHeaders(t *testing.T) {
	idem := Idempotency(IdempotencyConfig{Store: NewMemoryIdempotencyStore()})
	handler := func(c *context.Context) error {
		c.Writer.Header().Set("X-Order", "42")
		return c.JSON(http.StatusCreated, map[string]int{"id": 42})
	}
	do := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`{}`))
		r.Header.Set("Origin", "https://a.example")
		r.Header.Set(IdempotencyKeyHeader, "k1")
		return runChain(r, CORS(), RequestID(), idem, handler)
	}

	first := do()
	second := do()

	if second.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("second response was not a replay")
	}
	if second.Code != http.StatusCreated || second.Body.String() != first.Body.String() {
		t.Fatalf("replay = %d %q, want %d %q", second.Code, second.Body.String(), first.Code, first.Body.String())
	}
	for _, h := range []string{"Access-Control-Allow-Origin", RequestIDHeader, "X-Order", "Content-Type"} {
		if got := second.Header().Values(h); len(got) != 1 {
			t.Fatalf("%s has %d values on replay, want 1: %q", h, len(got), got)
		}
	}
	// The request id belongs to the replaying request, not the original one.
	if first.Header().Get(RequestIDHeader) == second.Header().Get(RequestIDHeader) {
		t.Fatalf("replay reused the original request id")
	}
	if second.Header().Get("X-Order") != "42" {
		t.Fatalf("handler header was not replayed")
	}
}

// 429 / 408 / 425 are transient: a retry with the same key must run the handler again.
func TestIdempotency_DoesNotCacheTransientStatuses(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusRequestTimeout, http.StatusTooEarly} {
		idem := Idempotency(IdempotencyConfig{Store: NewMemoryIdempotencyStore()})
		calls := 0
		handler := func(c *context.Context) error {
			calls++
			if calls == 1 {
				return c.JSON(status, map[string]string{"error": "retry"})
			}
			return c.JSON(http.StatusOK, map[string]string{"ok": "yes"})
		}
		do := func() *httptest.ResponseRecorder {
			r := httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`{}`))
			r.Header.Set(IdempotencyKeyHeader, "k1")
			return runChain(r, idem, handler)
		}
		if got := do().Code; got != status {
			t.Fatalf("first = %d, want %d", got, status)
		}
		second := do()
		if second.Code != http.StatusOK || calls != 2 {
			t.Fatalf("status %d: retry = %d after %d calls, want 200 after 2", status, second.Code, calls)
		}
	}
}

// A response over MaxResponseBytes is sent in full but not kept for replay.
func TestIdempotency_SkipsOversizedResponses(t *testing.T) {
	big := strings.Repeat("x", 64)
	cases := []struct {
		name      string
		max       int64
		wantCalls int
	}{
		{"over limit", 16, 2},
		{"under limit", 1024, 1},
		{"no limit", -1, 1},
	}
	for _, tc := range cases {
		idem := Idempotency(IdempotencyConfig{Store: NewMemoryIdempotencyStore(), MaxResponseBytes: tc.max})
		calls := 0
		handler := func(c *context.Context) error {
			calls++
			return c.String(http.StatusOK, big)
		}
		for i := 0; i < 2; i++ {
			r := httptest.NewRequest(http.MethodPost, "/export", strings.NewReader(`{}`))
			r.Header.Set(IdempotencyKeyHeader, "k1")
			rec := runChain(r, idem, handler)
			if rec.Body.String() != big {
				t.Fatalf("%s: body truncated to %d bytes", tc.name, rec.Body.Len())
			}
		}
		if calls != tc.wantCalls {
			t.Fatalf("%s: handler ran %d times, want %d", tc.name, calls, tc.wantCalls)
		}
	}
}

// JSON access-log lines must stay valid JSON for any request path.
func TestLogger_JSONFormatIsValidJSON(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	prev := emitLog
	emitLog = func(msg string) {
		mu.Lock()
		lines = append(lines, msg)
		mu.Unlock()
	}
	defer func() { emitLog = prev }()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.URL.Path = "/a\x01b\xff\"c\\d" // control byte, invalid UTF-8, quote, backslash
	runChain(r, LoggerWithConfig(LoggerConfig{Format: "json"}), func(c *context.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1", len(lines))
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\n%s", err, lines[0])
	}
	if entry["method"] != "GET" || entry["status"] != float64(200) {
		t.Fatalf("unexpected entry: %v", entry)
	}
	if path, _ := entry["path"].(string); !strings.HasPrefix(path, "/a\x01b") || !strings.HasSuffix(path, `"c\d`) {
		t.Fatalf("path not preserved: %q", entry["path"])
	}
}

// A handler panic after the deadline cannot reach Recovery (the 504 is already sent),
// so Timeout must log it rather than drop it.
func TestTimeout_LogsPanicAfterDeadline(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	}))
	log.SetFlags(0)
	defer func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}()

	done := make(chan struct{})
	rec := runChain(httptest.NewRequest(http.MethodGet, "/slow", nil),
		Timeout(10*time.Millisecond),
		func(c *context.Context) error {
			defer close(done)
			<-c.Request.Context().Done()
			time.Sleep(20 * time.Millisecond) // let the middleware answer 504 first
			panic("boom after deadline")
		})
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", rec.Code)
	}
	<-done

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		out := buf.String()
		mu.Unlock()
		if strings.Contains(out, "boom after deadline") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("panic was not logged; log output: %q", out)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// With an allow-list, the response varies by Origin even when the origin is refused.
func TestCORS_VaryOriginForRefusedOrigin(t *testing.T) {
	cors := CORSWithConfig(CORSConfig{AllowOrigins: []string{"https://good.example"}})
	ok := func(c *context.Context) error { return c.String(http.StatusOK, "ok") }

	for _, origin := range []string{"https://good.example", "https://evil.example", ""} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		rec := runChain(r, cors, ok)
		if got := rec.Header().Values("Vary"); len(got) != 1 || got[0] != "Origin" {
			t.Fatalf("origin %q: Vary = %q, want [Origin]", origin, got)
		}
		wantACAO := ""
		if origin == "https://good.example" {
			wantACAO = origin
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != wantACAO {
			t.Fatalf("origin %q: Access-Control-Allow-Origin = %q, want %q", origin, got, wantACAO)
		}
	}

	// Wildcard responses do not depend on Origin.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://any.example")
	if got := runChain(r, CORS(), ok).Header().Get("Vary"); got != "" {
		t.Fatalf("wildcard Vary = %q, want empty", got)
	}
}

func TestAcceptsGzip_Negotiation(t *testing.T) {
	cases := map[string]bool{
		"":                      false,
		"gzip":                  true,
		"GZIP":                  true,
		"x-gzip":                true,
		"deflate, gzip;q=0.5":   true,
		"gzip;q=0.05":           true, // small but non-zero quality still accepts
		"gzip;q=0":              false,
		"gzip; q=0.0":           false,
		"gzip;q=0.":             false,
		"gzip;q=abc":            true, // malformed q is ignored
		"br":                    false,
		"gzipped":               false, // not the gzip coding
		"*":                     true,
		"*;q=0":                 false,
		"*, gzip;q=0":           false, // explicit gzip refusal beats the wildcard
		"gzip;q=0, *":           false,
		"identity;q=1, *;q=0.1": true,
	}
	for header, want := range cases {
		if got := acceptsGzip(header); got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

// A body the handler already encoded must not be gzipped a second time.
func TestGzip_SkipsAlreadyEncodedResponse(t *testing.T) {
	body := strings.Repeat("already-encoded-bytes ", 200)
	r := httptest.NewRequest(http.MethodGet, "/asset", nil)
	r.Header.Set("Accept-Encoding", "gzip, br")
	rec := runChain(r, Gzip(), func(c *context.Context) error {
		c.Writer.Header().Set("Content-Encoding", "br")
		return c.Data(http.StatusOK, "text/plain", []byte(body))
	})
	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if rec.Body.String() != body {
		t.Fatalf("body was re-encoded (%d bytes, want %d)", rec.Body.Len(), len(body))
	}
}

// syncBuffer is a bytes.Buffer safe for concurrent use.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// LoggerConfig.Output receives the access log instead of standard output.
func TestLogger_WritesToConfiguredOutput(t *testing.T) {
	var out syncBuffer
	logger := LoggerWithConfig(LoggerConfig{Format: "json", Output: &out})
	for _, path := range []string{"/one", "/two"} {
		runChain(httptest.NewRequest(http.MethodGet, path, nil), logger, func(c *context.Context) error {
			return c.String(http.StatusOK, "ok")
		})
	}

	deadline := time.Now().Add(2 * time.Second)
	for strings.Count(out.String(), "\n") < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("log lines did not reach Output: %q", out.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for i, want := range []string{"/one", "/two"} {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		if entry["path"] != want {
			t.Fatalf("line %d path = %v, want %s (lines must keep request order)", i, entry["path"], want)
		}
	}
}

// When the buffer is full the line is written directly rather than dropped.
func TestLogger_OutputWhenBufferFull(t *testing.T) {
	logMu.Lock()
	old := logChan
	logChan = make(chan logEntry) // unbuffered and unread: every send would block
	logMu.Unlock()
	defer func() {
		logMu.Lock()
		logChan = old
		logMu.Unlock()
	}()

	var out syncBuffer
	enqueueLogTo(&out, "direct line\n")
	if out.String() != "direct line\n" {
		t.Fatalf("Output = %q, want the line written directly", out.String())
	}
}
