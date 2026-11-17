package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// logEntry is one access-log line and where it goes (nil means standard output).
type logEntry struct {
	out io.Writer
	msg string
}

func (e logEntry) write() {
	out := e.out
	if out == nil {
		out = os.Stdout
	}
	_, _ = io.WriteString(out, e.msg)
}

var (
	logMu   sync.RWMutex
	logChan = make(chan logEntry, 10000)
	logOnce sync.Once
	logWg   sync.WaitGroup
	emitLog = enqueueLog
)

func ensureLogWorker() {
	logOnce.Do(func() {
		logWg.Add(1)
		go func() {
			defer logWg.Done()
			for {
				logMu.RLock()
				ch := logChan
				logMu.RUnlock()
				entry, ok := <-ch
				if !ok {
					return
				}
				entry.write()
			}
		}()
	})
}

func enqueueLog(msg string) {
	enqueueLogTo(nil, msg)
}

// enqueueLogTo queues msg for out (nil means standard output).
func enqueueLogTo(out io.Writer, msg string) {
	ensureLogWorker()
	entry := logEntry{out: out, msg: msg}
	logMu.RLock()
	ch := logChan
	logMu.RUnlock()
	select {
	case ch <- entry:
	default:
		// Channel full: write synchronously so lines are not silently dropped.
		entry.write()
	}
}

// FlushLogger drains buffered access logs (best-effort). Safe to call on shutdown.
func FlushLogger() {
	ensureLogWorker()
	// Give the worker a brief moment to drain; remaining lines stay in the buffer
	// until process exit. Closing the channel is process-lifetime once.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		logMu.RLock()
		ch := logChan
		logMu.RUnlock()
		if len(ch) == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// LoggerConfig configures the access logger.
type LoggerConfig struct {
	// Skipper skips logging when it returns true.
	Skipper func(c *context.Context) bool
	// Format "text" (default) or "json".
	Format string
	// Output receives the log lines (default: standard output). It must be safe for
	// concurrent use: lines are normally written by one background goroutine, but when
	// the buffer is full a request writes its own line directly. Call FlushLogger before
	// exit to drain the buffer.
	Output io.Writer
}

// Logger returns a middleware that logs HTTP requests asynchronously.
func Logger() func(c *context.Context) error {
	return LoggerWithConfig(LoggerConfig{})
}

// LoggerWithConfig returns a configurable access logger.
func LoggerWithConfig(cfg LoggerConfig) func(c *context.Context) error {
	ensureLogWorker()
	return func(c *context.Context) error {
		if c == nil {
			return nil
		}
		start := time.Now()
		c.Next()
		if cfg.Skipper != nil {
			skip := false
			func() {
				defer func() { _ = recover() }()
				skip = cfg.Skipper(c)
			}()
			if skip {
				return nil
			}
		}
		latency := time.Since(start)

		status := c.StatusCode()
		if status == 0 {
			status = 200
		}

		reqID, _ := c.Get(RequestIDKey)
		reqIDStr := ""
		if s, ok := reqID.(string); ok && s != "" {
			reqIDStr = s
		}

		ip := c.ClientIP()
		if ip == "" && c.Request != nil {
			ip = c.Request.RemoteAddr
		}
		path := ""
		method := ""
		if c.Request != nil {
			method = c.Request.Method
			if c.Request.URL != nil {
				path = sanitizeLogPath(c.Request.URL.Path)
			}
		}

		var msg string
		if strings.EqualFold(cfg.Format, "json") {
			// jsonString rather than %q: Go string quoting emits hex escapes that are not valid JSON.
			msg = fmt.Sprintf(`{"time":%s,"status":%d,"latency_ms":%.3f,"ip":%s,"method":%s,"path":%s,"request_id":%s}`+"\n",
				jsonString(time.Now().Format(time.RFC3339)), status, float64(latency.Microseconds())/1000.0,
				jsonString(ip), jsonString(method), jsonString(path), jsonString(reqIDStr))
		} else {
			prefix := ""
			if reqIDStr != "" {
				prefix = reqIDStr + " | "
			}
			timestamp := time.Now().Format("2006/01/02 - 15:04:05")
			// Use %q for path so CR/LF in decoded URLs cannot forge log lines.
			msg = fmt.Sprintf("[KVolt] %s | %s%3d | %13v | %15s | %-7s %q\n",
				timestamp, prefix, status, latency, ip, method, path)
		}
		if cfg.Output != nil {
			enqueueLogTo(cfg.Output, msg)
		} else {
			emitLog(msg)
		}
		return nil
	}
}

// jsonString encodes s as a JSON string literal (control bytes and invalid UTF-8 included).
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

func sanitizeLogPath(path string) string {
	path = strings.ReplaceAll(path, "\r", "")
	path = strings.ReplaceAll(path, "\n", "")
	return path
}
