package kvolt

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestServeHTTP_TSRAndMethodNotAllowedAndFlush(t *testing.T) {
	app := New()
	app.RedirectTrailingSlash = true
	app.HandleMethodNotAllowed = true
	app.GET("/dir/", func(c *context.Context) error { return c.String(200, "ok") })
	app.POST("/form/", func(c *context.Context) error { return c.String(200, "f") })
	app.POST("/onlypost", func(c *context.Context) error { return c.String(200, "p") })

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/dir", nil))
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("GET TSR want 301 got %d", w.Code)
	}

	// TSR with query parameters
	wQuery := httptest.NewRecorder()
	app.ServeHTTP(wQuery, httptest.NewRequest("GET", "/dir?sort=desc&page=2", nil))
	if wQuery.Code != http.StatusMovedPermanently {
		t.Fatalf("GET TSR query want 301 got %d", wQuery.Code)
	}
	if loc := wQuery.Header().Get("Location"); loc != "/dir/?sort=desc&page=2" {
		t.Fatalf("TSR query want /dir/?sort=desc&page=2 got %q", loc)
	}

	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, httptest.NewRequest("POST", "/form", nil))
	if w2.Code != http.StatusPermanentRedirect {
		t.Fatalf("POST TSR want 308 got %d", w2.Code)
	}

	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, httptest.NewRequest("GET", "/onlypost", nil))
	if w3.Code != 405 {
		t.Fatalf("want 405 got %d allow=%s", w3.Code, w3.Header().Get("Allow"))
	}

	// Single-handler path + FlushHeaders when nothing written
	app2 := New()
	app2.GET("/empty", func(c *context.Context) error { return nil })
	w4 := httptest.NewRecorder()
	app2.ServeHTTP(w4, httptest.NewRequest("GET", "/empty", nil))
}

func TestProxyAndHTTPServerDefaults(t *testing.T) {
	if isTrustedProxy("1.1.1.1:80", nil) {
		t.Fatal("empty trusted")
	}
	if isTrustedProxy("not-ip", []string{"1.1.1.1"}) {
		t.Fatal("bad host")
	}
	if !isTrustedProxy("10.0.0.1:9", []string{"", "10.0.0.0/8", "bad/cidr", "10.0.0.1"}) {
		t.Fatal("want trusted")
	}
	if isTrustedProxy("8.8.8.8:1", []string{"10.0.0.1"}) {
		t.Fatal("untrusted")
	}

	app := New()
	app.Server.ReadHeaderTimeout = 0
	app.Server.ReadTimeout = 0
	app.Server.WriteTimeout = 0
	app.Server.IdleTimeout = 0
	app.Server.ShutdownTimeout = 0
	srv := app.HTTPServer(":0")
	if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 {
		t.Fatal("defaults")
	}
	if app.ShutdownTimeout() <= 0 {
		t.Fatal("shutdown default")
	}
	app.Server.ShutdownTimeout = 2 * time.Second
	if app.ShutdownTimeout() != 2*time.Second {
		t.Fatal("custom shutdown")
	}
	app.Server.MaxHeaderBytes = 4096
	app.Server.WriteTimeout = -1
	srv2 := app.HTTPServer(":1")
	if srv2.MaxHeaderBytes != 4096 || srv2.WriteTimeout != DefaultWriteTimeout {
		t.Fatalf("%+v", srv2)
	}
}
