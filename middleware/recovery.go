package middleware

import (
	"log"
	"runtime"

	"github.com/go-kvolt/kvolt/context"
)

// Recovery returns a middleware that recovers from any panic.
func Recovery() func(c *context.Context) error {
	return func(c *context.Context) error {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 1024)
				n := runtime.Stack(buf, false)
				log.Printf("[Painc] %v\n%s", err, buf[:n])
				c.Status(500).String(500, "Internal Server Error")
			}
		}()
		c.Next()
		return nil
	}
}
