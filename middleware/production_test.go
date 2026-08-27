package middleware

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestLogger_LogsRealStatus(t *testing.T) {
	var mu sync.Mutex
	var captured string
	prev := emitLog
	emitLog = func(msg string) {
		mu.Lock()
		captured = msg
		mu.Unlock()
	}
	t.Cleanup(func() { emitLog = prev })

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/missing", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Logger(),
		func(c *context.Context) error {
			return c.JSON(404, map[string]string{"error": "Not Found"})
		},
	}
	c.Next()
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	got := captured
	mu.Unlock()
	if !strings.Contains(got, "404") {
		t.Fatalf("logger should include 404, got %q", got)
	}
	if strings.Contains(got, " 200 ") && !strings.Contains(got, "404") {
		t.Fatalf("logger still looks like always-200: %q", got)
	}
}

func TestGzip_SkipsTinyBody(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Gzip(),
		func(c *context.Context) error {
			return c.String(200, "tiny")
		},
	}
	c.Next()
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("Gzip should skip bodies under 1KB")
	}
}

func TestMaxBodySize_RejectsOverLimit(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"party":"`+strings.Repeat("x", 80)+`"}`))
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		MaxBodySize(16),
		func(c *context.Context) error {
			var dest struct {
				X string `json:"x"`
			}
			err := c.BindJSON(&dest)
			if err == nil {
				t.Fatal("expected max-bytes error")
			}
			return c.JSON(413, map[string]string{"error": "too large"})
		},
	}
	c.Next()
	if w.Code != 413 {
		t.Fatalf("want 413, got %d body=%s", w.Code, w.Body.String())
	}
}
