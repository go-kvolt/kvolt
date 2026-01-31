package cache

import (
	"hash/fnv"
	"sync"
	"time"
)

const shardCount = 64

type item struct {
	value     interface{}
	expiresAt int64
}

// MemoryStore is a blazing fast sharded in-memory cache.
type MemoryStore struct {
	shards []*shard
}

type shard struct {
	items map[string]item
	mu    sync.RWMutex
}

// NewMemoryStore creates a new sharded memory store.
// cleanupInterval: How often to remove expired items.
func NewMemoryStore(cleanupInterval time.Duration) *MemoryStore {
	m := &MemoryStore{
		shards: make([]*shard, shardCount),
	}

	for i := 0; i < shardCount; i++ {
		m.shards[i] = &shard{
			items: make(map[string]item),
		}
	}

	if cleanupInterval > 0 {
		go m.janitor(cleanupInterval)
	}

	return m
}

func (m *MemoryStore) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return m.shards[uint(h.Sum32())%shardCount]
}

// Get retrieves a value.
func (m *MemoryStore) Get(key string) (interface{}, error) {
	s := m.getShard(key)
	s.mu.RLock()
	defer s.mu.RUnlock()

	i, exists := s.items[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	if i.expiresAt > 0 && time.Now().UnixNano() > i.expiresAt {
		return nil, ErrExpired
	}

	return i.value, nil
}

// Set stores a value.
func (m *MemoryStore) Set(key string, value interface{}, ttl time.Duration) error {
	s := m.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	var expires int64
	if ttl > 0 {
		expires = time.Now().Add(ttl).UnixNano()
	}

	s.items[key] = item{
		value:     value,
		expiresAt: expires,
	}

	return nil
}

// Delete removes a key.
func (m *MemoryStore) Delete(key string) error {
	s := m.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, key)
	return nil
}

// Flush clears everything.
func (m *MemoryStore) Flush() error {
	for _, s := range m.shards {
		s.mu.Lock()
		s.items = make(map[string]item)
		s.mu.Unlock()
	}
	return nil
}

func (m *MemoryStore) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now().UnixNano()
		for _, s := range m.shards {
			s.mu.Lock()
			for k, v := range s.items {
				if v.expiresAt > 0 && now > v.expiresAt {
					delete(s.items, k)
				}
			}
			s.mu.Unlock()
		}
	}
}
