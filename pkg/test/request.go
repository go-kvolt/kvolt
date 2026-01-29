package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt"
)

// Request is a builder for HTTP requests.
type Request struct {
	t      *testing.T
	engine *kvolt.Engine
	method string
	path   string
	header http.Header
	body   io.Reader
}

// NewRequest creates a new Request builder.
func NewRequest(t *testing.T, e *kvolt.Engine, method, path string) *Request {
	return &Request{
		t:      t,
		engine: e,
		method: method,
		path:   path,
		header: make(http.Header),
	}
}

// WithHeader adds a header to the request.
func (r *Request) WithHeader(key, value string) *Request {
	r.header.Add(key, value)
	return r
}

// WithBody sets the raw request body.
func (r *Request) WithBody(body []byte) *Request {
	r.body = bytes.NewReader(body)
	return r
}

// WithJSON sets the request body as JSON and adds Content-Type header.
func (r *Request) WithJSON(obj interface{}) *Request {
	b, err := json.Marshal(obj)
	if err != nil {
		r.t.Fatalf("Failed to marshal JSON body: %v", err)
	}
	r.header.Set("Content-Type", "application/json")
	return r.WithBody(b)
}

// Do executes the request and returns a Response tester.
func (r *Request) Do() *Response {
	req := httptest.NewRequest(r.method, r.path, r.body)
	req.Header = r.header

	w := httptest.NewRecorder()
	r.engine.ServeHTTP(w, req)

	return NewResponse(r.t, w)
}
