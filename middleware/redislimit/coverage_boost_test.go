package redislimit

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	goredis "github.com/redis/go-redis/v9"
)

func TestNewWithConfig_Defaults(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	lim := NewWithConfig(Config{
		Client: rdb,
		Limit:  0, // default 100
		Window: 0, // default 1s
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.1:9"
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	if err := lim(c); err != nil {
		t.Fatal(err)
	}
	if w.Header().Get("X-RateLimit-Limit") != "100" {
		t.Fatalf("limit header=%q", w.Header().Get("X-RateLimit-Limit"))
	}
}

func TestRedisLimit_ErrorHandlerPanic(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	lim := NewWithConfig(Config{
		Client: rdb,
		Limit:  1,
		Window: time.Minute,
		ErrorHandler: func(c *context.Context) error {
			panic("custom error handler boom")
		},
	})

	// 1st request succeeds
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.RemoteAddr = "198.51.100.2:9"
	c1 := context.New(w1, r1)
	c1.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	if err := lim(c1); err != nil {
		t.Fatal(err)
	}

	// 2nd request triggers rate limit and ErrorHandler panic recovery
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "198.51.100.2:9"
	c2 := context.New(w2, r2)
	c2.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	err = lim(c2)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if w2.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}
