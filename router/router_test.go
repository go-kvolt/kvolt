package router

import (
	"fmt"
	"testing"
)

func TestRouter_Basic(t *testing.T) {
	r := New()

	// Add root
	fmt.Println("TEST: Adding /")
	r.AddRoute("GET", "/", func(c any) error { return nil })

	// Find root
	h, _, found := r.Find("GET", "/")
	if !found || h == nil {
		t.Error("Did not find root /")
	} else {
		fmt.Println("TEST: Found root /")
	}

	// Add /ping
	fmt.Println("TEST: Adding /ping")
	r.AddRoute("GET", "/ping", func(c any) error { return nil })

	h, _, found = r.Find("GET", "/ping")
	if !found || h == nil {
		t.Error("Did not find /ping")
	} else {
		fmt.Println("TEST: Found /ping")
	}

	// Add /users/1
	r.AddRoute("GET", "/users/:id", func(c any) error { return nil })
	// Not testing param logic yet, just static matching of /ping vs /
}
