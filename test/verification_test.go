package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/middleware"
)

func setupEngine() *kvolt.Engine {
	app := kvolt.New()
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())

	app.GET("/ping", func(c *context.Context) error {
		return c.String(200, "pong")
	})
	app.POST("/echo", func(c *context.Context) error {
		var body map[string]interface{}
		if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
			return c.Status(400).String(400, "Bad Request")
		}
		return c.JSON(200, body)
	})
	app.GET("/user/:id", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"id": c.Param("id")})
	})
	app.GET("/files/*filepath", func(c *context.Context) error {
		return c.String(200, c.Param("filepath"))
	})
	v1 := app.Group("/v1")
	v1.GET("/hello", func(c *context.Context) error {
		return c.String(200, "v1 hello")
	})
	return app
}

func TestStaticRoutes(t *testing.T) {
	app := setupEngine()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	app.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "pong" {
		t.Errorf("GET /ping: want 200 pong, got %d %s", w.Code, w.Body.String())
	}
}

func TestParamRoutes(t *testing.T) {
	app := setupEngine()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/user/123", nil)
	app.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /user/123: want 200, got %d", w.Code)
	}
	body := strings.TrimSpace(w.Body.String())
	if body != `{"id":"123"}` {
		t.Errorf("GET /user/123: body %q", body)
	}
}

func TestWildcardRoutes(t *testing.T) {
	app := setupEngine()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files/css/style.css", nil)
	app.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "css/style.css" {
		t.Errorf("GET /files/...: want 200 css/style.css, got %d %s", w.Code, w.Body.String())
	}
}

func TestGroupRoutes(t *testing.T) {
	app := setupEngine()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/hello", nil)
	app.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "v1 hello" {
		t.Errorf("GET /v1/hello: want 200 v1 hello, got %d %s", w.Code, w.Body.String())
	}
}

func TestConcurrency(t *testing.T) {
	app := setupEngine()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/ping", nil)
			app.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Errorf("concurrent /ping: got %d", w.Code)
			}
		}()
	}
	wg.Wait()
}
