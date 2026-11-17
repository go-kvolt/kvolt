package session

import (
	"errors"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/pkg/cache"
)

// failingStore wraps a store and fails selected operations.
type failingStore struct {
	cache.Store
	failSetKey func(key string) bool
	failGetKey func(key string) bool
}

var errStoreDown = errors.New("store down")

func (f *failingStore) Set(key string, v interface{}, ttl time.Duration) error {
	if f.failSetKey != nil && f.failSetKey(key) {
		return errStoreDown
	}
	return f.Store.Set(key, v, ttl)
}

func (f *failingStore) Get(key string) (interface{}, error) {
	if f.failGetKey != nil && f.failGetKey(key) {
		return nil, errStoreDown
	}
	return f.Store.Get(key)
}

// A second Manager on the same store stands in for a restarted process or another
// instance: it has no in-memory creation times, yet MaxAge must still apply.
func TestMaxAge_EnforcedByAnotherManagerOnSameStore(t *testing.T) {
	store := cache.NewMemoryStore(0)
	defer store.Close()

	first := New(store, time.Hour).WithMaxAge(80 * time.Millisecond)
	token, err := first.Create("alice")
	if err != nil {
		t.Fatal(err)
	}

	second := New(store, time.Hour).WithMaxAge(80 * time.Millisecond)
	if got, err := second.Get(token); err != nil || got != "alice" {
		t.Fatalf("within MaxAge: Get = %v, %v; want alice", got, err)
	}
	// The second read uses the creation time cached from the marker.
	if got, err := second.Get(token); err != nil || got != "alice" {
		t.Fatalf("within MaxAge (cached): Get = %v, %v; want alice", got, err)
	}

	time.Sleep(120 * time.Millisecond)

	third := New(store, time.Hour).WithMaxAge(80 * time.Millisecond)
	if _, err := third.Get(token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("past MaxAge on a fresh manager: err = %v, want ErrSessionNotFound", err)
	}
	if _, err := second.Get(token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("past MaxAge on the caching manager: err = %v, want ErrSessionNotFound", err)
	}
	// The session data is gone from the store, not just hidden.
	if _, err := store.Get(token); err == nil {
		t.Fatalf("expired session still in the store")
	}
}

// A session with no marker (created while MaxAge was off) is past its lifetime once
// MaxAge is on.
func TestMaxAge_SessionWithoutMarkerIsRejected(t *testing.T) {
	store := cache.NewMemoryStore(0)
	defer store.Close()

	token, err := New(store, time.Hour).Create("legacy")
	if err != nil {
		t.Fatal(err)
	}
	capped := New(store, time.Hour).WithMaxAge(time.Hour)
	if _, err := capped.Get(token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
	if _, err := store.Get(token); err == nil {
		t.Fatalf("rejected session still in the store")
	}
}

// Sliding TTL refreshes must not extend the marker: MaxAge is absolute.
func TestMaxAge_NotExtendedByActivity(t *testing.T) {
	store := cache.NewMemoryStore(0)
	defer store.Close()

	m := New(store, time.Hour).WithMaxAge(150 * time.Millisecond)
	token, err := m.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		// A fresh manager each time forces the marker path rather than the local cache.
		if _, err := New(store, time.Hour).WithMaxAge(150 * time.Millisecond).Get(token); err != nil {
			if !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("session kept alive past MaxAge by repeated reads")
}

func TestMaxAge_DestroyRemovesMarker(t *testing.T) {
	store := cache.NewMemoryStore(0)
	defer store.Close()

	m := New(store, time.Hour).WithMaxAge(time.Hour)
	token, err := m.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(maxAgeKey(token)); err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	if err := m.Destroy(token); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(maxAgeKey(token)); err == nil {
		t.Fatalf("marker left behind after Destroy")
	}
}

func TestMaxAge_StoreErrors(t *testing.T) {
	base := cache.NewMemoryStore(0)
	defer base.Close()

	// Marker write fails: Create fails and leaves no session behind.
	fs := &failingStore{Store: base, failSetKey: func(k string) bool { return len(k) > 8 && k[len(k)-8:] == ":created" }}
	if _, err := New(fs, time.Hour).WithMaxAge(time.Hour).Create("x"); !errors.Is(err, errStoreDown) {
		t.Fatalf("Create err = %v, want store error", err)
	}
	if base.Len() != 0 {
		t.Fatalf("store holds %d keys after a failed Create, want 0", base.Len())
	}

	// Marker read fails: the error is reported, the session is not silently accepted.
	token, err := New(base, time.Hour).WithMaxAge(time.Hour).Create("y")
	if err != nil {
		t.Fatal(err)
	}
	fg := &failingStore{Store: base, failGetKey: func(k string) bool { return k == maxAgeKey(token) }}
	if _, err := New(fg, time.Hour).WithMaxAge(time.Hour).Get(token); !errors.Is(err, errStoreDown) {
		t.Fatalf("Get err = %v, want store error", err)
	}
}

func TestCreatedAt(t *testing.T) {
	for _, v := range []interface{}{int64(1700000000000), float64(1700000000000), int(1700000000000)} {
		got, ok := createdAt(v)
		if !ok || got.Unix() != 1700000000 {
			t.Errorf("createdAt(%T) = %v, %v", v, got, ok)
		}
	}
	if _, ok := createdAt("1700000000000"); ok {
		t.Errorf("createdAt accepted a string")
	}

	// An unreadable marker is still bounded by its store TTL, so the session is accepted.
	store := cache.NewMemoryStore(0)
	defer store.Close()
	m := New(store, time.Hour).WithMaxAge(time.Hour)
	token, err := m.Create("z")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Set(maxAgeKey(token), "garbage", time.Hour)
	if got, err := New(store, time.Hour).WithMaxAge(time.Hour).Get(token); err != nil || got != "z" {
		t.Fatalf("Get = %v, %v; want z", got, err)
	}
}
