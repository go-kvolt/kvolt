package middleware

import (
	"fmt"
	"time"

	"github.com/go-kvolt/kvolt/context"
)

var (
	// Buffered channel to prevent blocking the request handler
	// Size 10000 means we can burst 10k logs before blocking/dropping
	logChan = make(chan string, 10000)
)

func init() {
	// Background log consumer
	go func() {
		for msg := range logChan {
			fmt.Print(msg)
		}
	}()
}

// Logger returns a middleware that logs HTTP requests asynchronously.
func Logger() func(c *context.Context) error {
	return func(c *context.Context) error {
		start := time.Now()

		c.Next()

		latency := time.Since(start)

		// Format message
		timestamp := time.Now().Format("2006/01/02 - 15:04:05")
		msg := fmt.Sprintf("[KVolt] %s | %3d | %13v | %15s | %-7s %s\n",
			timestamp,
			200, // TODO: c.Response.Status check
			latency,
			c.Request.RemoteAddr,
			c.Request.Method,
			c.Request.URL.Path,
		)

		// Non-blocking send
		select {
		case logChan <- msg:
		default:
			// Channel full, drop log to preserve server stability
		}

		return nil
	}
}
