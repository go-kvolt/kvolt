package session

import (
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/pkg/cache"
)

func TestManager_NilMapSafe(t *testing.T) {
	store := cache.NewMemoryStore(0)
	// Directly constructed struct without New() has nil created map
	m := &Manager{
		store:  store,
		ttl:    time.Minute,
		MaxAge: time.Hour,
	}

	tok, err := m.Create("test-payload")
	if err != nil {
		t.Fatalf("unexpected Create error: %v", err)
	}

	val, err := m.Get(tok)
	if err != nil || val != "test-payload" {
		t.Fatalf("unexpected Get error: %v, val: %v", err, val)
	}

	if err := m.Destroy(tok); err != nil {
		t.Fatalf("unexpected Destroy error: %v", err)
	}
}
