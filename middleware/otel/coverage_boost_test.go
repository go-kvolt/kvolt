package otel

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMiddleware_SkipperNilRequestDefaultsAnd5xx(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })

	skipped := Middleware(Config{
		TracerProvider: tp,
		Skipper:        func(c *context.Context) bool { return true },
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/skip", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "s") }}
	_ = skipped(c)
	if len(exporter.GetSpans()) != 0 {
		t.Fatal("skipper should avoid spans")
	}

	// Defaults (empty config) + nil request
	mw := Middleware(Config{})
	cNil := &context.Context{}
	cNil.Handlers = []context.HandlerFunc{func(c *context.Context) error { return nil }}
	_ = mw(cNil)

	exporter.Reset()
	mw2 := Middleware(Config{TracerProvider: tp})
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("POST", "/fail", nil)
	c2 := context.New(w2, r2)
	// No RoutePath — falls back to URL.Path
	c2.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		return c.String(503, "down")
	}}
	_ = mw2(c2)
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans=%d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Fatalf("want Error status, got %v", spans[0].Status)
	}

	exporter.Reset()
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest("GET", "/ok", nil)
	c3 := context.New(w3, r3)
	c3.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		// status 0 → treated as 200
		return nil
	}}
	_ = mw2(c3)
	if len(exporter.GetSpans()) != 1 {
		t.Fatal("want span for ok path")
	}
}
