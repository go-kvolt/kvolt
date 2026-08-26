package middleware

import (
	"github.com/go-kvolt/kvolt/context"
	"github.com/google/uuid"
)

const (
	// RequestIDKey is the context Keys entry for the request id.
	RequestIDKey = "request_id"
	// RequestIDHeader is the HTTP header used to accept/send the id.
	RequestIDHeader = "X-Request-ID"
)

// RequestID sets X-Request-ID on every response. Reuses the incoming header when present.
func RequestID() func(c *context.Context) error {
	return func(c *context.Context) error {
		id := c.Request.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Writer.Header().Set(RequestIDHeader, id)
		c.Next()
		return nil
	}
}
