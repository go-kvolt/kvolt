package cache

import (
	"errors"
	"fmt"
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
	if store == nil {
		return fmt.Errorf("cache: store is nil")
	}
	if dest == nil {
		return fmt.Errorf("cache: dest is nil")
	}
	val, err := store.Get(key)
	if err != nil {
		return err
	}
	var b []byte
	switch v := val.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		var marshalErr error
		b, marshalErr = sonic.Marshal(val)
		if marshalErr != nil {
			return marshalErr
		}
	}
	return sonic.Unmarshal(b, dest)
}
