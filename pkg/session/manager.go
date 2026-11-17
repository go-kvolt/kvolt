package session

import (
	"errors"
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/v2/pkg/cache"
	"github.com/google/uuid"
)

var (
	ErrSessionNotFound = errors.New("session not found")
)

// Toucher optionally extends TTL only if the key still exists (no resurrect-on-Set).
type Toucher interface {
	Touch(key string, ttl time.Duration) error
}

// Manager handles session creation and retrieval.
type Manager struct {
	store cache.Store
	ttl   time.Duration
	// MaxAge is an absolute lifetime from Create (0 = sliding only, no absolute cap).
	//
	// The creation time is kept in the store next to the session, under a marker key
	// that expires after MaxAge and is never refreshed. The cap therefore holds across
	// restarts and across instances sharing a store. A session with no marker (created
	// while MaxAge was 0, or by a version that kept the time only in memory) is treated
	// as past its lifetime.
	MaxAge time.Duration

	mu        sync.Mutex
	created   map[string]time.Time // token -> createdAt: local cache of the store marker
	lastPrune time.Time
}

// New creates a new Session Manager.
func New(store cache.Store, ttl time.Duration) *Manager {
	return &Manager{
		store:   store,
		ttl:     ttl,
		created: make(map[string]time.Time),
	}
}

// WithMaxAge sets an absolute session lifetime independent of sliding TTL.
func (m *Manager) WithMaxAge(d time.Duration) *Manager {
	if m == nil {
		return nil
	}
	m.MaxAge = d
	return m
}

// Create creates a new session and returns the token.
func (m *Manager) Create(data interface{}) (string, error) {
	if m == nil || m.store == nil {
		return "", errors.New("nil session manager or store")
	}
	token := uuid.New().String()
	if err := m.store.Set(token, data, m.ttl); err != nil {
		return "", err
	}
	if m.MaxAge > 0 {
		now := time.Now()
		if err := m.store.Set(maxAgeKey(token), now.UnixMilli(), m.MaxAge); err != nil {
			_ = m.store.Delete(token)
			return "", err
		}
		m.mu.Lock()
		if m.created == nil {
			m.created = make(map[string]time.Time)
		}
		stale := m.pruneCreatedLocked(now)
		m.created[token] = now
		m.mu.Unlock()
		m.deleteStoreKeys(stale)
	}
	return token, nil
}

// maxAgeKey is the store key of a session's creation marker.
func maxAgeKey(token string) string { return token + ":created" }

// createdAt decodes a marker value (Unix milliseconds). Stores return it as written
// (int64) or, after a JSON round trip, as float64.
func createdAt(v interface{}) (time.Time, bool) {
	switch n := v.(type) {
	case int64:
		return time.UnixMilli(n), true
	case float64:
		return time.UnixMilli(int64(n)), true
	case int:
		return time.UnixMilli(int64(n)), true
	}
	return time.Time{}, false
}

// checkMarker enforces MaxAge for a token this process has no creation time for
// (created before a restart, or by another instance). The marker expires in the store
// after MaxAge, so a missing marker means the absolute lifetime is over.
func (m *Manager) checkMarker(token string) error {
	v, err := m.store.Get(maxAgeKey(token))
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) || errors.Is(err, cache.ErrExpired) {
			_ = m.store.Delete(token)
			return ErrSessionNotFound
		}
		return err
	}
	created, ok := createdAt(v)
	if !ok {
		return nil // marker present but unreadable: the store TTL still bounds it
	}
	if time.Since(created) > m.MaxAge {
		_ = m.Destroy(token)
		return ErrSessionNotFound
	}
	m.mu.Lock()
	if m.created == nil {
		m.created = make(map[string]time.Time)
	}
	m.created[token] = created
	m.mu.Unlock()
	return nil
}

// Get retrieves session data and refreshes sliding TTL without resurrecting deleted keys.
func (m *Manager) Get(token string) (interface{}, error) {
	if m == nil || m.store == nil {
		return nil, errors.New("nil session manager or store")
	}
	if m.MaxAge > 0 {
		m.mu.Lock()
		created, ok := m.created[token]
		expired := ok && time.Since(created) > m.MaxAge
		stale := m.pruneCreatedLocked(time.Now())
		m.mu.Unlock()
		m.deleteStoreKeys(stale)
		if expired {
			_ = m.Destroy(token)
			return nil, ErrSessionNotFound
		}
		if !ok {
			if err := m.checkMarker(token); err != nil {
				return nil, err
			}
		}
	}

	val, err := m.store.Get(token)
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) || errors.Is(err, cache.ErrExpired) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	// Sliding TTL: Touch only (never Set) so Destroy cannot race into a resurrected session.
	if t, ok := m.store.(Toucher); ok {
		if err := t.Touch(token, m.ttl); err != nil {
			if errors.Is(err, cache.ErrKeyNotFound) || errors.Is(err, cache.ErrExpired) {
				return nil, ErrSessionNotFound
			}
		}
	}

	return val, nil
}

// Destroy removes a session.
func (m *Manager) Destroy(token string) error {
	if m == nil || m.store == nil {
		return errors.New("nil session manager or store")
	}
	m.mu.Lock()
	delete(m.created, token)
	m.mu.Unlock()
	_ = m.store.Delete(maxAgeKey(token))
	return m.store.Delete(token)
}

// pruneCreatedLocked removes expired MaxAge bookkeeping entries and returns their tokens.
func (m *Manager) pruneCreatedLocked(now time.Time) []string {
	if m.MaxAge <= 0 || len(m.created) == 0 {
		return nil
	}
	throttle := m.MaxAge
	if throttle > time.Second {
		throttle = time.Second
	}
	if !m.lastPrune.IsZero() && now.Sub(m.lastPrune) < throttle {
		return nil
	}
	m.lastPrune = now
	var stale []string
	for tok, created := range m.created {
		if now.Sub(created) > m.MaxAge {
			delete(m.created, tok)
			stale = append(stale, tok)
		}
	}
	return stale
}

func (m *Manager) deleteStoreKeys(keys []string) {
	for _, k := range keys {
		_ = m.store.Delete(maxAgeKey(k))
		_ = m.store.Delete(k)
	}
}
