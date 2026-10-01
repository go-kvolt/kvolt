package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestReady_DoesNotLeakCheckerError(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Ready(&ready, func() error {
			return errors.New("dial tcp 10.0.0.9:5432: connection refused")
		}),
	}
	c.Next()
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "5432") || strings.Contains(body, "10.0.0.9") {
		t.Fatalf("leaked checker detail: %s", body)
	}
	if !strings.Contains(body, "not_ready") {
		t.Fatalf("body=%s", body)
	}
}
