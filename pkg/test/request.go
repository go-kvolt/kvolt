package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2"
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
		if r.t != nil {
			r.t.Errorf("Failed to marshal JSON body: %v", err)
		}
		return r
	}
	r.header.Set("Content-Type", "application/json")
	return r.WithBody(b)
}

// Do executes the request and returns a Response tester.
func (r *Request) Do() *Response {
	if r == nil || r.engine == nil {
		return nil
	}
	req := httptest.NewRequest(r.method, r.path, r.body)
	for k, vv := range r.header {
		req.Header[k] = append([]string(nil), vv...)
	}

	w := httptest.NewRecorder()
	r.engine.ServeHTTP(w, req)

	return NewResponse(r.t, w)
}
