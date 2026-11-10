package kvolt

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
)

func TestEngine_HTTPMethodsAndGroupMiddleware(t *testing.T) {
	app := New()
	var hits []string
	app.Use(func(c *context.Context) error {
		hits = append(hits, "root")
		c.Next()
		return nil
	})
	g := app.Group("/api")
	g.Use(func(c *context.Context) error {
		hits = append(hits, "group")
		c.Next()
		return nil
	})
	g.GET("/ping", func(c *context.Context) error { return c.String(200, "pong") })
	g.POST("/echo", func(c *context.Context) error {
		b, err := c.RawBody()
		if err != nil {
			return err
		}
		return c.String(200, string(b))
	})
	g.PUT("/x", func(c *context.Context) error {
		c.Status(204)
		return nil
	})
	g.PATCH("/x", func(c *context.Context) error {
		c.Status(204)
		return nil
	})
	g.DELETE("/x", func(c *context.Context) error {
		c.Status(204)
		return nil
	})
	g.HEAD("/ping", func(c *context.Context) error {
		c.Status(200)
		return nil
	})
	g.OPTIONS("/ping", func(c *context.Context) error { return c.String(200, "opt") })

	cases := []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/ping", 200},
		{"POST", "/api/echo", 200},
		{"PUT", "/api/x", 204},
		{"PATCH", "/api/x", 204},
		{"DELETE", "/api/x", 204},
		{"HEAD", "/api/ping", 200},
		{"OPTIONS", "/api/ping", 200},
		{"GET", "/missing", 404},
	}
	for _, tc := range cases {
		hits = nil
		body := strings.NewReader("")
		if tc.method == "POST" {
			body = strings.NewReader("hi")
		}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.path, body)
		app.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: want %d got %d", tc.method, tc.path, tc.want, w.Code)
		}
		if tc.want == 200 || tc.want == 204 {
			if len(hits) < 2 || hits[0] != "root" || hits[1] != "group" {
				t.Fatalf("%s %s middleware order=%v", tc.method, tc.path, hits)
			}
		}
	}
}

func TestEngine_ParamAndWildcard(t *testing.T) {
	app := New()
	app.GET("/users/:id", func(c *context.Context) error {
		return c.String(200, c.Param("id"))
	})
	app.GET("/files/*filepath", func(c *context.Context) error {
		return c.String(200, c.Param("filepath"))
	})

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/users/42", nil))
	if w.Body.String() != "42" {
		t.Fatalf("param=%q", w.Body.String())
	}

	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, httptest.NewRequest("GET", "/files/a/b.txt", nil))
	got := w2.Body.String()
	if !strings.Contains(got, "a/b.txt") {
		t.Fatalf("wildcard=%q", got)
	}
}

func TestEngine_TrustedProxiesClientIP(t *testing.T) {
	app := New()
	app.TrustedProxies = []string{"10.0.0.0/8"}
	var ip string
	app.GET("/", func(c *context.Context) error {
		ip = c.ClientIP()
		return c.String(200, "ok")
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.5")
	app.ServeHTTP(w, r)
	if ip != "203.0.113.9" {
		t.Fatalf("ClientIP=%q", ip)
	}
}

func TestEngine_DefaultRecoveryAndRequestID(t *testing.T) {
	app := Default()
	app.GET("/ok", func(c *context.Context) error { return c.String(200, "ok") })
	app.GET("/panic", func(c *context.Context) error { panic("x") })

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/ok", nil))
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request id")
	}

	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, httptest.NewRequest("GET", "/panic", nil))
	if w2.Code != 500 {
		t.Fatalf("panic want 500 got %d", w2.Code)
	}
}

func TestEngine_CORSPreflightStatus(t *testing.T) {
	app := New()
	app.Use(middleware.CORS())
	app.OPTIONS("/x", func(c *context.Context) error { return c.String(200, "should-not-run") })
	app.GET("/x", func(c *context.Context) error { return c.String(200, "get") })

	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/x", nil)
	r.Header.Set("Origin", "https://a.example")
	r.Header.Set("Access-Control-Request-Method", "POST")
	app.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("preflight want 204 got %d body=%q", w.Code, w.Body.String())
	}
}
