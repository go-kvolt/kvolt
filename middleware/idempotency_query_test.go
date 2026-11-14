package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestIdempotency_QueryStringDistinctKeys(t *testing.T) {
	req1 := httptest.NewRequest("POST", "/checkout?method=card", nil)
	c1 := context.New(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest("POST", "/checkout?method=crypto", nil)
	c2 := context.New(httptest.NewRecorder(), req2)

	key1 := defaultIdempotencyKey(c1, "req-123", false)
	key2 := defaultIdempotencyKey(c2, "req-123", false)

	if key1 == key2 {
		t.Fatalf("expected different idempotency keys for different query params, got %s == %s", key1, key2)
	}
}
