package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

type client struct {
	tokens     int
	lastRefill time.Time
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

const defaultLimiterMaxKeys = 10_000

// LimiterConfig configures a token-bucket rate limiter.
type LimiterConfig struct {
	// RPS is refill rate (tokens per second).
	RPS int
	// Burst is the maximum bucket size.
	Burst int
	// KeyFunc derives the rate-limit bucket key. Default: client IP host.
	KeyFunc func(c *context.Context) string
	// ErrorHandler overrides the default 429 JSON response.
	ErrorHandler func(c *context.Context) error
	// MaxKeys caps distinct bucket keys (default 10000). When full, expired then
	// one arbitrary oldest-ish entry is evicted (no background goroutine).
	MaxKeys int
}

// Limiter implements a token-bucket rate limiter keyed by client IP (host only, not port).
func Limiter(rps int, burst int) func(c *context.Context) error {
	return LimiterWithConfig(LimiterConfig{RPS: rps, Burst: burst})
}

// LimiterWithConfig allows custom keys (API key, tenant id, user id, IP).
func LimiterWithConfig(cfg LimiterConfig) func(c *context.Context) error {
	if cfg.RPS <= 0 {
		cfg.RPS = 10
	}
	if cfg.Burst <= 0 {
		cfg.Burst = cfg.RPS
	}
	if cfg.MaxKeys <= 0 {
		cfg.MaxKeys = defaultLimiterMaxKeys
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *context.Context) string {
			if ip := c.ClientIP(); ip != "" {
				return ip
			}
			return clientIP(c.Request.RemoteAddr)
		}
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(c *context.Context) error {
			return httperr.TooManyRequests("Too Many Requests")
		}
	}

	var mu sync.Mutex
	clients := make(map[string]*client)

	evictOne := func(now time.Time) {
		// Prefer stale buckets (>1m idle).
		for key, cl := range clients {
			if now.Sub(cl.lastRefill) > time.Minute {
				delete(clients, key)
				return
			}
		}
		// Otherwise drop one arbitrary key (map iteration) — O(1) amortised.
		for key := range clients {
			delete(clients, key)
			return
		}
	}

	return func(c *context.Context) error {
		key := cfg.KeyFunc(c)
		if key == "" {
			key = clientIP(c.Request.RemoteAddr)
		}

		mu.Lock()
		lim, exists := clients[key]
		now := time.Now()
		if !exists {
			for len(clients) >= cfg.MaxKeys {
				evictOne(now)
			}
			lim = &client{tokens: cfg.Burst, lastRefill: now}
			clients[key] = lim
		}

		elapsed := now.Sub(lim.lastRefill).Seconds()
		refill := int(elapsed * float64(cfg.RPS))
		if refill > 0 {
			lim.tokens += refill
			if lim.tokens > cfg.Burst {
				lim.tokens = cfg.Burst
			}
			lim.lastRefill = now
		}

		allowed := lim.tokens > 0
		if allowed {
			lim.tokens--
		}
		mu.Unlock()

		if allowed {
			c.Next()
			return nil
		}

		c.Writer.Header().Set("Retry-After", "1")
		err := cfg.ErrorHandler(c)
		if err != nil && !c.HeaderWritten() {
			if he, ok := httperr.As(err); ok {
				return c.JSON(he.Status, he)
			}
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "Too Many Requests"})
		}
		return err
	}
}
