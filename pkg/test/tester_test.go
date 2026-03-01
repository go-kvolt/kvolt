package test

import (
	"testing"

	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
)

func TestTester_GET(t *testing.T) {
	app := kvolt.New()
	app.GET("/hello", func(c *context.Context) error {
		return c.String(200, "world")
	})
	ts := New(t, app)
	ts.GET("/hello").Do().ExpectStatus(200).ExpectBody("world")
}

func TestTester_POST_JSON(t *testing.T) {
	app := kvolt.New()
	app.POST("/echo", func(c *context.Context) error {
		var m map[string]interface{}
		_ = c.Bind(&m)
		return c.JSON(200, m)
	})
	ts := New(t, app)
	ts.POST("/echo").WithJSON(map[string]string{"a": "b"}).Do().ExpectStatus(200).ExpectBodyContains("a")
}
