package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

const (
	// IdempotencyKeyHeader is the standard request header.
	IdempotencyKeyHeader = "Idempotency-Key"
	maxIdempotencyKeyLen = 128
)

// IdempotencyRecord is a cached successful (or finished) response for a key.
type IdempotencyRecord struct {
	Status  int
	Headers http.Header
	Body    []byte
}

// IdempotencyStore persists idempotent responses. Implementations must be safe for concurrent use.
type IdempotencyStore interface {
	Get(key string) (IdempotencyRecord, bool, error)
	Set(key string, rec IdempotencyRecord, ttl time.Duration) error
}

// MemoryIdempotencyStore is an in-process store (single instance only).
type MemoryIdempotencyStore struct {
	mu    sync.Mutex
	items map[string]memIdemItem
}

type memIdemItem struct {
	rec IdempotencyRecord
	exp time.Time
}

// NewMemoryIdempotencyStore creates an empty memory store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{items: make(map[string]memIdemItem)}
}

// Get implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Get(key string) (IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[key]
	if !ok {
		return IdempotencyRecord{}, false, nil
	}
	if !it.exp.IsZero() && time.Now().After(it.exp) {
		delete(s.items, key)
		return IdempotencyRecord{}, false, nil
	}
	return it.rec, true, nil
}

// Set implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Set(key string, rec IdempotencyRecord, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	s.items[key] = memIdemItem{rec: rec, exp: exp}
	return nil
}

// IdempotencyConfig configures Idempotency-Key handling for unsafe methods.
type IdempotencyConfig struct {
	Store IdempotencyStore
	// TTL for stored responses (default 24h).
	TTL time.Duration
	// Methods to protect (default POST, PUT, PATCH).
	Methods []string
	// Required when true rejects missing Idempotency-Key on protected methods.
	Required bool
	// KeyFunc builds the storage key (default: tenant + ":" + header value).
	KeyFunc func(c *context.Context, rawKey string) string
	// Skipper skips idempotency when true.
	Skipper func(c *context.Context) bool
}

// Idempotency replays prior responses for the same Idempotency-Key.
// Concurrent first-time requests with the same key: last write wins for the stored body;
// use a distributed lock in Store if you need stricter single-flight semantics.
func Idempotency(cfg IdempotencyConfig) func(c *context.Context) error {
	if cfg.Store == nil {
		panic("middleware: Idempotency Store is required")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}
	if len(cfg.Methods) == 0 {
		cfg.Methods = []string{http.MethodPost, http.MethodPut, http.MethodPatch}
	}
	allowed := make(map[string]bool, len(cfg.Methods))
	for _, m := range cfg.Methods {
		allowed[strings.ToUpper(m)] = true
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *context.Context, rawKey string) string {
			tenant := TenantID(c)
			sum := sha256.Sum256([]byte(c.Request.Method + "\n" + c.Request.URL.Path + "\n" + rawKey))
			h := hex.EncodeToString(sum[:16])
			if tenant != "" {
				return tenant + ":" + h
			}
			return h
		}
	}

	return func(c *context.Context) error {
		if cfg.Skipper != nil && cfg.Skipper(c) {
			c.Next()
			return nil
		}
		if !allowed[c.Request.Method] {
			c.Next()
			return nil
		}

		raw := sanitizeIdempotencyKey(c.Request.Header.Get(IdempotencyKeyHeader))
		if raw == "" {
			if cfg.Required {
				return writeMiddlewareError(c, httperr.BadRequest("idempotency_key_required", "Idempotency-Key header is required"))
			}
			c.Next()
			return nil
		}

		storeKey := cfg.KeyFunc(c, raw)
		if rec, ok, err := cfg.Store.Get(storeKey); err != nil {
			return httperr.Internal("idempotency store unavailable")
		} else if ok {
			replayIdempotency(c, rec)
			return nil
		}

		cw := &captureResponseWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = cw
		c.Next()

		// Cache finished responses (including error JSON) for replay.
		if cw.status > 0 {
			rec := IdempotencyRecord{
				Status:  cw.status,
				Headers: cloneHeader(cw.Header()),
				Body:    append([]byte(nil), cw.body.Bytes()...),
			}
			_ = cfg.Store.Set(storeKey, rec, cfg.TTL)
		}
		return nil
	}
}

func sanitizeIdempotencyKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) > maxIdempotencyKeyLen {
		key = key[:maxIdempotencyKeyLen]
	}
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return ""
	}
	return key
}

func replayIdempotency(c *context.Context, rec IdempotencyRecord) {
	dst := c.Writer.Header()
	for k, vv := range rec.Headers {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
	dst.Set("Idempotent-Replayed", "true")
	c.Writer.WriteHeader(rec.Status)
	if len(rec.Body) > 0 {
		_, _ = c.Writer.Write(rec.Body)
	}
}

func cloneHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vv := range h {
		out[k] = append([]string(nil), vv...)
	}
	return out
}

type captureResponseWriter struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func (w *captureResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	_, _ = w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
