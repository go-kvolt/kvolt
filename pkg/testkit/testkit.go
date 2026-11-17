package testkit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServer wraps the KVolt engine for testing.
type TestServer struct {
	Handler http.Handler
}

// New creates a new TestServer.
func New(h http.Handler) *TestServer {
	return &TestServer{Handler: h}
}

// Get performs a GET request.
func (ts *TestServer) Get(t *testing.T, path string) *Response {
	if ts == nil || ts.Handler == nil {
		return &Response{Recorder: httptest.NewRecorder()}
	}
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()

	ts.Handler.ServeHTTP(w, req)

	return &Response{Recorder: w}
}

// Post performs a POST request with JSON body.
func (ts *TestServer) Post(t *testing.T, path string, body interface{}) *Response {
	if ts == nil || ts.Handler == nil {
		return &Response{Recorder: httptest.NewRecorder()}
	}
	jsonBody, err := json.Marshal(body)
	if err != nil && t != nil {
		t.Helper()
		t.Fatalf("testkit: cannot encode request body: %v", err)
	}
	req := httptest.NewRequest("POST", path, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	ts.Handler.ServeHTTP(w, req)

	return &Response{Recorder: w}
}

// Put performs a PUT request with JSON body.
func (ts *TestServer) Put(t *testing.T, path string, body interface{}) *Response {
	if ts == nil || ts.Handler == nil {
		return &Response{Recorder: httptest.NewRecorder()}
	}
	jsonBody, err := json.Marshal(body)
	if err != nil && t != nil {
		t.Helper()
		t.Fatalf("testkit: cannot encode request body: %v", err)
	}
	req := httptest.NewRequest("PUT", path, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	ts.Handler.ServeHTTP(w, req)

	return &Response{Recorder: w}
}

// Delete performs a DELETE request.
func (ts *TestServer) Delete(t *testing.T, path string) *Response {
	if ts == nil || ts.Handler == nil {
		return &Response{Recorder: httptest.NewRecorder()}
	}
	req := httptest.NewRequest("DELETE", path, nil)
	w := httptest.NewRecorder()

	ts.Handler.ServeHTTP(w, req)

	return &Response{Recorder: w}
}

// Patch performs a PATCH request with JSON body.
func (ts *TestServer) Patch(t *testing.T, path string, body interface{}) *Response {
	if ts == nil || ts.Handler == nil {
		return &Response{Recorder: httptest.NewRecorder()}
	}
	jsonBody, err := json.Marshal(body)
	if err != nil && t != nil {
		t.Helper()
		t.Fatalf("testkit: cannot encode request body: %v", err)
	}
	req := httptest.NewRequest("PATCH", path, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	ts.Handler.ServeHTTP(w, req)

	return &Response{Recorder: w}
}

// Response wraps httptest.ResponseRecorder for easier assertions.
type Response struct {
	Recorder *httptest.ResponseRecorder
}

// Unmarshal decodes the response body JSON into v.
func (r *Response) Unmarshal(v interface{}) error {
	if r == nil || r.Recorder == nil {
		return nil
	}
	return json.Unmarshal(r.Recorder.Body.Bytes(), v)
}

// AssertStatus asserts the HTTP status code.
func (r *Response) AssertStatus(t *testing.T, code int) {
	if r == nil || r.Recorder == nil {
		return
	}
	if t != nil {
		t.Helper()
	}
	if t != nil && r.Recorder.Code != code {
		t.Errorf("Expected status %d, got %d", code, r.Recorder.Code)
	}
}

// AssertBody asserts the body contains string.
func (r *Response) AssertBody(t *testing.T, contains string) {
	if r == nil || r.Recorder == nil {
		return
	}
	if t != nil {
		t.Helper()
	}
	if t != nil && !bytes.Contains(r.Recorder.Body.Bytes(), []byte(contains)) {
		t.Errorf("Expected body to contain '%s'", contains)
	}
}
