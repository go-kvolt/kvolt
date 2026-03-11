package middleware

import (
	"log"
	"runtime"

	"github.com/go-kvolt/kvolt/context"
)

// RecoveryConfig configures the Recovery middleware.
type RecoveryConfig struct {
	// LogStackTrace when true logs the panic stack trace (useful in development).
	// Set false in production to avoid leaking implementation details.
	LogStackTrace bool
}

// DefaultRecoveryConfig is production-safe: stack is logged for debugging.
var DefaultRecoveryConfig = RecoveryConfig{
	LogStackTrace: true,
}

// Recovery returns a middleware that recovers from any panic.
func Recovery() func(c *context.Context) error {
	return RecoveryWithConfig(DefaultRecoveryConfig)
}

// RecoveryWithConfig returns a middleware that recovers from panic with the given config.
func RecoveryWithConfig(config RecoveryConfig) func(c *context.Context) error {
	return func(c *context.Context) error {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[Panic] %v", err)
				if config.LogStackTrace {
					buf := make([]byte, 4096)
					n := runtime.Stack(buf, false)
					log.Printf("[Panic] stack:\n%s", buf[:n])
				}
				if !c.HeaderWritten() {
					c.Status(500).String(500, "Internal Server Error")
				}
			}
		}()
		c.Next()
		return nil
	}
}
