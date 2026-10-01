package middleware

import (
	"net/http/httptest"
	"testing"

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
