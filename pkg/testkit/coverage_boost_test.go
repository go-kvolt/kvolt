package testkit

import (
	"testing"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
)

func TestAssertStatusAndBody(t *testing.T) {
	app := kvolt.New()
	app.GET("/", func(c *context.Context) error { return c.String(200, "hello world") })
	app.POST("/", func(c *context.Context) error { return c.String(201, "created") })
	ts := New(app)
	r := ts.Get(t, "/")
	r.AssertStatus(t, 200)
	r.AssertBody(t, "hello")
	// Exercise mismatch branches (Errorf only).
	soft := &testing.T{}
	r.AssertStatus(soft, 999)
	r.AssertBody(soft, "missing")
	r2 := ts.Post(t, "/", map[string]string{"a": "b"})
	r2.AssertStatus(t, 201)
}
