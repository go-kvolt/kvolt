package test

import (
	"net/http"
	"testing"

	"github.com/go-kvolt/kvolt"
)

// Tester is the main entry point for KVolt testing.
type Tester struct {
	t      *testing.T
	engine *kvolt.Engine
}

// New creates a new Tester instance.
func New(t *testing.T, e *kvolt.Engine) *Tester {
	return &Tester{
		t:      t,
		engine: e,
	}
}

// Request creates a new request builder.
func (t *Tester) Request(method, path string) *Request {
	return NewRequest(t.t, t.engine, method, path)
}

// GET creates a GET request.
func (t *Tester) GET(path string) *Request {
	return t.Request(http.MethodGet, path)
}

// POST creates a POST request.
func (t *Tester) POST(path string) *Request {
	return t.Request(http.MethodPost, path)
}

// PUT creates a PUT request.
func (t *Tester) PUT(path string) *Request {
	return t.Request(http.MethodPut, path)
}

// DELETE creates a DELETE request.
func (t *Tester) DELETE(path string) *Request {
	return t.Request(http.MethodDelete, path)
}

// PATCH creates a PATCH request.
func (t *Tester) PATCH(path string) *Request {
	return t.Request(http.MethodPatch, path)
}
