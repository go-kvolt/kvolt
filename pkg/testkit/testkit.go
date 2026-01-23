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
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()

	ts.Handler.ServeHTTP(w, req)

	return &Response{Recorder: w}
}

// Post performs a POST request with JSON body.
func (ts *TestServer) Post(t *testing.T, path string, body interface{}) *Response {
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	ts.Handler.ServeHTTP(w, req)

	return &Response{Recorder: w}
}

// Response wraps httptest.ResponseRecorder for easier assertions.
type Response struct {
	Recorder *httptest.ResponseRecorder
}

// AssertStatus asserts the HTTP status code.
func (r *Response) AssertStatus(t *testing.T, code int) {
	if r.Recorder.Code != code {
		t.Errorf("Expected status %d, got %d", code, r.Recorder.Code)
	}
}

// AssertBody asserts the body contains string.
func (r *Response) AssertBody(t *testing.T, contains string) {
	if !bytes.Contains(r.Recorder.Body.Bytes(), []byte(contains)) {
		t.Errorf("Expected body to contain '%s'", contains)
	}
}
