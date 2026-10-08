package context

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed int
}

func (f *flushRecorder) Flush() { f.flushed++ }

func TestSSE_Event(t *testing.T) {
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	c := New(rec, req)
	if err := c.SSE(); err != nil {
		t.Fatal(err)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type %q", ct)
	}
	if err := c.Event("ping", "hello"); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: ping") || !strings.Contains(body, "data: hello") {
		t.Fatalf("body %q", body)
	}
	if rec.flushed < 1 {
		t.Fatal("expected Flush")
	}
}

func TestSSEvent_SanitizesCRLFInEventAndID(t *testing.T) {
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	c := New(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
	_ = c.SSE()
	if err := c.SSEvent(SSEvent{
		Event: "ping\nevent: injected",
		ID:    "1\ndata: evil",
		Data:  "ok",
	}); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	// Injected newlines must not create new SSE fields on their own lines.
	if strings.Contains(body, "\nevent: injected") || strings.Contains(body, "\ndata: evil") {
		t.Fatalf("CRLF injection in SSE metadata: %q", body)
	}
	if !strings.Contains(body, "data: ok") {
		t.Fatalf("missing data payload: %q", body)
	}
}

func TestFlush_Unsupported(t *testing.T) {
	w := &plainWriter{header: make(http.Header)}
	c := New(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.Flush(); err == nil {
		t.Fatal("expected error")
	}
}

type plainWriter struct {
	header http.Header
	code   int
	body   []byte
}

func (p *plainWriter) Header() http.Header         { return p.header }
func (p *plainWriter) Write(b []byte) (int, error) { p.body = append(p.body, b...); return len(b), nil }
func (p *plainWriter) WriteHeader(statusCode int)  { p.code = statusCode }
