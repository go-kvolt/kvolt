package test

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Response wraps httptest.ResponseRecorder with assertion methods.
type Response struct {
	t        *testing.T
	Recorder *httptest.ResponseRecorder
}

// NewResponse creates a new Response assertion wrapper.
func NewResponse(t *testing.T, w *httptest.ResponseRecorder) *Response {
	return &Response{
		t:        t,
		Recorder: w,
	}
}

// ExpectStatus asserts that the response status code matches expectation.
func (r *Response) ExpectStatus(code int) *Response {
	if r.Recorder.Code != code {
		r.t.Errorf("Expected status code %d, got %d", code, r.Recorder.Code)
	}
	return r
}

// ExpectBody asserts that the response body equals the expected string.
func (r *Response) ExpectBody(expected string) *Response {
	if r.Recorder.Body.String() != expected {
		r.t.Errorf("Expected body %q, got %q", expected, r.Recorder.Body.String())
	}
	return r
}

// ExpectBodyContains asserts that the response body contains the substring.
func (r *Response) ExpectBodyContains(substring string) *Response {
	if !strings.Contains(r.Recorder.Body.String(), substring) {
		r.t.Errorf("Expected body to contain %q, got %q", substring, r.Recorder.Body.String())
	}
	return r
}

// ExpectHeader asserts that the response has a header with the given value.
func (r *Response) ExpectHeader(key, value string) *Response {
	got := r.Recorder.Header().Get(key)
	if got != value {
		r.t.Errorf("Expected header %s=%q, got %q", key, value, got)
	}
	return r
}

// ExpectJSON asserts that the response body matches the given JSON object.
// It works by unmarshaling the actual body into the type of the expected object,
// and then comparing them using DeepEqual.
func (r *Response) ExpectJSON(expected interface{}) *Response {
	actualJSON := r.Recorder.Body.Bytes()

	// Create a new instance of the expected type to unmarshal into
	actual := reflect.New(reflect.TypeOf(expected)).Interface()

	err := json.Unmarshal(actualJSON, actual)
	if err != nil {
		r.t.Errorf("Failed to unmarshal response JSON: %v. Body: %s", err, string(actualJSON))
		return r
	}

	// indirect because actual is a pointer to the type of expected
	actualValue := reflect.ValueOf(actual).Elem().Interface()

	if !reflect.DeepEqual(actualValue, expected) {
		r.t.Errorf("Expected JSON value %v, got %v", expected, actualValue)
	}
	return r
}
