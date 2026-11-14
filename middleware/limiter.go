package middleware

import (
	"container/list"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

type client struct {
	key        string
	tokens     int
	lastRefill time.Time
	elem       *list.Element
}

func maskIP(ipStr string) string {
	parsed := net.ParseIP(ipStr)
	if parsed == nil {
		return ipStr
	}
	if ip4 := parsed.To4(); ip4 != nil {
		return ip4.String()
	}
	// IPv6: group by /64 subnet to prevent address rotation bypass
	mask := net.CIDRMask(64, 128)
	return parsed.Mask(mask).String()
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return maskIP(remoteAddr)
	}
	return maskIP(host)
}

const defaultLimiterMaxKeys = 10_000

// limiterNow is time.Now; tests swap it to exercise time-based tests.
var limiterNow = time.Now

// LimiterConfig configures a token-bucket rate limiter.
type LimiterConfig struct {
	// RPS is refill rate (tokens per second).
	RPS int
	// Burst is the maximum bucket size.
	Burst int
	// KeyFunc derives the rate-limit bucket key. Default: client IP host (IPv6 /64 masked).
	KeyFunc func(c *context.Context) string
	// ErrorHandler overrides the default 429 JSON response.
	ErrorHandler func(c *context.Context) error
	// MaxKeys caps distinct bucket keys (default 10000). When full, the least recently
	// used entry is evicted in O(1) time without scanning.
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
				return maskIP(ip)
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
	lru := list.New()

	return func(c *context.Context) error {
		key := cfg.KeyFunc(c)
		if key == "" {
			key = clientIP(c.Request.RemoteAddr)
		}

		mu.Lock()
		lim, exists := clients[key]
		now := limiterNow()
		if !exists {
			for len(clients) >= cfg.MaxKeys {
				oldest := lru.Back()
				if oldest == nil {
					break
				}
				lru.Remove(oldest)
				cOld := oldest.Value.(*client)
				delete(clients, cOld.key)
			}
			lim = &client{key: key, tokens: cfg.Burst, lastRefill: now}
			lim.elem = lru.PushFront(lim)
			clients[key] = lim
		} else {
			lru.MoveToFront(lim.elem)
		}

		elapsed := now.Sub(lim.lastRefill)
		refill := int(elapsed.Seconds() * float64(cfg.RPS))
		if refill > 0 {
			lim.tokens += refill
			if lim.tokens > cfg.Burst {
				lim.tokens = cfg.Burst
				lim.lastRefill = now
			} else {
				lim.lastRefill = lim.lastRefill.Add(time.Duration(float64(refill) * float64(time.Second) / float64(cfg.RPS)))
			}
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
