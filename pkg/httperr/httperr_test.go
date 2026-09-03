package httperr

import (
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
