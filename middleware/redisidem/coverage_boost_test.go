package redisidem

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/middleware"
	goredis "github.com/redis/go-redis/v9"
)

func TestNew_PanicNilClient(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want panic")
		}
	}()
	_ = New(nil)
}

func TestStore_WithMaxBodyBytesAndErrors(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := New(rdb, WithPrefix("p:"), WithMaxBodyBytes(8), WithMaxBodyBytes(0) /* ignored */)
	err = store.Set("big", middleware.IdempotencyRecord{Status: 200, Body: []byte("0123456789")}, time.Minute)
	if err == nil || !strings.Contains(err.Error(), "exceeds max") {
		t.Fatalf("want max body error, got %v", err)
	}

	// ttl <= 0 path
	if err := store.Set("k", middleware.IdempotencyRecord{
		Status:  200,
		Headers: http.Header{"X": []string{"1"}},
		Body:    []byte("ok"),
	}, 0); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get("k")
	if err != nil || !ok || string(got.Body) != "ok" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}

	// corrupt JSON in redis
	if err := rdb.Set(t.Context(), "p:bad", "{not-json", 0).Err(); err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Get("bad")
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("want unmarshal error, got %v", err)
	}

	// redis error (closed server)
	mr.Close()
	_, _, err = store.Get("k")
	if err == nil {
		t.Fatal("want redis get error")
	}
	if err := store.Set("z", middleware.IdempotencyRecord{Status: 200, Body: []byte("x")}, time.Second); err == nil {
		t.Fatal("want redis set error")
	}
}

func TestStore_MarshalError(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := New(rdb)
	prev := recordMarshal
	recordMarshal = func(v interface{}) ([]byte, error) { return nil, errors.New("marshal boom") }
	defer func() { recordMarshal = prev }()

	err = store.Set("k", middleware.IdempotencyRecord{Status: 200, Body: []byte("x")}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("want marshal error, got %v", err)
	}
}
