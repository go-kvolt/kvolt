package middleware

import (
	"net"
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/context"
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

// Limiter implements a token-bucket rate limiter keyed by client IP (host only, not port).
func Limiter(rps int, burst int) func(c *context.Context) error {
	var mu sync.Mutex
	clients := make(map[string]*client)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			for ip, cl := range clients {
				if time.Since(cl.lastRefill) > time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *context.Context) error {
		ip := clientIP(c.Request.RemoteAddr)

		mu.Lock()
		lim, exists := clients[ip]
		if !exists {
			lim = &client{tokens: burst, lastRefill: time.Now()}
			clients[ip] = lim
		}

		now := time.Now()
		elapsed := now.Sub(lim.lastRefill).Seconds()
		refill := int(elapsed * float64(rps))
		if refill > 0 {
			lim.tokens += refill
			if lim.tokens > burst {
				lim.tokens = burst
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

		return c.JSON(429, map[string]string{"error": "Too Many Requests"})
	}
}
