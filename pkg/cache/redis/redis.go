// Package redis provides a Redis-backed cache.Store.
// Import this package only when you need Redis; memory cache stays in pkg/cache.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-kvolt/kvolt/v2/pkg/cache"
	goredis "github.com/redis/go-redis/v9"
)

// Store implements cache.Store using Redis. Values are JSON-encoded with sonic.
type Store struct {
	client *goredis.Client
	prefix string
}

// Option configures Store.
type Option func(*Store)

// WithPrefix sets a key prefix (e.g. "kvolt:"). Required for Flush.
func WithPrefix(prefix string) Option {
	return func(s *Store) { s.prefix = prefix }
}

// New wraps a go-redis client as a cache.Store.
func New(client *goredis.Client, opts ...Option) *Store {
	s := &Store{client: client}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Store) key(k string) string { return s.prefix + k }

// Get retrieves and JSON-decodes a value.
func (s *Store) Get(key string) (interface{}, error) {
	data, err := s.client.Get(context.Background(), s.key(key)).Bytes()
	if err == goredis.Nil {
		return nil, cache.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	var v interface{}
	if err := sonic.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cache redis unmarshal: %w", err)
	}
	return v, nil
}

// Set JSON-encodes and stores a value with optional TTL (0 = no expiry).
func (s *Store) Set(key string, value interface{}, ttl time.Duration) error {
	data, err := sonic.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache redis marshal: %w", err)
	}
	return s.client.Set(context.Background(), s.key(key), data, ttl).Err()
}

// Delete removes a key.
func (s *Store) Delete(key string) error {
	return s.client.Del(context.Background(), s.key(key)).Err()
}

// Flush deletes all keys with this store's prefix. Empty prefix is rejected.
func (s *Store) Flush() error {
	if s.prefix == "" {
		return fmt.Errorf("cache redis: Flush requires a key prefix to avoid wiping the whole DB")
	}
	ctx := context.Background()
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, s.prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Ensure Store implements cache.Store.
var _ cache.Store = (*Store)(nil)
