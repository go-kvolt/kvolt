package cache

import (
	"math"
	"testing"
	"time"
)

func TestGetInto_AndCRUD(t *testing.T) {
	s := NewMemoryStoreSized(0, 8)

	if err := s.Set("user", map[string]string{"name": "ada"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	var dest map[string]string
	if err := GetInto(s, "user", &dest); err != nil {
		t.Fatal(err)
	}
	if dest["name"] != "ada" {
		t.Fatalf("dest=%v", dest)
	}
	if err := GetInto(s, "missing", &dest); err != ErrKeyNotFound {
		t.Fatalf("want ErrKeyNotFound got %v", err)
	}

	if err := s.Touch("user", 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := s.Touch("user", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.Touch("missing", time.Second); err != ErrKeyNotFound {
		t.Fatalf("touch missing: %v", err)
	}

	if err := s.Delete("user"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("user"); err != ErrKeyNotFound {
		t.Fatalf("after delete: %v", err)
	}
	_ = s.Set("a", 1, 0)
	_ = s.Set("b", 2, 0)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 0 {
		t.Fatalf("len after flush=%d", s.Len())
	}
}

func TestMemoryStore_TouchExpired(t *testing.T) {
	s := NewMemoryStoreSized(0, 8)
	_ = s.Set("ttl", "x", 5*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	err := s.Touch("ttl", time.Second)
	if err != ErrExpired && err != ErrKeyNotFound {
		t.Fatalf("touch expired: %v", err)
	}
}

func TestMemoryStore_UpdateExistingAndEvict(t *testing.T) {
	s := NewMemoryStoreSized(0, 2)
	_ = s.Set("k1", "v1", 0)
	_ = s.Set("k1", "v1b", 0)
	v, err := s.Get("k1")
	if err != nil || v != "v1b" {
		t.Fatalf("got %v %v", v, err)
	}
	for i := 0; i < 200; i++ {
		_ = s.Set("n"+string(rune('a'+i%26))+string(rune(i)), i, 0)
	}
	if s.Len() > 64 {
		t.Fatalf("len too large: %d", s.Len())
	}
}

func TestMemoryStore_JanitorCleansExpired(t *testing.T) {
	s := NewMemoryStoreSized(15*time.Millisecond, 100)
	_ = s.Set("e", "gone", 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	if _, err := s.Get("e"); err != ErrKeyNotFound && err != ErrExpired {
		t.Fatalf("janitor/get: %v", err)
	}
}

func TestNewMemoryStoreSized_Unlimited(t *testing.T) {
	s := NewMemoryStoreSized(0, 0)
	for i := 0; i < 10; i++ {
		_ = s.Set(string(rune('a'+i)), i, 0)
	}
	if s.Len() != 10 {
		t.Fatalf("len=%d", s.Len())
	}
}

func TestGetInto_MarshalFailure(t *testing.T) {
	s := NewMemoryStoreSized(0, 8)
	// complex128 is unsupported by sonic/json (unlike chan, which can hang).
	if err := s.Set("bad", complex(1, 2), 0); err != nil {
		t.Fatal(err)
	}
	var dest string
	if err := GetInto(s, "bad", &dest); err == nil {
		t.Fatal("want marshal error")
	}
}

func TestMemoryStore_GetExpiredAndDeleteMiss(t *testing.T) {
	s := NewMemoryStoreSized(0, 8) // no janitor
	_ = s.Set("ttl", "x", 5*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, err := s.Get("ttl"); err != ErrExpired {
		t.Fatalf("want ErrExpired, got %v", err)
	}
	if err := s.Delete("missing"); err != nil {
		t.Fatal(err)
	}
}

func TestNewMemoryStoreSized_PerShardFloor(t *testing.T) {
	// Overflow (maxKeys+63) so perShard computes < 1 and hits the floor branch.
	s := NewMemoryStoreSized(0, math.MaxInt)
	if err := s.Set("k", 1, 0); err != nil {
		t.Fatal(err)
	}
}

func TestEvictLocked_EmptyLRU(t *testing.T) {
	s := NewMemoryStoreSized(0, 64)
	sh := s.shards[0]
	sh.mu.Lock()
	sh.max = 1
	sh.items["orphan"] = &item{key: "orphan", value: 1} // no LRU elem
	sh.evictLocked()
	sh.mu.Unlock()
}
