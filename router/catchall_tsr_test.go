package router

import (
	"testing"
)

func TestCatchAll_ExactPrefixAndTSR(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/files/*filepath", "fileHandler")

	// 1. Exact prefix with trailing slash
	h, p, ok := r.Find("GET", "/files/")
	if !ok || h != "fileHandler" {
		t.Fatalf("expected /files/ to match catch-all route, got ok=%v, h=%v", ok, h)
	}
	if p.Get("filepath") != "" {
		t.Fatalf("expected empty filepath param for /files/, got %q", p.Get("filepath"))
	}

	// 2. Subpath matching
	h, p, ok = r.Find("GET", "/files/doc.txt")
	if !ok || h != "fileHandler" {
		t.Fatalf("expected /files/doc.txt to match, got ok=%v, h=%v", ok, h)
	}
	if p.Get("filepath") != "doc.txt" {
		t.Fatalf("expected filepath=doc.txt, got %q", p.Get("filepath"))
	}

	// 3. Missing trailing slash -> TSR = true
	res := r.FindEx("GET", "/files", nil)
	if !res.TSR {
		t.Fatalf("expected TSR=true for /files when /files/*filepath registered, got %+v", res)
	}
}
