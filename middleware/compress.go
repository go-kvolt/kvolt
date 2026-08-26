package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/go-kvolt/kvolt/context"
)

const defaultGzipMinSize = 1024

// GzipConfig controls response compression.
type GzipConfig struct {
	// MinSize is the minimum uncompressed body size before gzip kicks in.
	// Smaller responses are sent as-is. Default 1024.
	MinSize int
	// SkipContentTypes, if non-empty, skips gzip when Content-Type has this prefix.
	// Default skips JSON and gRPC (CPU cost, tiny payloads).
	SkipContentTypes []string
}

// DefaultGzipConfig skips JSON APIs and tiny bodies.
var DefaultGzipConfig = GzipConfig{
	MinSize: defaultGzipMinSize,
	SkipContentTypes: []string{
		"application/json",
		"application/grpc",
		"application/grpc+proto",
	},
}

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	cfg         GzipConfig
	gz          *gzip.Writer
	buf         []byte
	usingGzip   bool
	skipped     bool
	wroteHeader bool
	status      int
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.status = code
}

func (w *gzipResponseWriter) skipContentType() bool {
	ct := w.Header().Get("Content-Type")
	for _, prefix := range w.cfg.SkipContentTypes {
		if prefix != "" && strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

func (w *gzipResponseWriter) startGzip() error {
	if w.usingGzip {
		return nil
	}
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	if !w.wroteHeader {
		code := w.status
		if code == 0 {
			code = http.StatusOK
		}
		w.ResponseWriter.WriteHeader(code)
		w.wroteHeader = true
	}
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(w.ResponseWriter)
	w.gz = gz
	w.usingGzip = true
	if len(w.buf) > 0 {
		if _, err := w.gz.Write(w.buf); err != nil {
			return err
		}
		w.buf = w.buf[:0]
	}
	return nil
}

func (w *gzipResponseWriter) flushPlain() error {
	if w.wroteHeader {
		return nil
	}
	code := w.status
	if code == 0 {
		code = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(code)
	w.wroteHeader = true
	if len(w.buf) > 0 {
		_, err := w.ResponseWriter.Write(w.buf)
		w.buf = w.buf[:0]
		return err
	}
	return nil
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.skipped {
		if !w.wroteHeader {
			code := w.status
			if code == 0 {
				code = http.StatusOK
			}
			w.ResponseWriter.WriteHeader(code)
			w.wroteHeader = true
		}
		return w.ResponseWriter.Write(b)
	}
	if w.usingGzip {
		return w.gz.Write(b)
	}
	if w.skipContentType() {
		w.skipped = true
		if err := w.flushPlain(); err != nil {
			return 0, err
		}
		return w.ResponseWriter.Write(b)
	}
	w.buf = append(w.buf, b...)
	if w.cfg.MinSize > 0 && len(w.buf) >= w.cfg.MinSize {
		if err := w.startGzip(); err != nil {
			return 0, err
		}
		return len(b), nil
	}
	return len(b), nil
}

func (w *gzipResponseWriter) Close() {
	if w.usingGzip && w.gz != nil {
		_ = w.gz.Close()
		gzipWriterPool.Put(w.gz)
		w.gz = nil
		return
	}
	_ = w.flushPlain()
}

// Gzip compresses large non-JSON responses when the client sends Accept-Encoding: gzip.
func Gzip() func(c *context.Context) error {
	return GzipWithConfig(DefaultGzipConfig)
}

// GzipWithConfig is Gzip with custom min size / skip types.
func GzipWithConfig(cfg GzipConfig) func(c *context.Context) error {
	if cfg.MinSize <= 0 {
		cfg.MinSize = defaultGzipMinSize
	}
	return func(c *context.Context) error {
		if strings.EqualFold(c.Request.Header.Get("Upgrade"), "websocket") {
			c.Next()
			return nil
		}
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			c.Next()
			return nil
		}

		orig := c.Writer
		gw := &gzipResponseWriter{ResponseWriter: orig, cfg: cfg, status: 0}
		c.Writer = gw
		c.Next()
		if gw.status == 0 && c.StatusCode() != 0 {
			gw.status = c.StatusCode()
		}
		gw.Close()
		c.Writer = orig
		return nil
	}
}
