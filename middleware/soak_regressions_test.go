package middleware

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// These tests pin down bugs found by running a real server under concurrent load.

func quietLog(t *testing.T) {
	t.Helper()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prev) })
}

// headerCounter counts WriteHeader calls: net/http logs every call after the first as
// "superfluous" and ignores it.
type headerCounter struct {
	*httptest.ResponseRecorder
	calls int
}

func (h *headerCounter) WriteHeader(code int) {
	h.calls++
	h.ResponseRecorder.WriteHeader(code)
}

func runChainOn(w http.ResponseWriter, r *http.Request, handlers ...context.HandlerFunc) {
	c := context.New(w, r)
	c.Handlers = handlers
	c.Next()
	c.FlushHeaders()
}

func gzipRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	return r
}

// With Gzip in the chain a panic used to be answered 200: Gzip committed the headers
// while the panic unwound, before Recovery could write the 500.
func TestGzip_PanicIsStillAnswered500(t *testing.T) {
	quietLog(t)
	w := &headerCounter{ResponseRecorder: httptest.NewRecorder()}
	runChainOn(w, gzipRequest(), Recovery(), Gzip(), func(c *context.Context) error {
		panic("boom")
	})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "internal") || strings.Contains(w.Body.String(), "boom") {
		t.Fatalf("body = %q, want the generic error without the panic value", w.Body.String())
	}
	if w.calls != 1 {
		t.Fatalf("WriteHeader called %d times, want 1", w.calls)
	}
}

// A handler that had already produced a response before panicking keeps that response.
func TestGzip_PanicAfterWriteKeepsWhatWasWritten(t *testing.T) {
	quietLog(t)
	w := &headerCounter{ResponseRecorder: httptest.NewRecorder()}
	runChainOn(w, gzipRequest(), Recovery(), Gzip(), func(c *context.Context) error {
		_ = c.String(http.StatusAccepted, "partial")
		panic("boom after write")
	})
	if w.Code != http.StatusAccepted || w.Body.String() != "partial" {
		t.Fatalf("got %d %q, want 202 partial", w.Code, w.Body.String())
	}
	if w.calls != 1 {
		t.Fatalf("WriteHeader called %d times, want 1", w.calls)
	}
}

// A handler that only sets a status (no body) is answered once, with that status.
func TestGzip_PendingStatusIsWrittenOnce(t *testing.T) {
	w := &headerCounter{ResponseRecorder: httptest.NewRecorder()}
	runChainOn(w, gzipRequest(), Gzip(), func(c *context.Context) error {
		c.Status(http.StatusNoContent)
		return nil
	})
	if w.Code != http.StatusNoContent || w.calls != 1 {
		t.Fatalf("status = %d after %d WriteHeader calls, want 204 after 1", w.Code, w.calls)
	}

	// Raw writes after a pending status: the status is used and the Context knows.
	w = &headerCounter{ResponseRecorder: httptest.NewRecorder()}
	runChainOn(w, gzipRequest(), Gzip(), func(c *context.Context) error {
		c.Status(http.StatusCreated)
		_, err := c.Writer.Write([]byte("raw"))
		return err
	})
	if w.Code != http.StatusCreated || w.Body.String() != "raw" || w.calls != 1 {
		t.Fatalf("got %d %q after %d WriteHeader calls, want 201 raw after 1", w.Code, w.Body.String(), w.calls)
	}
}

// A streamed response (SSE) is flushed before the gzip threshold, which sends the
// headers uncompressed. A later event larger than the threshold used to switch the
// stream to gzip mid-way and corrupt it.
func TestGzip_StreamedResponseStaysPlain(t *testing.T) {
	big := strings.Repeat("x", 4000)
	w := &headerCounter{ResponseRecorder: httptest.NewRecorder()}
	runChainOn(w, gzipRequest(), Gzip(), func(c *context.Context) error {
		if err := c.SSE(); err != nil {
			return err
		}
		if err := c.Event("tick", "small"); err != nil {
			return err
		}
		return c.Event("tick", big)
	})
	if enc := w.Header().Get("Content-Encoding"); enc != "" {
		t.Fatalf("Content-Encoding = %q on a stream that was already flushed plain", enc)
	}
	want := "event: tick\ndata: small\n\nevent: tick\ndata: " + big + "\n\n"
	if w.Body.String() != want {
		t.Fatalf("stream corrupted: got %d bytes starting %.60q", w.Body.Len(), w.Body.String())
	}
	if w.Code != http.StatusOK || w.calls != 1 {
		t.Fatalf("status %d after %d WriteHeader calls, want 200 after 1", w.Code, w.calls)
	}
	if !w.Flushed {
		t.Fatalf("events were not flushed to the client")
	}
}

// A handler that ignores the deadline keeps running after the 504. Whatever it does
// then must not reach the response, even with another writer-wrapping middleware
// (Gzip) outside Timeout. Run with -race: the recorder is not safe for concurrent use,
// so a late write is also reported as a data race.
func TestTimeout_LateHandlerCannotTouchTheResponse(t *testing.T) {
	finished := make(chan struct{})
	rec := httptest.NewRecorder()
	c := context.New(rec, gzipRequest())
	c.Set("upstream", "kept")
	c.Handlers = []context.HandlerFunc{
		Gzip(),
		Timeout(20 * time.Millisecond),
		func(c *context.Context) error {
			defer close(finished)
			time.Sleep(90 * time.Millisecond) // ignores the deadline
			c.Writer.Header().Set("X-Late", "1")
			c.Set("late", true)
			c.Status(http.StatusTeapot)
			return c.JSON(http.StatusOK, map[string]string{"late": "yes"})
		},
	}
	c.Next()
	c.FlushHeaders()

	check := func(when string) {
		t.Helper()
		if rec.Code != http.StatusGatewayTimeout || rec.Body.String() != `{"error":"Gateway Timeout"}` {
			t.Fatalf("%s: got %d %q, want the 504 body only", when, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Late") != "" {
			t.Fatalf("%s: late header reached the response", when)
		}
	}
	check("right after the timeout")
	if c.StatusCode() != http.StatusGatewayTimeout || !c.HeaderWritten() {
		t.Fatalf("request context: status %d written %v, want 504 written", c.StatusCode(), c.HeaderWritten())
	}

	<-finished
	check("after the late handler finished")
	if _, ok := c.Get("late"); ok {
		t.Fatalf("late handler's Set reached the request context")
	}
	if v, _ := c.Get("upstream"); v != "kept" {
		t.Fatalf("request context lost its own values")
	}
}

// When the handler finishes in time, what it did is visible to the middleware outside.
func TestTimeout_OutcomeVisibleUpstream(t *testing.T) {
	var sawUser interface{}
	var sawStatus int
	var sawWritten bool
	rec := httptest.NewRecorder()
	c := context.New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Handlers = []context.HandlerFunc{
		func(c *context.Context) error { // e.g. Audit / Logger: reads after Next
			c.Next()
			sawUser, _ = c.Get("user")
			sawStatus, sawWritten = c.StatusCode(), c.HeaderWritten()
			return nil
		},
		Timeout(time.Second),
		func(c *context.Context) error {
			c.Set("user", "alice")
			c.Writer.Header().Set("X-Made", "yes")
			return c.String(http.StatusCreated, "made")
		},
	}
	c.Next()
	if rec.Code != http.StatusCreated || rec.Body.String() != "made" || rec.Header().Get("X-Made") != "yes" {
		t.Fatalf("response = %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	if sawUser != "alice" || sawStatus != http.StatusCreated || !sawWritten {
		t.Fatalf("upstream saw user=%v status=%d written=%v", sawUser, sawStatus, sawWritten)
	}
	// The request keeps its own context, not the (now cancelled) timeout context.
	if err := c.Request.Context().Err(); err != nil {
		t.Fatalf("request context is cancelled after Timeout returned: %v", err)
	}
}

// A panic before the deadline still reaches Recovery, which answers 500.
func TestTimeout_PanicBeforeDeadlineAnswered500(t *testing.T) {
	quietLog(t)
	rec := httptest.NewRecorder()
	runChainOn(rec, httptest.NewRequest(http.MethodGet, "/", nil), Recovery(), Timeout(time.Second),
		func(c *context.Context) error {
			c.Writer.Header().Set("X-Partial", "set-before-panic")
			panic("boom")
		})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// If the handler already started the response when the deadline passes, the response
// stays as sent (no 504 appended) and later writes are dropped.
func TestTimeout_ResponseAlreadyStarted(t *testing.T) {
	finished := make(chan struct{})
	rec := httptest.NewRecorder()
	c := context.New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Handlers = []context.HandlerFunc{
		Timeout(30 * time.Millisecond),
		func(c *context.Context) error {
			defer close(finished)
			_ = c.String(http.StatusOK, "first;")
			time.Sleep(90 * time.Millisecond)
			_, _ = c.Writer.Write([]byte("late"))
			return nil
		},
	}
	c.Next()
	<-finished
	if rec.Code != http.StatusOK || rec.Body.String() != "first;" {
		t.Fatalf("got %d %q, want 200 with only the part sent before the deadline", rec.Code, rec.Body.String())
	}
	if c.StatusCode() != http.StatusOK || !c.HeaderWritten() {
		t.Fatalf("request context: status %d written %v", c.StatusCode(), c.HeaderWritten())
	}
}

type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	server, client := net.Pipe()
	_ = client.Close()
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

func TestTimeoutWriter_FlushAndHijack(t *testing.T) {
	// Flush before any write commits the handler's headers with a 200.
	rec := httptest.NewRecorder()
	tw := &timeoutWriter{ResponseWriter: rec, h: http.Header{"X-A": {"1"}}}
	tw.Flush()
	if rec.Code != http.StatusOK || rec.Header().Get("X-A") != "1" || !rec.Flushed {
		t.Fatalf("flush: code %d header %q flushed %v", rec.Code, rec.Header().Get("X-A"), rec.Flushed)
	}

	// A hijacked connection belongs to the handler: no 504 may be written to it.
	hr := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	tw = &timeoutWriter{ResponseWriter: hr, h: http.Header{}}
	conn, _, err := tw.Hijack()
	if err != nil || !hr.hijacked {
		t.Fatalf("hijack failed: %v", err)
	}
	_ = conn.Close()
	if !tw.wroteHeader {
		t.Fatalf("hijack must mark the response as started")
	}

	// After the deadline the connection is no longer the handler's to take.
	tw = &timeoutWriter{ResponseWriter: &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}, h: http.Header{}, timedOut: true}
	if _, _, err := tw.Hijack(); err == nil {
		t.Fatalf("hijack after timeout must fail")
	}
}

// scriptedStore answers Get from a script, to reproduce an interleaving exactly.
type scriptedStore struct {
	mu   sync.Mutex
	gets []func() (IdempotencyRecord, bool, error)
	n    int
	sets int
}

func (s *scriptedStore) Get(string) (IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.n >= len(s.gets) {
		return IdempotencyRecord{}, false, nil
	}
	fn := s.gets[s.n]
	s.n++
	return fn()
}

func (s *scriptedStore) Set(string, IdempotencyRecord, time.Duration) error {
	s.mu.Lock()
	s.sets++
	s.mu.Unlock()
	return nil
}

// Request B looks the key up (miss), then request A stores its result and releases the
// gate, then B takes the gate. B must look again instead of running the handler a
// second time — under load this interleaving produced duplicate orders.
func TestIdempotency_LeaderRechecksStoreBeforeRunning(t *testing.T) {
	stored := IdempotencyRecord{Status: http.StatusCreated, Body: []byte(`{"order":"first"}`),
		Headers: http.Header{"Content-Type": {"application/json"}}}
	store := &scriptedStore{gets: []func() (IdempotencyRecord, bool, error){
		func() (IdempotencyRecord, bool, error) { return IdempotencyRecord{}, false, nil }, // B's first lookup
		func() (IdempotencyRecord, bool, error) { return stored, true, nil },               // after A finished
	}}
	ran := false
	r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{}`))
	r.Header.Set(IdempotencyKeyHeader, "k1")
	rec := runChain(r, Idempotency(IdempotencyConfig{Store: store}), func(c *context.Context) error {
		ran = true
		return c.JSON(http.StatusCreated, map[string]string{"order": "second"})
	})
	if ran {
		t.Fatalf("handler ran again although the first request had already stored its result")
	}
	if rec.Code != http.StatusCreated || rec.Body.String() != `{"order":"first"}` || rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("got %d %q replayed=%q, want the stored response", rec.Code, rec.Body.String(), rec.Header().Get("Idempotent-Replayed"))
	}
	if store.sets != 0 {
		t.Fatalf("replay stored the record again")
	}
}

func TestIdempotency_RecheckStoreError(t *testing.T) {
	store := &scriptedStore{gets: []func() (IdempotencyRecord, bool, error){
		func() (IdempotencyRecord, bool, error) { return IdempotencyRecord{}, false, nil },
		func() (IdempotencyRecord, bool, error) { return IdempotencyRecord{}, false, errors.New("store down") },
	}}
	ran := false
	r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{}`))
	r.Header.Set(IdempotencyKeyHeader, "k1")
	rec := runChain(r, Idempotency(IdempotencyConfig{Store: store}), func(c *context.Context) error {
		ran = true
		return c.NoContent()
	})
	if ran || rec.Code != http.StatusInternalServerError {
		t.Fatalf("ran=%v status=%d, want 500 without running the handler", ran, rec.Code)
	}
}

// Many concurrent requests with one key run the handler exactly once.
func TestIdempotency_ConcurrentSameKeyRunsOnce(t *testing.T) {
	idem := Idempotency(IdempotencyConfig{Store: NewMemoryIdempotencyStore()})
	var mu sync.Mutex
	runs := 0
	handler := func(c *context.Context) error {
		mu.Lock()
		runs++
		n := runs
		mu.Unlock()
		time.Sleep(2 * time.Millisecond)
		return c.JSON(http.StatusCreated, map[string]int{"run": n})
	}
	for round := 0; round < 40; round++ {
		mu.Lock()
		runs = 0
		mu.Unlock()
		key := "k" + strings.Repeat("x", round%7) + string(rune('a'+round%26))
		var wg sync.WaitGroup
		bodies := make([]string, 8)
		for i := range bodies {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{}`))
				r.Header.Set(IdempotencyKeyHeader, key)
				bodies[i] = runChain(r, idem, handler).Body.String()
			}(i)
		}
		wg.Wait()
		mu.Lock()
		got := runs
		mu.Unlock()
		if got != 1 {
			t.Fatalf("round %d: handler ran %d times for one key", round, got)
		}
		for _, b := range bodies {
			if b != bodies[0] {
				t.Fatalf("round %d: responses differ: %q vs %q", round, bodies[0], b)
			}
		}
	}
}
