package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestTimeout_PanicPropagatedToRecovery(t *testing.T) {
	recovered := false
	recMw := RecoveryWithConfig(RecoveryConfig{
		RecoverHandler: func(c *context.Context, r interface{}) {
			recovered = true
			_ = c.JSON(http.StatusInternalServerError, map[string]string{"error": "panic caught"})
		},
	})
	toMw := Timeout(1 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	c := context.New(rec, req)

	c.Handlers = []context.HandlerFunc{
		recMw,
		toMw,
		func(c *context.Context) error {
			panic("downstream explosion")
		},
	}

	c.Next()

	if !recovered {
		t.Fatalf("expected panic to be caught by recovery handler")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 status code, got %d", rec.Code)
	}
}

func TestTimeout_RestoresOriginalWriter(t *testing.T) {
	toMw := Timeout(100 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := context.New(rec, req)
	origWriter := c.Writer

	c.Handlers = []context.HandlerFunc{
		toMw,
		func(c *context.Context) error {
			return c.String(http.StatusOK, "ok")
		},
	}

	c.Next()

	if c.Writer != origWriter {
		t.Fatalf("expected c.Writer to be restored to original writer")
	}
}
