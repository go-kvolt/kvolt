package test

import (
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
)

func TestTester_AllMethodsAndAssertions(t *testing.T) {
	app := kvolt.New()
	app.GET("/h", func(c *context.Context) error {
		c.Writer.Header().Set("X-Test", "1")
		return c.JSON(200, map[string]string{"ok": "yes"})
	})
	app.PUT("/p", func(c *context.Context) error { return c.String(200, "put") })
	app.DELETE("/d", func(c *context.Context) error { return c.String(200, "del") })
	app.PATCH("/a", func(c *context.Context) error { return c.String(200, "patch") })
	app.POST("/echo", func(c *context.Context) error {
		return c.String(201, "created")
	})

	ts := New(t, app)
	ts.GET("/h").WithHeader("Accept", "application/json").Do().
		ExpectStatus(200).
		ExpectHeader("X-Test", "1").
		ExpectJSON(map[string]string{"ok": "yes"}).
		ExpectBodyContains("ok")

	ts.PUT("/p").Do().ExpectStatus(200).ExpectBody("put")
	ts.DELETE("/d").Do().ExpectStatus(200).ExpectBody("del")
	ts.PATCH("/a").Do().ExpectStatus(200).ExpectBody("patch")
	ts.POST("/echo").WithBody([]byte("raw")).Do().ExpectStatus(201)
}

func TestResponse_MismatchPaths(t *testing.T) {
	// Use a sub-tester so failed assertions don't fail the parent.
	st := &testing.T{}
	app := kvolt.New()
	app.GET("/x", func(c *context.Context) error { return c.String(200, "body") })
	ts := New(st, app)
	resp := ts.GET("/x").Do()
	resp.ExpectStatus(404)
	resp.ExpectBody("nope")
	resp.ExpectBodyContains("zzz")
	resp.ExpectHeader("X-Missing", "no")
	resp.ExpectJSON(map[string]string{"a": "b"})
	if !st.Failed() {
		t.Fatal("expected sub-tester to record assertion failures")
	}
}

func TestWithJSON_MarshalFailure(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		st := &testing.T{}
		app := kvolt.New()
		ts := New(st, app)
		// Fatally Goexits this goroutine on marshal error — covered via defer.
		ts.POST("/").WithJSON(complex(1, 2))
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestExpectJSON_ValueMismatch(t *testing.T) {
	st := &testing.T{}
	app := kvolt.New()
	app.GET("/j", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"ok": "yes"})
	})
	ts := New(st, app)
	ts.GET("/j").Do().ExpectJSON(map[string]string{"ok": "no"})
	if !st.Failed() {
		t.Fatal("want ExpectJSON deep-equal failure")
	}
}
