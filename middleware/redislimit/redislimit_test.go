package redislimit

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	goredis "github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T, limit int, window time.Duration) (func(c *context.Context) error, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb, limit, window), mr
}

func TestRedisLimit_AllowsThenBlocks(t *testing.T) {
	lim, _ := newTestLimiter(t, 2, time.Minute)
	hits := 0
	handler := func(c *context.Context) error {
		hits++
		return c.String(200, "ok")
	}

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "203.0.113.10:1234"
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{handler}
		if err := lim(c); err != nil {
			t.Fatalf("i=%d unexpected err %v", i, err)
		}
		if w.Code != 200 {
			t.Fatalf("i=%d want 200 got %d", i, w.Code)
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.10:1234"
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{handler}
	err := lim(c)
	if err == nil {
		t.Fatal("expected rate-limit error on 3rd request")
	}
	if hits != 2 {
		t.Fatalf("3rd request should not reach handler, hits=%d", hits)
	}
}

func TestRedisLimit_FailClosedOnRedisDown(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	lim := New(rdb, 10, time.Minute)
	mr.Close()
	_ = rdb.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.10:1"
	c := context.New(w, r)
	reached := false
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		reached = true
		return c.String(200, "ok")
	}}
	err = lim(c)
	if err == nil {
		t.Fatal("want error when Redis is down")
	}
	if reached {
		t.Fatal("must fail closed (not call Next)")
	}
}

func TestRedisLimit_PanicWithoutClient(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = NewWithConfig(Config{Limit: 1, Window: time.Second})
}
