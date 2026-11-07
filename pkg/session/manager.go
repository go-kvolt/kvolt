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
	MaxAge time.Duration

	mu      sync.Mutex
	created map[string]time.Time // token -> createdAt for absolute MaxAge
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
	m.MaxAge = d
	return m
}

// Create creates a new session and returns the token.
func (m *Manager) Create(data interface{}) (string, error) {
	token := uuid.New().String()
	if err := m.store.Set(token, data, m.ttl); err != nil {
		return "", err
	}
	if m.MaxAge > 0 {
		m.mu.Lock()
		m.created[token] = time.Now()
		m.mu.Unlock()
	}
	return token, nil
}

// Get retrieves session data and refreshes sliding TTL without resurrecting deleted keys.
func (m *Manager) Get(token string) (interface{}, error) {
	if m.MaxAge > 0 {
		m.mu.Lock()
		created, ok := m.created[token]
		m.mu.Unlock()
		if ok && time.Since(created) > m.MaxAge {
			_ = m.Destroy(token)
			return nil, ErrSessionNotFound
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
			// Non-fatal: return value even if touch failed for other reasons.
		}
	}

	return val, nil
}

// Destroy removes a session.
func (m *Manager) Destroy(token string) error {
	m.mu.Lock()
	delete(m.created, token)
	m.mu.Unlock()
	return m.store.Delete(token)
}
