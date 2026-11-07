package redislimit

import (
	"net/http/httptest"
	"testing"

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
