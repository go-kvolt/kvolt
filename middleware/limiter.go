package middleware

import (
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/context"
)

type client struct {
	tokens     int
	lastRefill time.Time
}

// Limiter implements a simple Token Bucket rate limiter.
func Limiter(rps int, burst int) func(c *context.Context) error {
	var mu sync.Mutex
	clients := make(map[string]*client)

	// Cleanup routine (leak prevention)
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastRefill) > time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *context.Context) error {
		ip := c.Request.RemoteAddr // basic IP extraction

		mu.Lock()
		defer mu.Unlock()

		lim, exists := clients[ip]
		if !exists {
			clients[ip] = &client{tokens: burst, lastRefill: time.Now()}
			lim = clients[ip]
		}

		// Refill
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

		if lim.tokens > 0 {
			lim.tokens--
			c.Next()
			return nil
		}

		c.Status(429).String(429, "Too Many Requests")
		return nil // Stop chain
	}
}
