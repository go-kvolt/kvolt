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

func TestResponse_NilT(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(500)
	resp := &Response{Recorder: w}
	// Calling with nil t should not panic even on mismatch
	resp.AssertStatus(nil, 200)
	resp.AssertBody(nil, "something_else")
}

func TestTestServer_PutDeletePatchAndUnmarshal(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			w.WriteHeader(200)
			w.Write([]byte(`{"action":"updated"}`))
		case "DELETE":
			w.WriteHeader(204)
		case "PATCH":
			w.WriteHeader(200)
			w.Write([]byte(`{"action":"patched"}`))
		}
	})
	ts := New(h)

	// PUT
	respPut := ts.Put(t, "/items/1", map[string]string{"name": "new"})
	respPut.AssertStatus(t, 200)
	var putBody struct {
		Action string `json:"action"`
	}
	if err := respPut.Unmarshal(&putBody); err != nil {
		t.Fatalf("Unmarshal PUT error: %v", err)
	}
	if putBody.Action != "updated" {
		t.Fatalf("expected updated, got %s", putBody.Action)
	}

	// DELETE
	respDel := ts.Delete(t, "/items/1")
	respDel.AssertStatus(t, 204)

	// PATCH
	respPatch := ts.Patch(t, "/items/1", map[string]string{"name": "patch"})
	respPatch.AssertStatus(t, 200)
	var patchBody struct {
		Action string `json:"action"`
	}
	if err := respPatch.Unmarshal(&patchBody); err != nil {
		t.Fatalf("Unmarshal PATCH error: %v", err)
	}
	if patchBody.Action != "patched" {
		t.Fatalf("expected patched, got %s", patchBody.Action)
	}

	// Nil server and nil response unmarshal guards
	var tsNil *TestServer
	_ = tsNil.Put(t, "/", nil)
	_ = tsNil.Delete(t, "/")
	_ = tsNil.Patch(t, "/", nil)
	var respNil *Response
	if err := respNil.Unmarshal(&putBody); err != nil {
		t.Fatal("expected nil error on nil response unmarshal")
	}
}
