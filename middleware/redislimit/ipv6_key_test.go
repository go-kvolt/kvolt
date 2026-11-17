package redislimit

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	goredis "github.com/redis/go-redis/v9"
)

func TestRateLimitIPKey(t *testing.T) {
	cases := map[string]string{
		"203.0.113.10":          "203.0.113.10",
		"::ffff:203.0.113.10":   "203.0.113.10",
		"2001:db8:1:2:aaaa::1":  "2001:db8:1:2::",
		"2001:db8:1:2:bbbb::99": "2001:db8:1:2::",
		"2001:db8:1:3::1":       "2001:db8:1:3::",
		"not-an-ip":             "not-an-ip",
		"":                      "",
	}
	for in, want := range cases {
		if got := rateLimitIPKey(in); got != want {
			t.Errorf("rateLimitIPKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// Addresses in one IPv6 /64 share a bucket, so rotating addresses does not reset the limit.
func TestRedisLimit_IPv6AddressesInSame64ShareBucket(t *testing.T) {
	lim, _ := newTestLimiter(t, 2, time.Minute)
	ok := func(c *context.Context) error { return c.String(200, "ok") }

	hit := func(remote string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{lim, ok}
		c.Next()
		return w.Code
	}

	if got := hit("[2001:db8:1:2::1]:1000"); got != 200 {
		t.Fatalf("first request = %d, want 200", got)
	}
	if got := hit("[2001:db8:1:2::2]:1000"); got != 200 {
		t.Fatalf("second request = %d, want 200", got)
	}
	if got := hit("[2001:db8:1:2:ffff::3]:1000"); got != 429 {
		t.Fatalf("third address in the same /64 = %d, want 429", got)
	}
	if got := hit("[2001:db8:1:9::1]:1000"); got != 200 {
		t.Fatalf("different /64 = %d, want 200", got)
	}
}

// With FailOpen a Redis outage lets requests through; without it they get a 500.
func TestRedisLimit_FailOpen(t *testing.T) {
	for _, failOpen := range []bool{false, true} {
		mr := miniredis.RunT(t)
		rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), MaxRetries: -1, DialTimeout: 200 * time.Millisecond})
		lim := NewWithConfig(Config{Client: rdb, Limit: 1, Window: time.Minute, FailOpen: failOpen})
		mr.Close()

		reached := false
		w := httptest.NewRecorder()
		c := context.New(w, httptest.NewRequest("GET", "/", nil))
		c.Handlers = []context.HandlerFunc{lim, func(c *context.Context) error {
			reached = true
			return c.String(200, "ok")
		}}
		c.Next()
		_ = rdb.Close()

		if failOpen && (!reached || w.Code != 200) {
			t.Fatalf("FailOpen: reached=%v status=%d, want the request to pass", reached, w.Code)
		}
		if !failOpen && (reached || w.Code != 500) {
			t.Fatalf("fail closed: reached=%v status=%d, want 500 without reaching the handler", reached, w.Code)
		}
	}
}
