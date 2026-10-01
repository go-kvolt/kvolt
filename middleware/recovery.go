package middleware

import (
	"log"
	"net/http"
	"runtime"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// RecoveryConfig configures the Recovery middleware.
type RecoveryConfig struct {
	LogStackTrace bool
	// RecoverHandler overrides the default JSON 500 response.
	RecoverHandler func(c *context.Context, recovered interface{})
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
				if c.HeaderWritten() {
					return
				}
				if config.RecoverHandler != nil {
					config.RecoverHandler(c, err)
					return
				}
				_ = c.JSON(http.StatusInternalServerError, httperr.Internal("internal server error"))
			}
		}()
		c.Next()
		return nil
	}
}
