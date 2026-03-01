package testkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTestServer_Get(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	ts := New(h)
	resp := ts.Get(t, "/")
	resp.AssertStatus(t, 200)
	resp.AssertBody(t, "ok")
}

func TestTestServer_Post(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		w.Write([]byte(`{"created":true}`))
	})
	ts := New(h)
	resp := ts.Post(t, "/", map[string]bool{"x": true})
	resp.AssertStatus(t, 201)
	resp.AssertBody(t, "created")
}

func TestResponse_AssertStatus(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(404)
	resp := &Response{Recorder: w}
	// Should not call t.Errorf when code matches
	resp.AssertStatus(t, 404)
}
