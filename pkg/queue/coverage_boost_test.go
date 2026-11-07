package queue

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryQueue_RequeueFullTriggersDeadLetter(t *testing.T) {
	var dead atomic.Bool
	done := make(chan struct{})
	q := NewMemoryQueue(1, 0, // no workers; we call process directly
		WithDeadLetter(func(job Job, err error) {
			dead.Store(true)
			close(done)
		}),
		WithRetryWait(5*time.Millisecond),
	)
	q.Register("flaky", func(job Job) error { return errFail })
	if err := q.Push("fill", nil); err != nil { // occupy the only buffer slot
		t.Fatal(err)
	}
	q.process(Job{ID: "1", Name: "flaky", Attempt: 1, MaxRetries: 1})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dead letter not called on requeue-full")
	}
	if !dead.Load() {
		t.Fatal("want dead letter")
	}
}
