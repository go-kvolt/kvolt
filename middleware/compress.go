package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/go-kvolt/kvolt/context"
)

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Gzip returns a middleware that compresses HTTP responses.
// It checks 'Accept-Encoding' header and compresses if 'gzip' is supported.
func Gzip() func(c *context.Context) error {
	return func(c *context.Context) error {
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			c.Next()
			return nil
		}

		// Wrap writer
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		c.Writer.Header().Set("Content-Encoding", "gzip")
		c.Writer.Header().Set("Vary", "Accept-Encoding")

		// Replace writer in context
		origWriter := c.Writer
		c.Writer = gzipWriter{ResponseWriter: origWriter, Writer: gz}

		c.Next()

		// Ensure flush
		// gz.Close() is called by defer, but we need to ensure size logic works if we add it later
		return nil
	}
}
