package session

import (
	"errors"
	"time"

	"github.com/go-kvolt/kvolt/pkg/cache"
	"github.com/google/uuid"
)

var (
	ErrSessionNotFound = errors.New("session not found")
)

// Manager handles session creation and retrieval.
type Manager struct {
	store cache.Store
	ttl   time.Duration
}

// New creates a new Session Manager.
func New(store cache.Store, ttl time.Duration) *Manager {
	return &Manager{
		store: store,
		ttl:   ttl,
	}
}

// Create creates a new session and returns the token.
func (m *Manager) Create(data interface{}) (string, error) {
	token := uuid.New().String()
	if err := m.store.Set(token, data, m.ttl); err != nil {
		return "", err
	}
	return token, nil
}

// Get retrieves session data.
func (m *Manager) Get(token string) (interface{}, error) {
	val, err := m.store.Get(token)
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) || errors.Is(err, cache.ErrExpired) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	// Extend TTL on access (sliding window) - Optional, but good for sessions
	// We ignore error here as it's not critical if extension fails
	_ = m.store.Set(token, val, m.ttl)

	return val, nil
}

// Destroy removes a session.
func (m *Manager) Destroy(token string) error {
	return m.store.Delete(token)
}
