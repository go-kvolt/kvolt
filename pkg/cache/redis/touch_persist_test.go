package redis

import (
	"errors"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/pkg/cache"
	"github.com/go-kvolt/kvolt/v2/pkg/session"
)

// Touch with no TTL on a key that already has no expiry must succeed: Redis PERSIST
// answers 0 for that key as well as for a missing one.
func TestStore_TouchNoTTLOnPersistentKey(t *testing.T) {
	s, mr := newTestStore(t)
	defer mr.Close()

	if err := s.Set("persistent", "v", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.Touch("persistent", 0); err != nil {
		t.Fatalf("Touch on a key without expiry: %v", err)
	}

	if err := s.Set("expiring", "v", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := s.Touch("expiring", 0); err != nil {
		t.Fatalf("Touch removing an expiry: %v", err)
	}
	if ttl := mr.TTL(s.key("expiring")); ttl != 0 {
		t.Fatalf("expiry not removed, ttl = %v", ttl)
	}

	if err := s.Touch("missing", 0); !errors.Is(err, cache.ErrKeyNotFound) {
		t.Fatalf("Touch on a missing key = %v, want ErrKeyNotFound", err)
	}

	mr.Close()
	if err := s.Touch("persistent", 0); err == nil || errors.Is(err, cache.ErrKeyNotFound) {
		t.Fatalf("Touch with Redis down = %v, want a connection error", err)
	}
}

// A session with no TTL stored in Redis must stay readable.
func TestStore_SessionWithoutTTL(t *testing.T) {
	s, mr := newTestStore(t)
	defer mr.Close()

	m := session.New(s, 0)
	token, err := m.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := m.Get(token)
		if err != nil || got != "alice" {
			t.Fatalf("Get #%d = %v, %v; want alice", i, got, err)
		}
	}
}

// MaxAge is enforced from the marker stored in Redis, so it holds for a manager that
// did not create the session (another instance, or the same one after a restart).
func TestStore_SessionMaxAgeAcrossManagers(t *testing.T) {
	s, mr := newTestStore(t)
	defer mr.Close()

	token, err := session.New(s, time.Hour).WithMaxAge(10 * time.Minute).Create("alice")
	if err != nil {
		t.Fatal(err)
	}

	other := session.New(s, time.Hour).WithMaxAge(10 * time.Minute)
	if got, err := other.Get(token); err != nil || got != "alice" {
		t.Fatalf("within MaxAge: Get = %v, %v; want alice", got, err)
	}

	mr.FastForward(11 * time.Minute)

	fresh := session.New(s, time.Hour).WithMaxAge(10 * time.Minute)
	if _, err := fresh.Get(token); !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("past MaxAge: err = %v, want ErrSessionNotFound", err)
	}
	if mr.Exists(s.key(token)) {
		t.Fatalf("expired session still in Redis")
	}
}
