package middleware

import (
	"strings"
	"unicode"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/google/uuid"
)

const (
	// RequestIDKey is the context Keys entry for the request id.
	RequestIDKey = "request_id"
	// RequestIDHeader is the HTTP header used to accept/send the id.
	RequestIDHeader = "X-Request-ID"
	maxRequestIDLen = 128
)

// RequestID sets X-Request-ID on every response.
// Incoming values are sanitized (alphanumeric, '.', '_', '-'; max 128); invalid values are replaced.
func RequestID() func(c *context.Context) error {
	return func(c *context.Context) error {
		id := sanitizeRequestID(c.Request.Header.Get(RequestIDHeader))
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Writer.Header().Set(RequestIDHeader, id)
		c.Next()
		return nil
	}
}

func sanitizeRequestID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if len(id) > maxRequestIDLen {
		id = id[:maxRequestIDLen]
	}
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return ""
	}
	return id
}
