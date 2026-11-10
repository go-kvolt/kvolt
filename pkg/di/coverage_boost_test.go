package di

import (
	"errors"
	"testing"
)

func TestMustProvideFunc(t *testing.T) {
	c := NewContainer()
	c.MustProvideFunc(func() int { return 7 })
	var n int
	if !c.Invoke(&n) || n != 7 {
		t.Fatal(n)
	}
	if err := c.ProvideFunc(func() (int, string) { return 1, "x" }); err == nil {
		t.Fatal("want second return must be error")
	}
	if err := c.ProvideFunc(func() {}); err == nil {
		t.Fatal("want numOut error")
	}
	if err := c.ProvideFunc(func(d *string) int { return 1 }); err == nil {
		t.Fatal("want missing dep")
	}
	if err := c.ProvideFunc(func() (int, error) { return 0, errors.New("boom") }); err == nil {
		t.Fatal("want ctor error")
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("want panic")
		}
	}()
	c.MustProvideFunc(42) // invalid constructor
}
