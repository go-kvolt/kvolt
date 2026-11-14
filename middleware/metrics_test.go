package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestMetrics(t *testing.T) {
	store := &MetricsStore{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	Metrics(store)(c)
	snap := store.Snapshot()
	if snap.Requests != 1 || snap.Errors4xx != 0 {
		t.Fatalf("snap=%+v", snap)
	}
}

func TestMetrics_OnRequestPanic(t *testing.T) {
	store := &MetricsStore{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/panichook", nil)
	c := context.New(w, r)
	mw := MetricsWithConfig(MetricsConfig{
		Store: store,
		OnRequest: func(method, path string, status int, d time.Duration) {
			panic("boom in OnRequest hook")
		},
	})
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	if err := mw(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Snapshot().Requests != 1 {
		t.Fatalf("expected 1 recorded request, got %d", store.Snapshot().Requests)
	}
}
