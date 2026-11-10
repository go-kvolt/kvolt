package httperr

import "testing"

func TestError_NilAndEmptyMessage(t *testing.T) {
	var e *Error
	if e.Error() != "" {
		t.Fatal("nil")
	}
	e2 := New(400, "bad", "")
	if e2.Error() != "bad" {
		t.Fatal(e2.Error())
	}
}
