package kvolt

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

// discardWriter is a minimal ResponseWriter to keep ServeHTTP benches focused
// on framework work (not httptest.ResponseRecorder buffering).
type discardWriter struct {
	h      http.Header
	status int
}

func (d *discardWriter) Header() http.Header {
	if d.h == nil {
		d.h = make(http.Header)
	}
	return d.h
}
func (d *discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardWriter) WriteHeader(statusCode int)  { d.status = statusCode }
func (d *discardWriter) Flush()                      {}

func resetDiscard(d *discardWriter) {
	clear(d.h)
	d.status = 0
}

func BenchmarkEngine_Ping(b *testing.B) {
	app := New()
	app.GET("/ping", func(c *context.Context) error {
		return c.String(200, "pong")
	})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetDiscard(w)
		app.ServeHTTP(w, req)
	}
}

func BenchmarkEngine_PingParallel(b *testing.B) {
	app := New()
	app.GET("/ping", func(c *context.Context) error {
		return c.String(200, "pong")
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := &discardWriter{}
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		for pb.Next() {
			resetDiscard(w)
			app.ServeHTTP(w, req)
		}
	})
}

func BenchmarkEngine_JSON(b *testing.B) {
	app := New()
	app.GET("/json", func(c *context.Context) error {
		return c.JSON(200, map[string]any{"ok": true, "n": 42})
	})
	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetDiscard(w)
		app.ServeHTTP(w, req)
	}
}

func BenchmarkEngine_Param(b *testing.B) {
	app := New()
	app.GET("/users/:id", func(c *context.Context) error {
		return c.String(200, c.Param("id"))
	})
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetDiscard(w)
		app.ServeHTTP(w, req)
	}
}

func BenchmarkEngine_BindJSON(b *testing.B) {
	app := New()
	app.POST("/invoice", func(c *context.Context) error {
		var req struct {
			Party       string `json:"party"`
			AmountPaise int64  `json:"amount_paise"`
		}
		if err := c.BindJSON(&req); err != nil {
			return err
		}
		return c.JSON(200, map[string]any{"ok": true})
	})
	body := `{"party":"acme","amount_paise":1000}`
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetDiscard(w)
		r := httptest.NewRequest(http.MethodPost, "/invoice", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		app.ServeHTTP(w, r)
	}
}

func BenchmarkEngine_DefaultPing(b *testing.B) {
	// Includes Default() middleware (Recovery, RequestID, MaxBodySize).
	app := Default()
	app.GET("/ping", func(c *context.Context) error {
		return c.String(200, "pong")
	})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetDiscard(w)
		app.ServeHTTP(w, req)
	}
}
