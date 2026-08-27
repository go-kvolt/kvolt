package middleware

import (
	"fmt"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

var (
	logChan = make(chan string, 10000)
	emitLog = func(msg string) {
		select {
		case logChan <- msg:
		default:
		}
	}
)

func init() {
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

		status := c.StatusCode()
		if status == 0 {
			status = 200
		}

		reqID, _ := c.Get(RequestIDKey)
		reqIDStr := ""
		if s, ok := reqID.(string); ok && s != "" {
			reqIDStr = s + " | "
		}

		timestamp := time.Now().Format("2006/01/02 - 15:04:05")
		msg := fmt.Sprintf("[KVolt] %s | %s%3d | %13v | %15s | %-7s %s\n",
			timestamp,
			reqIDStr,
			status,
			latency,
			c.Request.RemoteAddr,
			c.Request.Method,
			c.Request.URL.Path,
		)

		emitLog(msg)
		return nil
	}
}
