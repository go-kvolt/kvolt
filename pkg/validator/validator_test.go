package validator

import (
	"testing"
)

func TestValidate_Required(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}
	err := Validate(&S{})
	if err == nil {
		t.Error("required empty: want error")
	}
	err = Validate(&S{Name: "x"})
	if err != nil {
		t.Errorf("required set: %v", err)
	}
}

func TestValidate_Email(t *testing.T) {
	type S struct {
		Email string `validate:"email"`
	}
	err := Validate(&S{Email: "invalid"})
	if err == nil {
		t.Error("invalid email: want error")
	}
	err = Validate(&S{Email: "user@example.com"})
	if err != nil {
		t.Errorf("valid email: %v", err)
	}
	err = Validate(&S{Email: "Admin@Company.ONLINE"})
	if err != nil {
		t.Errorf("uppercase and long TLD email should be valid: %v", err)
	}
	err = Validate(&S{Email: "erp.user@enterprise.internal"})
	if err != nil {
		t.Errorf("enterprise internal email should be valid: %v", err)
	}
}

func TestValidate_Min(t *testing.T) {
	type S struct {
		Password string `validate:"min=6"`
	}
	err := Validate(&S{Password: "short"})
	if err == nil {
		t.Error("min=6 with 5 chars: want error")
	}
	err = Validate(&S{Password: "longenough"})
	if err != nil {
		t.Errorf("min=6 ok: %v", err)
	}
}

func TestValidate_Max(t *testing.T) {
	type S struct {
		Code  string   `validate:"max=5"`
		Items []string `validate:"max=2"`
	}
	err := Validate(&S{Code: "toolong"})
	if err == nil {
		t.Error("max=5 with 7 chars: want error")
	}
	err = Validate(&S{Code: "ok", Items: []string{"1", "2", "3"}})
	if err == nil {
		t.Error("max=2 with 3 items: want error")
	}
	err = Validate(&S{Code: "ok", Items: []string{"1", "2"}})
	if err != nil {
		t.Errorf("max ok: %v", err)
	}
}

func TestValidate_NoTags(t *testing.T) {
	type S struct {
		A string
	}
	err := Validate(&S{A: ""})
	if err != nil {
		t.Errorf("no validate tags: %v", err)
	}
}

func TestValidate_IntMinMax(t *testing.T) {
	type S struct {
		Age int  `validate:"min=18,max=65"`
		Qty uint `validate:"min=1,max=100"`
	}
	if err := Validate(&S{Age: 16, Qty: 5}); err == nil {
		t.Fatal("age 16 < 18: want error")
	}
	if err := Validate(&S{Age: 70, Qty: 5}); err == nil {
		t.Fatal("age 70 > 65: want error")
	}
	if err := Validate(&S{Age: 25, Qty: 0}); err == nil {
		t.Fatal("qty 0 < 1: want error")
	}
	if err := Validate(&S{Age: 25, Qty: 150}); err == nil {
		t.Fatal("qty 150 > 100: want error")
	}
	if err := Validate(&S{Age: 25, Qty: 50}); err != nil {
		t.Fatalf("valid int range: %v", err)
	}
}

func TestValidate_NilAndNilPointer(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}
	var nilPtr *S
	if err := Validate(nilPtr); err != nil {
		t.Fatalf("expected nil error for nil pointer, got: %v", err)
	}
	if err := Validate(nil); err != nil {
		t.Fatalf("expected nil error for nil interface, got: %v", err)
	}
}

type OrderLine struct {
	SKU string `validate:"required"`
	Qty int    `validate:"min=1"`
}

type CustomerInfo struct {
	Email string `validate:"required,email"`
}

type OrderPayload struct {
	Customer CustomerInfo
	Lines    []OrderLine
}

func TestValidate_NestedStructAndSlice(t *testing.T) {
	badOrder := OrderPayload{
		Customer: CustomerInfo{Email: "invalid-email"},
		Lines: []OrderLine{
			{SKU: "PROD-1", Qty: 2},
		},
	}
	if err := Validate(&badOrder); err == nil {
		t.Fatal("expected error on invalid customer email")
	}

	badLineOrder := OrderPayload{
		Customer: CustomerInfo{Email: "test@example.com"},
		Lines: []OrderLine{
			{SKU: "", Qty: 2},
		},
	}
	if err := Validate(&badLineOrder); err == nil {
		t.Fatal("expected error on empty line SKU")
	}

	validOrder := OrderPayload{
		Customer: CustomerInfo{Email: "test@example.com"},
		Lines: []OrderLine{
			{SKU: "PROD-1", Qty: 2},
		},
	}
	if err := Validate(&validOrder); err != nil {
		t.Fatalf("unexpected error on valid order: %v", err)
	}
}

func TestValidate_FloatMinMax(t *testing.T) {
	type Product struct {
		Price    float64 `validate:"min=0.01,max=9999.99"`
		Discount float32 `validate:"min=0,max=100"`
	}

	if err := Validate(&Product{Price: 0.005, Discount: 10}); err == nil {
		t.Fatal("expected error on price < 0.01")
	}
	if err := Validate(&Product{Price: 10000.0, Discount: 10}); err == nil {
		t.Fatal("expected error on price > 9999.99")
	}
	if err := Validate(&Product{Price: 50.0, Discount: -1}); err == nil {
		t.Fatal("expected error on discount < 0")
	}
	if err := Validate(&Product{Price: 50.0, Discount: 105}); err == nil {
		t.Fatal("expected error on discount > 100")
	}
	if err := Validate(&Product{Price: 19.99, Discount: 15}); err != nil {
		t.Fatalf("unexpected error on valid product: %v", err)
	}
}

func TestValidate_MapRecursionAndMinMax(t *testing.T) {
	type Item struct {
		Name string `validate:"required"`
	}
	type Catalog struct {
		Items map[string]Item   `validate:"min=1,max=3"`
		Tags  map[string]string `validate:"max=2"`
	}

	// Map min validation
	if err := Validate(&Catalog{Items: map[string]Item{}}); err == nil {
		t.Fatal("expected error on map min=1 with 0 items")
	}

	// Map max validation
	if err := Validate(&Catalog{
		Items: map[string]Item{
			"a": {Name: "A"},
			"b": {Name: "B"},
			"c": {Name: "C"},
			"d": {Name: "D"},
		},
	}); err == nil {
		t.Fatal("expected error on map max=3 with 4 items")
	}

	// Recursive validation inside map value
	if err := Validate(&Catalog{
		Items: map[string]Item{
			"valid": {Name: ""}, // empty name
		},
	}); err == nil {
		t.Fatal("expected error on invalid item inside map")
	}

	// Valid catalog
	valid := &Catalog{
		Items: map[string]Item{
			"item1": {Name: "Book"},
		},
		Tags: map[string]string{
			"category": "education",
		},
	}
	if err := Validate(valid); err != nil {
		t.Fatalf("unexpected error on valid catalog: %v", err)
	}
}
