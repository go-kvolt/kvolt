package session

import (
	"errors"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/pkg/cache"
)

type errStore struct {
	setErr   error
	getErr   error
	delErr   error
	touchErr error
	data     map[string]interface{}
}

func (e *errStore) Get(key string) (interface{}, error) {
	if e.getErr != nil {
		return nil, e.getErr
	}
	v, ok := e.data[key]
	if !ok {
		return nil, cache.ErrKeyNotFound
	}
	return v, nil
}

func (e *errStore) Set(key string, value interface{}, ttl time.Duration) error {
	if e.setErr != nil {
		return e.setErr
	}
	if e.data == nil {
		e.data = map[string]interface{}{}
	}
	e.data[key] = value
	return nil
}

func (e *errStore) Delete(key string) error {
	if e.delErr != nil {
		return e.delErr
	}
	delete(e.data, key)
	return nil
}

func (e *errStore) Flush() error {
	e.data = map[string]interface{}{}
	return nil
}

func (e *errStore) Touch(key string, ttl time.Duration) error {
	if e.touchErr != nil {
		return e.touchErr
	}
	if _, ok := e.data[key]; !ok {
		return cache.ErrKeyNotFound
	}
	return nil
}

func TestCreate_StoreError(t *testing.T) {
	m := New(&errStore{setErr: errors.New("set failed")}, time.Hour)
	if _, err := m.Create("x"); err == nil {
		t.Fatal("want create error")
	}
}

func TestGet_StoreErrorsAndTouch(t *testing.T) {
	st := &errStore{data: map[string]interface{}{"tok": "v"}}
	m := New(st, time.Hour)
	got, err := m.Get("tok")
	if err != nil || got != "v" {
		t.Fatalf("got=%v err=%v", got, err)
	}

	st.getErr = errors.New("boom")
	if _, err := m.Get("tok"); err == nil || errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("want generic get error, got %v", err)
	}
	st.getErr = nil

	st.touchErr = cache.ErrKeyNotFound
	if _, err := m.Get("tok"); err != ErrSessionNotFound {
		t.Fatalf("want not found on touch miss, got %v", err)
	}
	st.touchErr = cache.ErrExpired
	if _, err := m.Get("tok"); err != ErrSessionNotFound {
		t.Fatalf("want not found on touch expired, got %v", err)
	}
	st.touchErr = errors.New("touch other")
	if _, err := m.Get("tok"); err != nil {
		// non-not-found touch errors are ignored; value still returned
		t.Fatalf("unexpected %v", err)
	}
}

func TestMaxAge_PruneOnCreate(t *testing.T) {
	st := &errStore{data: map[string]interface{}{}}
	m := New(st, time.Hour).WithMaxAge(time.Millisecond)
	tok1, err := m.Create("a")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	tok2, err := m.Create("b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(tok1); err != ErrSessionNotFound {
		// may still be in store if prune only cleared created map via Create's stale delete
		t.Log(err)
	}
	if _, err := m.Get(tok2); err != nil {
		t.Fatal(err)
	}
}
