package redisidem

import (
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/middleware"
	goredis "github.com/redis/go-redis/v9"
)

func TestStore_SetGet(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := New(rdb, WithPrefix("test:idem:"))
	rec := middleware.IdempotencyRecord{
		Status:  201,
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"ok":true}`),
	}
	if err := store.Set("k1", rec, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get("k1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Status != 201 || string(got.Body) != `{"ok":true}` {
		t.Fatalf("got=%+v", got)
	}
	if got.Headers.Get("Content-Type") != "application/json" {
		t.Fatalf("headers=%v", got.Headers)
	}

	if _, ok, err := store.Get("missing"); err != nil || ok {
		t.Fatalf("missing: ok=%v err=%v", ok, err)
	}
}

func TestStore_TTLExpiry(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := New(rdb)
	_ = store.Set("k", middleware.IdempotencyRecord{Status: 200, Body: []byte("x")}, time.Second)
	mr.FastForward(2 * time.Second)
	if _, ok, err := store.Get("k"); err != nil || ok {
		t.Fatalf("expected expired, ok=%v err=%v", ok, err)
	}
}
