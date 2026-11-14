package context

import (
	"net/http/httptest"
	"testing"
)

func TestJSON_EncodingErrorDoesNotCommitHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest("GET", "/", nil))

	// Channel cannot be JSON-encoded
	err := c.JSON(200, map[string]interface{}{
		"bad": make(chan int),
	})
	if err == nil {
		t.Fatal("expected JSON encoding error for channel type")
	}

	if c.HeaderWritten() {
		t.Fatal("expected c.HeaderWritten() to remain false after encoding failure")
	}

	if rec.Code != 200 && rec.Code != 0 {
		// recorder default before WriteHeader is 200, but rec.Flushed should be false
	}
	if rec.Body.Len() > 0 {
		t.Fatalf("expected empty recorder body on encoding failure, got %q", rec.Body.String())
	}
}
