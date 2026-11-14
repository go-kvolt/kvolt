package middleware

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/go-kvolt/kvolt/v2/context"
)

// HealthChecker is a readiness probe callback. Return error when not ready.
type HealthChecker func() error

// Live returns a simple liveness handler ({"status":"ok"}).
func Live() func(c *context.Context) error {
	return func(c *context.Context) error {
		if c == nil {
			return nil
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Ready returns a readiness handler. When ready is false or a checker fails, responds 503.
// Checker error details are logged only (not returned to clients).
func Ready(ready *atomic.Bool, checkers ...HealthChecker) func(c *context.Context) error {
	return func(c *context.Context) error {
		if c == nil {
			return nil
		}
		if ready != nil && !ready.Load() {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		}
		for _, check := range checkers {
			if check == nil {
				continue
			}
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic in health check: %v", r)
					}
				}()
				err = check()
			}()
			if err != nil {
				log.Printf("[Ready] check failed: %v", err)
				return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			}
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	}
}
