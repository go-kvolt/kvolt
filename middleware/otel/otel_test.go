package otel

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMiddleware_CreatesSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })

	mw := Middleware(Config{TracerProvider: tp, TracerName: "test"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ping", nil)
	c := context.New(w, r)
	c.RoutePath = "/ping"
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		return c.JSON(200, map[string]string{"ok": "1"})
	}}
	_ = mw(c)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want 1 span got %d", len(spans))
	}
	if spans[0].Name != "GET /ping" {
		t.Fatalf("name=%q", spans[0].Name)
	}
}
