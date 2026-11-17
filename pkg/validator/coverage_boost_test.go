package validator

import (
	"reflect"
	"testing"
)

func TestIsEmptyKinds(t *testing.T) {
	if !isEmpty(reflect.ValueOf("")) {
		t.Fatal("empty string")
	}
	if isEmpty(reflect.ValueOf("x")) {
		t.Fatal("nonempty string")
	}
	if !isEmpty(reflect.ValueOf([]int{})) {
		t.Fatal("empty slice")
	}
	if !isEmpty(reflect.ValueOf(map[string]int{})) {
		t.Fatal("empty map")
	}
	var p *int
	if !isEmpty(reflect.ValueOf(p)) {
		t.Fatal("nil ptr")
	}
	if !isEmpty(reflect.ValueOf(0)) || !isEmpty(reflect.ValueOf(false)) || !isEmpty(reflect.ValueOf(0.0)) {
		t.Fatal("zero number / false must be empty")
	}
	if isEmpty(reflect.ValueOf(7)) || isEmpty(reflect.ValueOf(true)) {
		t.Fatal("non-zero number / true must not be empty")
	}
	zero, no := 0, false
	if isEmpty(reflect.ValueOf(&zero)) || isEmpty(reflect.ValueOf(&no)) {
		t.Fatal("a non-nil pointer to 0 / false is present")
	}
}

func TestValidate_NonStruct(t *testing.T) {
	if err := Validate(42); err != nil {
		t.Fatalf("non-struct should return nil, got %v", err)
	}
	if err := Validate("x"); err != nil {
		t.Fatalf("string should return nil, got %v", err)
	}
}
