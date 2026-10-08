package redis

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/pkg/cache"
	goredis "github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := New(client, WithPrefix("test:"))
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})
	return store, mr
}

func TestStore_SetGet(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.Set("user", map[string]string{"name": "Ada"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	val, err := store.Get("user")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := val.(map[string]interface{})
	if !ok || m["name"] != "Ada" {
		t.Fatalf("got %#v", val)
	}
}

func TestStore_Miss(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.Get("missing")
	if err != cache.ErrKeyNotFound {
		t.Fatalf("want ErrKeyNotFound, got %v", err)
	}
}

func TestStore_TTL(t *testing.T) {
	store, mr := newTestStore(t)
	if err := store.Set("k", "v", time.Second); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Second)
	_, err := store.Get("k")
	if err != cache.ErrKeyNotFound {
		t.Fatalf("want miss after TTL, got %v", err)
	}
}

func TestStore_DeleteFlush(t *testing.T) {
	store, _ := newTestStore(t)
	_ = store.Set("a", 1, 0)
	_ = store.Set("b", 2, 0)
	if err := store.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("a"); err != cache.ErrKeyNotFound {
		t.Fatal(err)
	}
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("b"); err != cache.ErrKeyNotFound {
		t.Fatal(err)
	}
}

func TestStore_FlushRequiresPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()
	store := New(client)
	if err := store.Flush(); err == nil {
		t.Fatal("expected error without prefix")
	}
}
