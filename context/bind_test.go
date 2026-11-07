package context

import (
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestBindQuery_RejectsUint8Overflow(t *testing.T) {
	r := httptest.NewRequest("GET", "/?n=300", nil)
	c := New(httptest.NewRecorder(), r)
	var q struct {
		N uint8 `query:"n"`
	}
	err := c.BindQuery(&q)
	if err == nil {
		t.Fatalf("want overflow error, got value %d", q.N)
	}
	if _, ok := err.(*strconv.NumError); !ok {
		// ParseUint wraps as *strconv.NumError
		if err.Error() == "" {
			t.Fatalf("unexpected error type: %T %v", err, err)
		}
	}
	if q.N == 44 {
		t.Fatal("silently truncated 300→44 (SetUint without bit-size check)")
	}
}

func TestBindQuery_AcceptsUint8InRange(t *testing.T) {
	r := httptest.NewRequest("GET", "/?n=200", nil)
	c := New(httptest.NewRecorder(), r)
	var q struct {
		N uint8 `query:"n"`
	}
	if err := c.BindQuery(&q); err != nil {
		t.Fatal(err)
	}
	if q.N != 200 {
		t.Fatalf("want 200 got %d", q.N)
	}
}

func TestBindQuery_RejectsInt8Overflow(t *testing.T) {
	r := httptest.NewRequest("GET", "/?n=200", nil)
	c := New(httptest.NewRecorder(), r)
	var q struct {
		N int8 `query:"n"`
	}
	if err := c.BindQuery(&q); err == nil {
		t.Fatalf("want overflow error, got %d", q.N)
	}
}

func TestBindQuery_RejectsInvalidJSONField(t *testing.T) {
	r := httptest.NewRequest("GET", "/?meta=not-json", nil)
	c := New(httptest.NewRecorder(), r)
	var q struct {
		Meta map[string]string `query:"meta"`
	}
	if err := c.BindQuery(&q); err == nil {
		t.Fatal("want JSON unmarshal error")
	}
}
