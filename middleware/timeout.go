package middleware

import (
	stdctx "context"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// Timeout attaches a deadline to the request context.
// Handlers should check c.Request.Context().Done() on slow DB work.
// If the deadline is exceeded and nothing was written, a 504 is sent.
func Timeout(d time.Duration) func(c *context.Context) error {
	return func(c *context.Context) error {
		if d <= 0 {
			c.Next()
			return nil
		}
		ctx, cancel := stdctx.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if ctx.Err() == stdctx.DeadlineExceeded && !c.HeaderWritten() {
			return c.JSON(504, map[string]string{"error": "Gateway Timeout"})
		}
		return nil
	}
}
