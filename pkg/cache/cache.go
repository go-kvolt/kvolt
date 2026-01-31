package cache

import (
	"errors"
	"time"
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
