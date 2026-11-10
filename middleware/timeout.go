package middleware

import (
	"bufio"
	stdctx "context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// Timeout attaches a deadline to the request context and bounds the response wait.
// After d, if nothing was written, a 504 is sent and further handler writes are dropped.
// Handlers should still check c.Request.Context().Done() to stop work early.
func Timeout(d time.Duration) func(c *context.Context) error {
	return func(c *context.Context) error {
		if d <= 0 {
			c.Next()
			return nil
		}
		ctx, cancel := stdctx.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		tw := &timeoutWriter{ResponseWriter: c.Writer, h: make(http.Header)}
		for k, vv := range c.Writer.Header() {
			tw.h[k] = append([]string(nil), vv...)
		}
		c.Writer = tw

		done := make(chan struct{})
		go func() {
			defer close(done)
			c.Next()
		}()

		select {
		case <-done:
			tw.mu.Lock()
			defer tw.mu.Unlock()
			if !tw.wroteHeader && tw.status != 0 {
				tw.writeHeaderLocked(tw.status)
			}
			return nil
		case <-ctx.Done():
			tw.mu.Lock()
			if tw.wroteHeader {
				tw.timedOut = true
				tw.mu.Unlock()
				return nil
			}
			tw.timedOut = true
			for k, vv := range tw.h {
				dst := tw.ResponseWriter.Header()
				dst.Del(k)
				for _, v := range vv {
					dst.Add(k, v)
				}
			}
			tw.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
			tw.ResponseWriter.WriteHeader(http.StatusGatewayTimeout)
			_, _ = tw.ResponseWriter.Write([]byte(`{"error":"Gateway Timeout"}`))
			tw.wroteHeader = true
			tw.status = http.StatusGatewayTimeout
			tw.mu.Unlock()
			return nil
		}
	}
}

type timeoutWriter struct {
	http.ResponseWriter
	h           http.Header
	mu          sync.Mutex
	wroteHeader bool
	timedOut    bool
	status      int
}

func (w *timeoutWriter) Header() http.Header {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.wroteHeader {
		return w.ResponseWriter.Header()
	}
	return w.h
}

func (w *timeoutWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.wroteHeader || w.timedOut {
		return
	}
	w.writeHeaderLocked(code)
}

func (w *timeoutWriter) writeHeaderLocked(code int) {
	w.status = code
	for k, vv := range w.h {
		dst := w.ResponseWriter.Header()
		dst.Del(k)
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
	w.ResponseWriter.WriteHeader(code)
	w.wroteHeader = true
}

func (w *timeoutWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timedOut {
		return len(b), nil
	}
	if !w.wroteHeader {
		w.writeHeaderLocked(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *timeoutWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timedOut {
		return
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *timeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("timeout: Hijacker not supported")
	}
	return h.Hijack()
}

func (w *timeoutWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
