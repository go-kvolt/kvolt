package logx

import (
	"io"
	"os"
	"sync"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	_ = w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestDefaultWritesToStdoutUnchanged(t *testing.T) {
	Set(nil)
	out := captureStdout(t, func() {
		Println("server", "started")
		Printf("\nstopping %d workers\n", 3)
	})
	if want := "server started\n\nstopping 3 workers\n"; out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

func TestSetRoutesTrimmedMessages(t *testing.T) {
	var mu sync.Mutex
	var got []string
	Set(func(msg string) {
		mu.Lock()
		got = append(got, msg)
		mu.Unlock()
	})
	defer Set(nil)

	out := captureStdout(t, func() {
		Println("server", "started")
		Printf("\nstopping %d workers\n", 3)
		Println()       // layout-only line: dropped
		Printf("\n \n") // whitespace only: dropped
	})
	if out != "" {
		t.Fatalf("stdout = %q, want nothing once a destination is set", out)
	}
	want := []string{"server started", "stopping 3 workers"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("messages = %q, want %q", got, want)
	}

	Set(nil)
	if out := captureStdout(t, func() { Println("back") }); out != "back\n" {
		t.Fatalf("after Set(nil) stdout = %q, want %q", out, "back\n")
	}
}

func TestConcurrentUse(t *testing.T) {
	var mu sync.Mutex
	n := 0
	Set(func(string) { mu.Lock(); n++; mu.Unlock() })
	defer Set(nil)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); Printf("msg %d", 1) }()
	}
	wg.Wait()
	if n != 50 {
		t.Fatalf("got %d messages, want 50", n)
	}
}
