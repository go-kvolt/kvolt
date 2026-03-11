package middleware

import (
	"net/http"

	"github.com/go-kvolt/kvolt/context"
)

// DefaultMaxBodyBytes is 1MB. Use MaxBodySize or MaxBodySizeBytes to override.
const DefaultMaxBodyBytes = 1 << 20

// MaxBodySize limits the request body size to the given number of bytes.
// Requests exceeding the limit will fail when the body is read (e.g. in Bind());
// the reader returns an error so handlers can respond with 413 Payload Too Large.
// Use this to prevent large payload attacks.
func MaxBodySize(maxBytes int) func(c *context.Context) error {
	return func(c *context.Context) error {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxBytes))
		c.Next()
		return nil
	}
}

// MaxBodySizeBytes is like MaxBodySize but accepts an int64 (e.g. for 10*1024*1024).
func MaxBodySizeBytes(maxBytes int64) func(c *context.Context) error {
	return MaxBodySize(int(maxBytes))
}
