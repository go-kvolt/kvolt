package middleware

import (
	"fmt"
	"strings"
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

// LoggerConfig configures the access logger.
type LoggerConfig struct {
	// Skipper skips logging when it returns true.
	Skipper func(c *context.Context) bool
	// Format "text" (default) or "json".
	Format string
}

// Logger returns a middleware that logs HTTP requests asynchronously.
func Logger() func(c *context.Context) error {
	return LoggerWithConfig(LoggerConfig{})
}

// LoggerWithConfig returns a configurable access logger.
func LoggerWithConfig(cfg LoggerConfig) func(c *context.Context) error {
	return func(c *context.Context) error {
		start := time.Now()
		c.Next()
		if cfg.Skipper != nil && cfg.Skipper(c) {
			return nil
		}
		latency := time.Since(start)

		status := c.StatusCode()
		if status == 0 {
			status = 200
		}

		reqID, _ := c.Get(RequestIDKey)
		reqIDStr := ""
		if s, ok := reqID.(string); ok && s != "" {
			reqIDStr = s
		}

		ip := c.ClientIP()
		if ip == "" {
			ip = c.Request.RemoteAddr
		}
		path := c.Request.URL.Path
		method := c.Request.Method

		var msg string
		if strings.EqualFold(cfg.Format, "json") {
			msg = fmt.Sprintf(`{"time":%q,"status":%d,"latency_ms":%.3f,"ip":%q,"method":%q,"path":%q,"request_id":%q}`+"\n",
				time.Now().Format(time.RFC3339), status, float64(latency.Microseconds())/1000.0, ip, method, path, reqIDStr)
		} else {
			prefix := ""
			if reqIDStr != "" {
				prefix = reqIDStr + " | "
			}
			timestamp := time.Now().Format("2006/01/02 - 15:04:05")
			msg = fmt.Sprintf("[KVolt] %s | %s%3d | %13v | %15s | %-7s %s\n",
				timestamp, prefix, status, latency, ip, method, path)
		}
		emitLog(msg)
		return nil
	}
}
