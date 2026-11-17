package middleware

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kvolt/kvolt/v2/context"
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
	// The handler already encoded the body (br, zstd, pre-gzipped file): compressing
	// again would send a double-encoded body under a single Content-Encoding.
	if w.Header().Get("Content-Encoding") != "" {
		return true
	}
	ct := w.Header().Get("Content-Type")
	for _, prefix := range w.cfg.SkipContentTypes {
		if prefix != "" && strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

// errStartGzip, when set, makes startGzip fail (test hook for Write error path).
var errStartGzip error

func (w *gzipResponseWriter) startGzip() error {
	if w.usingGzip {
		return nil
	}
	if errStartGzip != nil {
		return errStartGzip
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
	if !w.wroteHeader {
		code := w.status
		if code == 0 {
			code = http.StatusOK
		}
		w.ResponseWriter.WriteHeader(code)
		w.wroteHeader = true
	}
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

// touched reports whether the handler produced any part of a response.
func (w *gzipResponseWriter) touched() bool {
	return w.wroteHeader || w.usingGzip || w.skipped || w.status != 0 || len(w.buf) > 0
}

func (w *gzipResponseWriter) Close() {
	if w.usingGzip {
		if w.gz != nil {
			_ = w.gz.Close()
			gzipWriterPool.Put(w.gz)
			w.gz = nil
		}
		return
	}
	_ = w.flushPlain()
}

func (w *gzipResponseWriter) Flush() {
	if w.usingGzip && w.gz != nil {
		_ = w.gz.Flush()
	} else if !w.skipped {
		// A flush before the gzip threshold sends the headers without Content-Encoding
		// (a streamed response: SSE, chunked progress). From here on the body must stay
		// uncompressed; switching to gzip later would corrupt the stream.
		w.skipped = true
		_ = w.flushPlain()
	}
	rw := w.ResponseWriter
	for rw != nil {
		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
			return
		}
		if u, ok := rw.(interface{ Unwrap() http.ResponseWriter }); ok {
			rw = u.Unwrap()
		} else {
			break
		}
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rw := w.ResponseWriter
	for rw != nil {
		if h, ok := rw.(http.Hijacker); ok {
			return h.Hijack()
		}
		if u, ok := rw.(interface{ Unwrap() http.ResponseWriter }); ok {
			rw = u.Unwrap()
		} else {
			break
		}
	}
	return nil, nil, errors.New("gzip: underlying ResponseWriter does not support Hijacker")
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func acceptsGzip(header string) bool {
	if header == "" {
		return false
	}
	// An explicit gzip entry decides; "*" applies only when gzip is not listed.
	wildcard := false
	for _, part := range strings.Split(header, ",") {
		coding, params, _ := strings.Cut(part, ";")
		coding = strings.ToLower(strings.TrimSpace(coding))
		if coding != "gzip" && coding != "x-gzip" && coding != "*" {
			continue
		}
		ok := encodingQuality(params) > 0
		if coding != "*" {
			return ok
		}
		wildcard = wildcard || ok
	}
	return wildcard
}

// encodingQuality returns the q value from Accept-Encoding parameters (1 when absent or malformed).
func encodingQuality(params string) float64 {
	for _, p := range strings.Split(params, ";") {
		k, v, found := strings.Cut(p, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(k), "q") {
			continue
		}
		q, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 1
		}
		return q
	}
	return 1
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
		if c == nil || c.Request == nil || c.Writer == nil {
			if c != nil {
				c.Next()
			}
			return nil
		}
		if strings.EqualFold(c.Request.Header.Get("Upgrade"), "websocket") {
			c.Next()
			return nil
		}
		if !acceptsGzip(c.Request.Header.Get("Accept-Encoding")) {
			c.Next()
			return nil
		}

		orig := c.Writer
		gw := &gzipResponseWriter{ResponseWriter: orig, cfg: cfg, status: 0}
		c.Writer = gw
		defer func() {
			c.Writer = orig
			if !gw.touched() {
				// Nothing was written: on a panic Recovery must be free to answer 500,
				// and otherwise a pending c.Status() is flushed by the Engine. Committing
				// headers here would turn both into a 200.
				return
			}
			if gw.status == 0 && c.StatusCode() != 0 {
				gw.status = c.StatusCode()
			}
			gw.Close()
			if gw.wroteHeader {
				c.SetHeaderWritten()
			}
		}()
		c.Next()
		return nil
	}
}
