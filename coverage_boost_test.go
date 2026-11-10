package kvolt

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestEngine_DocsAnyHandleAttachHTML(t *testing.T) {
	app := New()
	app.NoRoute(func(c *context.Context) error { return c.String(404, "no") })
	app.NoMethod(func(c *context.Context) error { return c.String(405, "nope") })
	app.HandleMethodNotAllowed = true

	r := app.GET("/doc", func(c *context.Context) error { return c.String(200, "ok") }).
		Desc("d").Tags("t").Request(map[string]string{"a": "b"}).
		Response(200, map[string]string{"ok": "1"}).Security("bearer")
	if r == nil {
		t.Fatal("route")
	}
	app.Any("/any", func(c *context.Context) error {
		c.Status(204)
		return nil
	})
	app.Handle("TRACE", "/trace", func(c *context.Context) error { return c.String(200, "t") })

	app.Attach("k", 1)
	if v, ok := app.Attachment("k"); !ok || v != 1 {
		t.Fatal("attach")
	}
	_ = app.HTTPServer(":0")
	_ = app.ShutdownTimeout()
	app.Server.ShutdownTimeout = 1500 * time.Millisecond
	if app.ShutdownTimeout() != 1500*time.Millisecond {
		t.Fatalf("shutdown timeout %v", app.ShutdownTimeout())
	}
	app.Server.WriteTimeout = -1
	_ = app.HTTPServer(":1")
	_ = app.IsTrustedProxy("127.0.0.1:1")
	app.TrustedProxies = []string{"127.0.0.1"}
	_ = app.IsTrustedProxy("127.0.0.1:9")

	dir := t.TempDir()
	p := filepath.Join(dir, "x.html")
	if err := os.WriteFile(p, []byte("<b>{{.}}</b>"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.LoadHTMLGlob(filepath.Join(dir, "*.html"))

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/doc", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, httptest.NewRequest("GET", "/missing", nil))
	if w2.Code != 404 {
		t.Fatal(w2.Code)
	}
	app.GET("/onlyget", func(c *context.Context) error { return c.String(200, "g") })
	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, httptest.NewRequest("POST", "/onlyget", nil))
	if w3.Code != 405 {
		t.Fatalf("want 405 got %d", w3.Code)
	}
	_ = app.Routes()
	_ = formatListenURL("http", ":8080")
	_ = formatListenURL("https", "127.0.0.1:443")
	_ = alternatePath("/x/")
	_ = alternatePath("/x")
	_ = alternatePath("")
	_ = alternatePath("/")
}
