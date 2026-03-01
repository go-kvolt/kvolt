package di

import (
	"testing"
)

type FakeService struct {
	Value string
}

func TestContainer_ProvideAndInvoke(t *testing.T) {
	c := NewContainer()
	svc := &FakeService{Value: "hello"}
	c.Provide(svc)

	var out *FakeService
	ok := c.Invoke(&out)
	if !ok {
		t.Fatal("Invoke: want true")
	}
	if out != svc || out.Value != "hello" {
		t.Errorf("Invoke: want same instance with Value hello, got %v", out)
	}
}

func TestContainer_InvokeNilTarget(t *testing.T) {
	c := NewContainer()
	var out *FakeService
	ok := c.Invoke(out) // pass nil pointer (out is nil)
	if ok {
		t.Error("Invoke(nil): want false")
	}
}

func TestContainer_InvokeMissingService(t *testing.T) {
	c := NewContainer()
	var out *FakeService
	ok := c.Invoke(&out)
	if ok {
		t.Error("Invoke without Provide: want false")
	}
	if out != nil {
		t.Error("Invoke missing: out should remain nil")
	}
}
