package session

import (
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/pkg/cache"
)

func TestManager_CreateGetDestroy(t *testing.T) {
	store := cache.NewMemoryStore(time.Minute)
	manager := New(store, time.Hour)

	token, err := manager.Create(map[string]string{"user": "alice"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("Create: token empty")
	}

	data, err := manager.Get(token)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	m, ok := data.(map[string]string)
	if !ok {
		t.Fatalf("Get: type %T", data)
	}
	if m["user"] != "alice" {
		t.Errorf("Get: want user=alice, got %v", m["user"])
	}

	err = manager.Destroy(token)
	if err != nil {
		t.Errorf("Destroy: %v", err)
	}

	_, err = manager.Get(token)
	if err == nil {
		t.Error("Get after Destroy: want error")
	}
	if err != nil && err != ErrSessionNotFound {
		t.Errorf("Get after Destroy: want ErrSessionNotFound, got %v", err)
	}
}

func TestManager_GetInvalidToken(t *testing.T) {
	store := cache.NewMemoryStore(time.Minute)
	manager := New(store, time.Hour)

	_, err := manager.Get("nonexistent-token")
	if err != ErrSessionNotFound {
		t.Errorf("Get invalid: want ErrSessionNotFound, got %v", err)
	}
}
