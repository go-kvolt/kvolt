// Package redisidem provides a Redis-backed middleware.IdempotencyStore.
// Import explicitly so core middleware does not require Redis for single-instance apps.
package redisidem

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-kvolt/kvolt/v2/middleware"
	goredis "github.com/redis/go-redis/v9"
)

const defaultMaxBodyBytes = 1 << 20 // 1 MiB

// Overridable in tests to exercise marshal failure paths.
var recordMarshal = sonic.Marshal

// Store implements middleware.IdempotencyStore using Redis JSON values.
type Store struct {
	client       *goredis.Client
	prefix       string
	maxBodyBytes int
}

// Option configures Store.
type Option func(*Store)

// WithPrefix sets a key prefix (default "kvolt:idem:").
func WithPrefix(prefix string) Option {
	return func(s *Store) { s.prefix = prefix }
}

// WithMaxBodyBytes caps cached response body size (default 1 MiB). Larger bodies are rejected.
func WithMaxBodyBytes(n int) Option {
	return func(s *Store) {
		if n > 0 {
			s.maxBodyBytes = n
		}
	}
}

// New wraps a go-redis client as an IdempotencyStore.
func New(client *goredis.Client, opts ...Option) *Store {
	if client == nil {
		panic("redisidem: Client required")
	}
	s := &Store{client: client, prefix: "kvolt:idem:", maxBodyBytes: defaultMaxBodyBytes}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type storedRec struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

func (s *Store) key(k string) string { return s.prefix + k }

// Get implements middleware.IdempotencyStore.
func (s *Store) Get(key string) (middleware.IdempotencyRecord, bool, error) {
	data, err := s.client.Get(context.Background(), s.key(key)).Bytes()
	if err == goredis.Nil {
		return middleware.IdempotencyRecord{}, false, nil
	}
	if err != nil {
		return middleware.IdempotencyRecord{}, false, err
	}
	var raw storedRec
	if err := sonic.Unmarshal(data, &raw); err != nil {
		return middleware.IdempotencyRecord{}, false, fmt.Errorf("redisidem unmarshal: %w", err)
	}
	return middleware.IdempotencyRecord{
		Status:  raw.Status,
		Headers: http.Header(raw.Headers),
		Body:    raw.Body,
	}, true, nil
}

// Set implements middleware.IdempotencyStore.
func (s *Store) Set(key string, rec middleware.IdempotencyRecord, ttl time.Duration) error {
	if s.maxBodyBytes > 0 && len(rec.Body) > s.maxBodyBytes {
		return fmt.Errorf("redisidem: body %d exceeds max %d", len(rec.Body), s.maxBodyBytes)
	}
	raw := storedRec{
		Status:  rec.Status,
		Headers: map[string][]string(rec.Headers),
		Body:    rec.Body,
	}
	data, err := recordMarshal(raw)
	if err != nil {
		return fmt.Errorf("redisidem marshal: %w", err)
	}
	ctx := context.Background()
	if ttl > 0 {
		return s.client.Set(ctx, s.key(key), data, ttl).Err()
	}
	return s.client.Set(ctx, s.key(key), data, 0).Err()
}

// Ensure Store satisfies the interface at compile time.
var _ middleware.IdempotencyStore = (*Store)(nil)
