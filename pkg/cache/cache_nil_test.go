package cache

import (
	"testing"
	"time"
)

func TestGetInto_NilGuards(t *testing.T) {
	if err := GetInto(nil, "key", &struct{}{}); err == nil {
		t.Fatal("expected error on nil store")
	}

	store := NewMemoryStore(0)
	if err := GetInto(store, "key", nil); err == nil {
		t.Fatal("expected error on nil dest")
	}
}

func TestMemoryStore_Uninitialized(t *testing.T) {
	var m *MemoryStore
	if _, err := m.Get("key"); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
	if err := m.Set("key", "val", time.Hour); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
	if err := m.Delete("key"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := m.Touch("key", time.Hour); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}

	m2 := &MemoryStore{}
	if _, err := m2.Get("key"); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
	if err := m2.Set("key", "val", time.Hour); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
	if err := m2.Delete("key"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := m2.Touch("key", time.Hour); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}
