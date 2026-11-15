package test

import (
	"testing"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
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

func TestResponse_Unmarshal(t *testing.T) {
	app := kvolt.New()
	app.GET("/user", func(c *context.Context) error {
		return c.JSON(200, map[string]interface{}{"id": 42, "name": "alice"})
	})
	ts := New(t, app)
	resp := ts.GET("/user").Do().ExpectStatus(200)

	var u struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := resp.Unmarshal(&u); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if u.ID != 42 || u.Name != "alice" {
		t.Fatalf("unexpected user: %+v", u)
	}

	var respNil *Response
	if err := respNil.Unmarshal(&u); err != nil {
		t.Fatal("expected nil error on nil response unmarshal")
	}
}
