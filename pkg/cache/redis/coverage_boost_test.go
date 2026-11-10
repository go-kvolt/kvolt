package redis

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/pkg/cache"
	goredis "github.com/redis/go-redis/v9"
)

type failCmdHook struct{ name string }

func (h failCmdHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }
func (h failCmdHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		if strings.EqualFold(cmd.Name(), h.name) {
			return fmt.Errorf("forced %s failure", h.name)
		}
		return next(ctx, cmd)
	}
}
func (h failCmdHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}

func TestStore_Touch(t *testing.T) {
	store, mr := newTestStore(t)
	if err := store.Touch("missing", time.Minute); err != cache.ErrKeyNotFound {
		t.Fatalf("want miss, got %v", err)
	}
	if err := store.Set("k", "v", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.Touch("k", 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.Touch("k", 0); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Hour)
	// persisted key should still exist
	if _, err := store.Get("k"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("k"); err != nil {
		t.Fatal(err)
	}
	if err := store.Touch("k", 0); err != cache.ErrKeyNotFound {
		t.Fatalf("want miss after delete, got %v", err)
	}
}

func TestStore_GetUnmarshalError(t *testing.T) {
	store, mr := newTestStore(t)
	if err := mr.Set("test:bad", "{"); err != nil {
		t.Fatal(err)
	}
	_, err := store.Get("bad")
	if err == nil {
		t.Fatal("want unmarshal error")
	}
}

func TestStore_SetMarshalError(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.Set("bad", complex(1, 2), 0); err == nil {
		t.Fatal("want marshal error")
	}
}

func TestStore_RedisErrorsViaClose(t *testing.T) {
	store, mr := newTestStore(t)
	_ = store.Set("k", "v", time.Minute)

	mr.Close()

	if _, err := store.Get("k"); err == nil {
		t.Fatal("want get error after close")
	}
	if err := store.Touch("k", time.Minute); err == nil {
		t.Fatal("want expire error after close")
	}
	if err := store.Touch("k", 0); err == nil {
		t.Fatal("want persist error after close")
	}
	if err := store.Flush(); err == nil {
		t.Fatal("want flush/scan error after close")
	}
}

func TestStore_FlushDelError(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	client.AddHook(failCmdHook{name: "del"})

	store := New(client, WithPrefix("flush:"))
	if err := store.Set("a", 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Flush(); err == nil || !strings.Contains(err.Error(), "del") {
		t.Fatalf("want del failure, got %v", err)
	}
}
