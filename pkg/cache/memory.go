package cache

import (
	"container/list"
	"hash/maphash"
	"sync"
	"time"
)

const shardCount = 64

// DefaultMaxKeys is the production cap for NewMemoryStore (about 780 keys per shard).
const DefaultMaxKeys = 50_000

type item struct {
	key       string
	value     interface{}
	expiresAt int64
	elem      *list.Element
}

// MemoryStore is a sharded in-memory cache with LRU eviction.
type MemoryStore struct {
	shards  []*shard
	maxKeys int
	seed    maphash.Seed
}

type shard struct {
	items map[string]*item
	lru   *list.List
	max   int
	mu    sync.Mutex
}

// NewMemoryStore creates a sharded store capped at DefaultMaxKeys.
// cleanupInterval: how often to remove expired items. 0 disables the janitor.
func NewMemoryStore(cleanupInterval time.Duration) *MemoryStore {
	return NewMemoryStoreSized(cleanupInterval, DefaultMaxKeys)
}

// NewMemoryStoreSized creates a store with an explicit key cap.
// maxKeys <= 0 means unlimited (tests / special cases only).
func NewMemoryStoreSized(cleanupInterval time.Duration, maxKeys int) *MemoryStore {
	perShard := 0
	if maxKeys > 0 {
		perShard = (maxKeys + shardCount - 1) / shardCount
		if perShard < 1 {
			perShard = 1
		}
	}
	m := &MemoryStore{
		shards:  make([]*shard, shardCount),
		maxKeys: maxKeys,
		seed:    maphash.MakeSeed(),
	}
	for i := 0; i < shardCount; i++ {
		m.shards[i] = &shard{
			items: make(map[string]*item),
			lru:   list.New(),
			max:   perShard,
		}
	}
	if cleanupInterval > 0 {
		go m.janitor(cleanupInterval)
	}
	return m
}

func (m *MemoryStore) getShard(key string) *shard {
	var h maphash.Hash
	h.SetSeed(m.seed)
	h.WriteString(key)
	return m.shards[uint(h.Sum64()%shardCount)]
}

func (s *shard) removeLocked(key string) {
	it, ok := s.items[key]
	if !ok {
		return
	}
	if it.elem != nil {
		s.lru.Remove(it.elem)
	}
	delete(s.items, key)
}

func (s *shard) touchLocked(it *item) {
	if it.elem != nil {
		s.lru.MoveToFront(it.elem)
	}
}

func (s *shard) evictLocked() {
	if s.max <= 0 {
		return
	}
	for len(s.items) >= s.max {
		back := s.lru.Back()
		if back == nil {
			return
		}
		it := back.Value.(*item)
		s.removeLocked(it.key)
	}
}

// Get retrieves a value.
func (m *MemoryStore) Get(key string) (interface{}, error) {
	s := m.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	it, exists := s.items[key]
	if !exists {
		return nil, ErrKeyNotFound
	}
	if it.expiresAt > 0 && time.Now().UnixNano() > it.expiresAt {
		s.removeLocked(key)
		return nil, ErrExpired
	}
	s.touchLocked(it)
	return it.value, nil
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

	if it, exists := s.items[key]; exists {
		it.value = value
		it.expiresAt = expires
		s.touchLocked(it)
		return nil
	}

	s.evictLocked()
	it := &item{key: key, value: value, expiresAt: expires}
	it.elem = s.lru.PushFront(it)
	s.items[key] = it
	return nil
}

// Delete removes a key.
func (m *MemoryStore) Delete(key string) error {
	s := m.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(key)
	return nil
}

// Flush clears everything.
func (m *MemoryStore) Flush() error {
	for _, s := range m.shards {
		s.mu.Lock()
		s.items = make(map[string]*item)
		s.lru = list.New()
		s.mu.Unlock()
	}
	return nil
}

func (m *MemoryStore) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().UnixNano()
		for _, s := range m.shards {
			s.mu.Lock()
			for k, it := range s.items {
				if it.expiresAt > 0 && now > it.expiresAt {
					s.removeLocked(k)
				}
			}
			s.mu.Unlock()
		}
	}
}

// Len returns the number of live keys (not including expired until janitor/Get).
func (m *MemoryStore) Len() int {
	n := 0
	for _, s := range m.shards {
		s.mu.Lock()
		n += len(s.items)
		s.mu.Unlock()
	}
	return n
}
