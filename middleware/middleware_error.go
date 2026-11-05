package middleware

import (
	"net/http"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// writeMiddlewareError writes httperr JSON when the handler error was not already written.
func writeMiddlewareError(c *context.Context, err error) error {
	if err == nil {
		return nil
	}
	if c.HeaderWritten() {
		return err
	}
	if he, ok := httperr.As(err); ok {
		return c.JSON(he.Status, he)
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal_error"})
}
