package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestIdempotency_Replays(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	mw := Idempotency(IdempotencyConfig{Store: store, TTL: time.Hour})
	hits := 0
	handler := func(c *context.Context) error {
		hits++
		return c.JSON(201, map[string]string{"id": "1"})
	}

	run := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/orders", nil)
		r.Header.Set(IdempotencyKeyHeader, "key-abc")
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{handler}
		_ = mw(c)
		return w
	}

	w1 := run()
	if w1.Code != 201 || hits != 1 {
		t.Fatalf("first: code=%d hits=%d", w1.Code, hits)
	}
	w2 := run()
	if w2.Code != 201 || hits != 1 {
		t.Fatalf("replay should not re-run handler: code=%d hits=%d", w2.Code, hits)
	}
	if w2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatal("missing Idempotent-Replayed header")
	}
	if w1.Body.String() != w2.Body.String() {
		t.Fatalf("body mismatch %q vs %q", w1.Body.String(), w2.Body.String())
	}
}

func TestIdempotency_Required(t *testing.T) {
	mw := Idempotency(IdempotencyConfig{
		Store:    NewMemoryIdempotencyStore(),
		Required: true,
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = mw(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", w.Code)
	}
}

func TestIdempotency_SkipsSafeMethods(t *testing.T) {
	mw := Idempotency(IdempotencyConfig{Store: NewMemoryIdempotencyStore(), Required: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x", nil)
	c := context.New(w, r)
	hit := false
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		hit = true
		return c.String(200, "ok")
	}}
	_ = mw(c)
	if !hit || w.Code != 200 {
		t.Fatalf("GET should pass without key, hit=%v code=%d", hit, w.Code)
	}
}
