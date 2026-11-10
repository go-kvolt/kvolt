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
	if isEmpty(reflect.ValueOf(0)) {
		t.Fatal("int zero should not be empty via default")
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
