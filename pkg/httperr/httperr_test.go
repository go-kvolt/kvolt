package httperr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestNewAndHelpers(t *testing.T) {
	e := BadRequest("invalid_json", "bad body").WithDetails(map[string]string{"field": "email"})
	if e.Status != http.StatusBadRequest || e.Code != "invalid_json" {
		t.Fatalf("unexpected error: %+v", e)
	}
	if e.Details == nil {
		t.Fatal("expected details")
	}
	got, ok := As(e)
	if !ok || got.Code != "invalid_json" {
		t.Fatalf("As failed: %v %v", got, ok)
	}
}

func TestAllStatusHelpers(t *testing.T) {
	cases := []struct {
		e      *Error
		status int
		code   string
	}{
		{Unauthorized("", "nope"), 401, "unauthorized"},
		{Forbidden("", "no"), 403, "forbidden"},
		{NotFound("", "gone"), 404, "not_found"},
		{TooManyRequests(""), 429, "rate_limited"},
		{Conflict("", "dup"), 409, "conflict"},
		{UnprocessableEntity("", "bad"), 422, "unprocessable_entity"},
		{PayloadTooLarge(""), 413, "payload_too_large"},
		{ServiceUnavailable(""), 503, "unavailable"},
		{Internal(""), 500, "internal_error"},
		{New(0, "", "x"), 500, "error"},
	}
	for _, tc := range cases {
		if tc.e.Status != tc.status || tc.e.Code != tc.code {
			t.Fatalf("got status=%d code=%q want %d %q msg=%q", tc.e.Status, tc.e.Code, tc.status, tc.code, tc.e.Message)
		}
		if tc.e.Error() == "" {
			t.Fatal("empty Error()")
		}
	}
}

func TestAsWrappedAndNil(t *testing.T) {
	if _, ok := As(nil); ok {
		t.Fatal("As(nil)")
	}
	if _, ok := As(errors.New("plain")); ok {
		t.Fatal("As(plain)")
	}
	inner := NotFound("missing", "no")
	wrapped := fmt.Errorf("wrap: %w", inner)
	got, ok := As(wrapped)
	if !ok || got.Code != "missing" {
		t.Fatalf("As wrapped: %v %v", got, ok)
	}
	var nilE *Error
	if nilE.Error() != "" {
		t.Fatal("nil Error()")
	}
	if nilE.WithDetails("x") != nil {
		t.Fatal("nil WithDetails")
	}
}
