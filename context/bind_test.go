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

func TestBindQuery_RepeatedParamsAndSlices(t *testing.T) {
	r := httptest.NewRequest("GET", "/?tags=go&tags=kvolt&ids=10&ids=20&rates=1.5&rates=2.5&flags=true&flags=false", nil)
	c := New(httptest.NewRecorder(), r)
	var q struct {
		Tags  []string  `query:"tags"`
		IDs   []int     `query:"ids"`
		Rates []float64 `query:"rates"`
		Flags []bool    `query:"flags"`
	}
	if err := c.BindQuery(&q); err != nil {
		t.Fatalf("BindQuery: %v", err)
	}
	if len(q.Tags) != 2 || q.Tags[0] != "go" || q.Tags[1] != "kvolt" {
		t.Errorf("Tags want [go kvolt], got %v", q.Tags)
	}
	if len(q.IDs) != 2 || q.IDs[0] != 10 || q.IDs[1] != 20 {
		t.Errorf("IDs want [10 20], got %v", q.IDs)
	}
	if len(q.Rates) != 2 || q.Rates[0] != 1.5 || q.Rates[1] != 2.5 {
		t.Errorf("Rates want [1.5 2.5], got %v", q.Rates)
	}
	if len(q.Flags) != 2 || !q.Flags[0] || q.Flags[1] {
		t.Errorf("Flags want [true false], got %v", q.Flags)
	}
}
