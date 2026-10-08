package cache

import (
	"errors"
	"time"

	"github.com/bytedance/sonic"
)

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrExpired     = errors.New("key expired")
)

// Store is the interface for caching systems.
type Store interface {
	// Get retrieves a value from the cache.
	Get(key string) (interface{}, error)

	// Set places a value in the cache with a TTL.
	Set(key string, value interface{}, ttl time.Duration) error

	// Delete removes a key from the cache.
	Delete(key string) error

	// Flush clears all keys from the cache.
	Flush() error
}

// GetInto loads a key and JSON-decodes it into dest (useful for Redis round-trips).
// Memory stores that hold native values will re-marshal via JSON.
func GetInto(store Store, key string, dest interface{}) error {
	val, err := store.Get(key)
	if err != nil {
		return err
	}
	b, err := sonic.Marshal(val)
	if err != nil {
		return err
	}
	return sonic.Unmarshal(b, dest)
}
