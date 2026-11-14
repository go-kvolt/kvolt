// Package otel provides optional OpenTelemetry HTTP middleware for KVolt.
// Import explicitly; core middleware does not pull OpenTelemetry SDKs.
package otel

import (
	"fmt"

	"github.com/go-kvolt/kvolt/v2/context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Config configures tracing middleware.
type Config struct {
	// TracerName defaults to "github.com/go-kvolt/kvolt/v2".
	TracerName string
	// Propagator defaults to otel.GetTextMapPropagator().
	Propagator propagation.TextMapPropagator
	// TracerProvider defaults to otel.GetTracerProvider().
	TracerProvider trace.TracerProvider
	// Skipper skips tracing when true.
	Skipper func(c *context.Context) bool
}

// Middleware starts a server span per request and records status / route.
func Middleware(cfg Config) func(c *context.Context) error {
	if cfg.TracerName == "" {
		cfg.TracerName = "github.com/go-kvolt/kvolt/v2"
	}
	if cfg.Propagator == nil {
		cfg.Propagator = otel.GetTextMapPropagator()
	}
	if cfg.TracerProvider == nil {
		cfg.TracerProvider = otel.GetTracerProvider()
	}
	tr := cfg.TracerProvider.Tracer(cfg.TracerName)

	return func(c *context.Context) error {
		if c == nil || c.Request == nil {
			if c != nil {
				c.Next()
			}
			return nil
		}
		if cfg.Skipper != nil && cfg.Skipper(c) {
			c.Next()
			return nil
		}

		ctx := cfg.Propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		path := c.RoutePath
		if path == "" && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		spanName := c.Request.Method + " " + path
		ctx, span := tr.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(c.Request.Method),
				semconv.URLPath(path),
				attribute.String("http.route", path),
			),
		)
		defer func() {
			if r := recover(); r != nil {
				span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", r))
				span.SetAttributes(semconv.HTTPResponseStatusCode(500))
				span.End()
				panic(r)
			}
			span.End()
		}()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		status := c.StatusCode()
		if status == 0 {
			status = 200
		}
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))
		if status >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
		} else {
			span.SetStatus(codes.Ok, "")
		}
		return nil
	}
}
