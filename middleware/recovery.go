package middleware

import (
	"errors"
	"log"
	"net/http"
	"runtime"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// RecoveryConfig configures the Recovery middleware.
type RecoveryConfig struct {
	LogStackTrace bool
	// StackSize is the max stack dump size in bytes (default 64KiB).
	StackSize int
	// RecoverHandler overrides the default JSON 500 response.
	RecoverHandler func(c *context.Context, recovered interface{})
}

// DefaultRecoveryConfig is production-safe: stack is logged for debugging.
var DefaultRecoveryConfig = RecoveryConfig{
	LogStackTrace: true,
	StackSize:     64 << 10,
}

// Recovery returns a middleware that recovers from any panic.
func Recovery() func(c *context.Context) error {
	return RecoveryWithConfig(DefaultRecoveryConfig)
}

// RecoveryWithConfig returns a middleware that recovers from panic with the given config.
func RecoveryWithConfig(config RecoveryConfig) func(c *context.Context) error {
	if config.StackSize <= 0 {
		config.StackSize = 64 << 10
	}
	return func(c *context.Context) error {
		defer func() {
			if err := recover(); err != nil {
				// Client disconnect / hijack abort — do not treat as a server bug.
				if e, ok := err.(error); ok && errors.Is(e, http.ErrAbortHandler) {
					panic(err)
				}
				log.Printf("[Panic] %v", err)
				if config.LogStackTrace {
					buf := make([]byte, config.StackSize)
					n := runtime.Stack(buf, false)
					log.Printf("[Panic] stack:\n%s", buf[:n])
				}
				if c.HeaderWritten() {
					return
				}
				if config.RecoverHandler != nil {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[Panic in RecoverHandler] %v", r)
							if !c.HeaderWritten() {
								_ = c.JSON(http.StatusInternalServerError, httperr.Internal("internal server error"))
							}
						}
					}()
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
