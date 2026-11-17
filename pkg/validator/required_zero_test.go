package validator

import (
	"testing"
	"time"
)

// required rejects the zero value of numbers, booleans and structs, not only empty
// strings: an omitted JSON field decodes to the zero value and must not pass.
func TestValidate_RequiredRejectsZeroValues(t *testing.T) {
	type order struct {
		Quantity int       `validate:"required"`
		Price    float64   `validate:"required"`
		Accepted bool      `validate:"required"`
		PlacedAt time.Time `validate:"required"`
	}
	valid := order{Quantity: 1, Price: 9.5, Accepted: true, PlacedAt: time.Unix(1700000000, 0)}
	if err := Validate(valid); err != nil {
		t.Fatalf("valid order rejected: %v", err)
	}

	cases := map[string]func(o *order){
		"Quantity": func(o *order) { o.Quantity = 0 },
		"Price":    func(o *order) { o.Price = 0 },
		"Accepted": func(o *order) { o.Accepted = false },
		"PlacedAt": func(o *order) { o.PlacedAt = time.Time{} },
	}
	for field, zero := range cases {
		o := valid
		zero(&o)
		err := Validate(o)
		if err == nil {
			t.Errorf("%s: zero value passed required", field)
			continue
		}
		if want := "field " + field + " is required"; err.Error() != want {
			t.Errorf("%s: error = %q, want %q", field, err.Error(), want)
		}
	}
}

// A pointer field distinguishes "not sent" (nil) from "sent as 0 / false".
func TestValidate_RequiredPointerAllowsExplicitZero(t *testing.T) {
	type settings struct {
		Retries *int  `validate:"required"`
		Enabled *bool `validate:"required"`
	}
	zero, off := 0, false
	if err := Validate(settings{Retries: &zero, Enabled: &off}); err != nil {
		t.Fatalf("explicit 0 / false through a pointer rejected: %v", err)
	}
	if err := Validate(settings{Enabled: &off}); err == nil {
		t.Fatalf("nil pointer passed required")
	}
}

// Without required, zero values are still fine.
func TestValidate_ZeroValuesWithoutRequired(t *testing.T) {
	type page struct {
		Offset int `validate:"min=0"`
		Debug  bool
	}
	if err := Validate(page{}); err != nil {
		t.Fatalf("zero values without required rejected: %v", err)
	}
}
