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
