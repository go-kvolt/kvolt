// Package redislimit provides a Redis-backed rate limiter.
// Import explicitly; core middleware does not pull go-redis.
package redislimit

import (
	"context"
	"strconv"
	"time"

	kvctx "github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/redis/go-redis/v9"
)

var incrExpireScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return current
`)

// Config configures a Redis fixed-window rate limiter.
type Config struct {
	Client       *redis.Client
	Limit        int
	Window       time.Duration
	KeyPrefix    string
	KeyFunc      func(c *kvctx.Context) string
	ErrorHandler func(c *kvctx.Context) error
}

// New rate-limits across instances using Redis INCR + EXPIRE.
func New(client *redis.Client, limit int, window time.Duration) func(c *kvctx.Context) error {
	return NewWithConfig(Config{Client: client, Limit: limit, Window: window})
}

// NewWithConfig returns a configured Redis rate limiter.
func NewWithConfig(cfg Config) func(c *kvctx.Context) error {
	if cfg.Client == nil {
		panic("redislimit: Client required")
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 100
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Second
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "kvolt:rl:"
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *kvctx.Context) string { return c.ClientIP() }
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(c *kvctx.Context) error {
			return httperr.TooManyRequests("Too Many Requests")
		}
	}

	return func(c *kvctx.Context) error {
		if c == nil || c.Writer == nil {
			if c != nil {
				c.Next()
			}
			return nil
		}
		keyPart := ""
		if cfg.KeyFunc != nil {
			func() {
				defer func() { _ = recover() }()
				keyPart = cfg.KeyFunc(c)
			}()
		}
		key := cfg.KeyPrefix + keyPart
		ctx := context.Background()
		if c.Request != nil && c.Request.Context() != nil {
			ctx = c.Request.Context()
		}
		windowMs := cfg.Window.Milliseconds()
		if windowMs <= 0 {
			windowMs = 1000
		}
		n, err := incrExpireScript.Run(ctx, cfg.Client, []string{key}, windowMs).Int64()
		if err != nil {
			return httperr.Internal("rate limiter unavailable")
		}
		remaining := cfg.Limit - int(n)
		if remaining < 0 {
			remaining = 0
		}
		c.Writer.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.Limit))
		c.Writer.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if int(n) > cfg.Limit {
			retrySec := int(cfg.Window.Seconds())
			if retrySec <= 0 {
				retrySec = 1
			}
			c.Writer.Header().Set("Retry-After", strconv.Itoa(retrySec))
			return cfg.ErrorHandler(c)
		}
		c.Next()
		return nil
	}
}
