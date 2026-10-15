// Package redislimit provides a Redis-backed rate limiter.
// Import explicitly; core middleware does not pull go-redis.
package redislimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	kvctx "github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/redis/go-redis/v9"
)

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
		key := cfg.KeyPrefix + cfg.KeyFunc(c)
		ctx := context.Background()
		n, err := cfg.Client.Incr(ctx, key).Result()
		if err != nil {
			return httperr.Internal("rate limiter unavailable")
		}
		if n == 1 {
			_ = cfg.Client.Expire(ctx, key, cfg.Window).Err()
		}
		remaining := cfg.Limit - int(n)
		if remaining < 0 {
			remaining = 0
		}
		c.Writer.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.Limit))
		c.Writer.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if int(n) > cfg.Limit {
			c.Writer.Header().Set("Retry-After", fmt.Sprintf("%d", int(cfg.Window.Seconds())))
			return cfg.ErrorHandler(c)
		}
		c.Next()
		return nil
	}
}
