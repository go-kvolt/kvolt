package cache

import (
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	c := NewMemoryStore(1 * time.Second)

	// Test Set/Get
	c.Set("foo", "bar", 2*time.Second)
	val, err := c.Get("foo")
	if err != nil || val != "bar" {
		t.Fatalf("expected bar, got %v (err: %v)", val, err)
	}

	// Test Expiration
	time.Sleep(3 * time.Second)
	_, err = c.Get("foo")
	if err != ErrExpired && err != ErrKeyNotFound {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func TestCacheEviction(t *testing.T) {
	c := NewMemoryStoreSized(0, 64)
	for i := 0; i < 400; i++ {
		c.Set(string(rune('a'+i%26))+string(rune(i)), i, 0)
	}
	if c.Len() > 64 {
		t.Fatalf("LRU cap: len %d want <= 64", c.Len())
	}
}

func BenchmarkCacheSet(b *testing.B) {
	c := NewMemoryStore(0)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Set("foo", "bar", time.Minute)
		}
	})
}

func BenchmarkCacheGet(b *testing.B) {
	c := NewMemoryStore(0)
	c.Set("foo", "bar", time.Minute)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Get("foo")
		}
	})
}
