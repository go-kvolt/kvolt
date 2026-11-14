package middleware

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// IdempotencyKeyHeader is the standard request header.
	IdempotencyKeyHeader      = "Idempotency-Key"
	maxIdempotencyKeyLen      = 128
	defaultIdempotencyMaxKeys = 10_000
)

// IdempotencyRecord is a cached response for a key (2xx/4xx only; 5xx are not stored).
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
	mu      sync.Mutex
	items   map[string]memIdemItem
	maxKeys int
}

type memIdemItem struct {
	rec IdempotencyRecord
	exp time.Time
}

// NewMemoryIdempotencyStore creates an empty memory store (default max 10_000 keys).
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{items: make(map[string]memIdemItem), maxKeys: defaultIdempotencyMaxKeys}
}

// WithMaxKeys caps distinct keys (evicts expired then oldest). Returns the same store.
func (s *MemoryIdempotencyStore) WithMaxKeys(n int) *MemoryIdempotencyStore {
	if n > 0 {
		s.maxKeys = n
	}
	return s
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
	now := time.Now()
	for k, it := range s.items {
		if !it.exp.IsZero() && now.After(it.exp) {
			delete(s.items, k)
		}
	}
	for len(s.items) >= s.maxKeys {
		var oldestKey string
		var oldestExp time.Time
		first := true
		for k, it := range s.items {
			t := it.exp
			if t.IsZero() {
				t = now
			}
			if first || t.Before(oldestExp) {
				oldestKey = k
				oldestExp = t
				first = false
			}
		}
		if oldestKey == "" {
			break
		}
		delete(s.items, oldestKey)
	}
	exp := time.Time{}
	if ttl > 0 {
		exp = now.Add(ttl)
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
	// IncludeBodyHash folds a SHA-256 of the request body into the default storage key (default true).
	// Set false only when body may be unbound or streaming.
	IncludeBodyHash *bool
	// KeyFunc builds the storage key (default: tenant + actor + method/path/body/key hash).
	KeyFunc func(c *context.Context, rawKey string) string
	// Skipper skips idempotency when true.
	Skipper func(c *context.Context) bool
}

func idempotencyIncludeBody(cfg IdempotencyConfig) bool {
	if cfg.IncludeBodyHash == nil {
		return true
	}
	return *cfg.IncludeBodyHash
}

// Idempotency replays prior responses for the same Idempotency-Key.
// Same-process concurrent first requests wait on a single-flight gate; use a
// distributed lock in Store for multi-instance strictness.
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
		includeBody := idempotencyIncludeBody(cfg)
		cfg.KeyFunc = func(c *context.Context, rawKey string) string {
			return defaultIdempotencyKey(c, rawKey, includeBody)
		}
	}

	var inflight sync.Map // storeKey -> chan struct{}

	return func(c *context.Context) error {
		if cfg.Skipper != nil && cfg.Skipper(c) {
			c.Next()
			return nil
		}
		if !allowed[c.Request.Method] {
			c.Next()
			return nil
		}

		raw, keyOK := sanitizeIdempotencyKey(c.Request.Header.Get(IdempotencyKeyHeader))
		if !keyOK {
			return writeMiddlewareError(c, httperr.BadRequest("idempotency_key_invalid", "Idempotency-Key is invalid or too long"))
		}
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

		ch := make(chan struct{})
		if actual, loaded := inflight.LoadOrStore(storeKey, ch); loaded {
			<-actual.(chan struct{})
			if rec, ok, err := cfg.Store.Get(storeKey); err != nil {
				return httperr.Internal("idempotency store unavailable")
			} else if ok {
				replayIdempotency(c, rec)
				return nil
			}
			// Leader failed without caching (e.g. 5xx); fall through and run.
		} else {
			defer func() {
				close(ch)
				inflight.Delete(storeKey)
			}()
		}

		cw := &captureResponseWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = cw
		c.Next()

		// Do not cache 5xx — retries should re-execute after transient failures.
		if cw.status > 0 && cw.status < 500 {
			rec := IdempotencyRecord{
				Status:  cw.status,
				Headers: sanitizeIdempotencyHeaders(cw.Header()),
				Body:    append([]byte(nil), cw.body.Bytes()...),
			}
			_ = cfg.Store.Set(storeKey, rec, cfg.TTL)
		}
		return nil
	}
}

func defaultIdempotencyKey(c *context.Context, rawKey string, includeBody bool) string {
	tenant := TenantID(c)
	actor := idempotencyActor(c)
	var b strings.Builder
	b.WriteString(c.Request.Method)
	b.WriteByte('\n')
	b.WriteString(c.Request.URL.Path)
	b.WriteByte('\n')
	b.WriteString(actor)
	b.WriteByte('\n')
	b.WriteString(rawKey)
	if includeBody {
		b.WriteByte('\n')
		if raw, err := c.RawBody(); err == nil {
			sum := sha256.Sum256(raw)
			b.WriteString(hex.EncodeToString(sum[:]))
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	h := hex.EncodeToString(sum[:16])
	parts := make([]string, 0, 3)
	if tenant != "" {
		parts = append(parts, tenant)
	}
	if actor != "" {
		parts = append(parts, actor)
	}
	parts = append(parts, h)
	return strings.Join(parts, ":")
}

func idempotencyActor(c *context.Context) string {
	raw, ok := c.Get("user")
	if !ok || raw == nil {
		return ""
	}
	switch claims := raw.(type) {
	case jwt.MapClaims:
		for _, k := range []string{"sub", "user_id", "uid", "email"} {
			if v, ok := claims[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	case map[string]interface{}:
		for _, k := range []string{"sub", "user_id", "uid", "email"} {
			if v, ok := claims[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// sanitizeIdempotencyKey returns the key and whether it is acceptable.
// Keys longer than maxIdempotencyKeyLen are rejected (not truncated).
func sanitizeIdempotencyKey(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", true
	}
	if len(key) > maxIdempotencyKeyLen {
		return "", false
	}
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return "", false
	}
	return key, true
}

func sanitizeIdempotencyHeaders(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vv := range h {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Set-Cookie") {
			continue
		}
		out[k] = append([]string(nil), vv...)
	}
	return out
}

func replayIdempotency(c *context.Context, rec IdempotencyRecord) {
	dst := c.Writer.Header()
	for k, vv := range rec.Headers {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Set-Cookie") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
	dst.Set("Idempotent-Replayed", "true")
	_ = c.Data(rec.Status, "", rec.Body)
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

func (w *captureResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *captureResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("idempotency: underlying ResponseWriter does not support Hijacker")
	}
	return h.Hijack()
}

func (w *captureResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
