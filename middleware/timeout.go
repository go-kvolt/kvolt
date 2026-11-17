package middleware

import (
	"bufio"
	stdctx "context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// Timeout attaches a deadline to the request context and bounds the response wait.
// After d, if nothing was written, a 504 is sent and further handler writes are dropped.
// Handlers should still check c.Request.Context().Done() to stop work early.
//
// The rest of the chain runs on its own goroutine with its own Context (see
// Context.Fork). A handler that ignores the deadline keeps running after the 504 has
// been sent, but everything it does goes to that private Context and a writer that
// discards late output: it cannot touch the response, the pooled Context, or the
// middleware that is unwinding on the request goroutine.
func Timeout(d time.Duration) func(c *context.Context) error {
	return func(c *context.Context) error {
		if c == nil || c.Request == nil || c.Writer == nil {
			if c != nil {
				c.Next()
			}
			return nil
		}
		if d <= 0 {
			c.Next()
			return nil
		}
		ctx, cancel := stdctx.WithTimeout(c.Request.Context(), d)
		defer cancel()

		origWriter := c.Writer
		preH := make(http.Header)
		tw := &timeoutWriter{ResponseWriter: origWriter, h: make(http.Header)}
		for k, vv := range origWriter.Header() {
			tw.h[k] = append([]string(nil), vv...)
			preH[k] = append([]string(nil), vv...)
		}
		fork := c.Fork(tw, c.Request.WithContext(ctx))

		var panicVal interface{}
		var timedOut atomic.Bool
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() {
				if r := recover(); r != nil {
					panicVal = r
					// After the deadline nobody is waiting to re-panic this value, so
					// Recovery never sees it. Log it here or it is lost.
					if timedOut.Load() {
						log.Printf("[KVolt Timeout] panic in handler after timeout: %v", r)
					}
				}
			}()
			fork.Next()
		}()

		select {
		case <-done:
			// The handler goroutine is finished, so the fork is ours again.
			tw.mu.Lock()
			if !tw.wroteHeader {
				// Headers set without a body (c.Status(204), or a panic before any
				// write) are still pending in the private map: hand them over.
				replaceHeader(origWriter.Header(), tw.h)
			}
			tw.mu.Unlock()
			c.Join(fork)
			if panicVal != nil {
				panic(panicVal)
			}
			return nil
		case <-ctx.Done():
			timedOut.Store(true)
			tw.mu.Lock()
			defer tw.mu.Unlock()
			tw.timedOut = true
			if tw.wroteHeader {
				// The handler had already started the response; it stays as sent.
				c.Status(tw.status)
				c.SetHeaderWritten()
				return nil
			}
			replaceHeader(origWriter.Header(), preH)
			origWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
			origWriter.WriteHeader(http.StatusGatewayTimeout)
			_, _ = origWriter.Write([]byte(`{"error":"Gateway Timeout"}`))
			tw.wroteHeader = true
			tw.status = http.StatusGatewayTimeout
			c.Status(http.StatusGatewayTimeout)
			c.SetHeaderWritten()
			return nil
		}
	}
}

// replaceHeader makes dst hold exactly the entries of src.
func replaceHeader(dst, src http.Header) {
	for k := range dst {
		if _, ok := src[k]; !ok {
			delete(dst, k)
		}
	}
	for k, vv := range src {
		dst[k] = append([]string(nil), vv...)
	}
}

// timeoutWriter is the ResponseWriter the handler goroutine sees. Every method is
// serialized with the timeout decision by mu; once timedOut is set nothing reaches the
// real writer any more, because the request goroutine may already have returned it to
// net/http.
type timeoutWriter struct {
	http.ResponseWriter
	h           http.Header // the handler's view of the headers; copied out on WriteHeader
	mu          sync.Mutex
	wroteHeader bool
	timedOut    bool
	status      int
}

// Header always returns the private map: handing out the real one would let the
// handler goroutine mutate it after the request goroutine has moved on.
func (w *timeoutWriter) Header() http.Header {
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
	replaceHeader(w.ResponseWriter.Header(), w.h)
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
	if !w.wroteHeader {
		// Flushing commits the headers (net/http sends an implicit 200): commit ours.
		w.writeHeaderLocked(http.StatusOK)
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

func (w *timeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timedOut {
		return nil, nil, errors.New("timeout: the request has already been answered")
	}
	rw := w.ResponseWriter
	for rw != nil {
		if h, ok := rw.(http.Hijacker); ok {
			conn, buf, err := h.Hijack()
			if err == nil {
				// The connection now belongs to the handler: never write a 504 to it.
				w.wroteHeader = true
				w.status = http.StatusSwitchingProtocols
			}
			return conn, buf, err
		}
		if u, ok := rw.(interface{ Unwrap() http.ResponseWriter }); ok {
			rw = u.Unwrap()
		} else {
			break
		}
	}
	return nil, nil, errors.New("timeout: Hijacker not supported")
}

func (w *timeoutWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
