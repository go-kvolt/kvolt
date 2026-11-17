package kvolt

import (
	"strings"
	"sync"
	"testing"

	"github.com/go-kvolt/kvolt/v2/pkg/queue"
)

// SetLogFunc is process-wide: it also covers packages outside the core, such as the queue.
func TestSetLogFunc_CapturesFrameworkStatusMessages(t *testing.T) {
	var mu sync.Mutex
	var got []string
	SetLogFunc(func(msg string) {
		mu.Lock()
		got = append(got, msg)
		mu.Unlock()
	})
	defer SetLogFunc(nil)

	q := queue.NewMemoryQueue(1, 2)
	q.Start()
	q.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2: %q", len(got), got)
	}
	if !strings.Contains(got[0], "Queue started with 2 workers") || !strings.Contains(got[1], "Queue stopped") {
		t.Fatalf("unexpected messages: %q", got)
	}
	for _, m := range got {
		if strings.ContainsAny(m, "\r\n") {
			t.Fatalf("message carries a newline: %q", m)
		}
	}
}
