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

func TestMemoryQueue_Uninitialized(t *testing.T) {
	var q *MemoryQueue
	q.Register("test", func(job Job) error { return nil })
	q.Start()
	q.Stop()
	if err := q.Push("test", nil); err == nil {
		t.Fatal("expected error on nil queue push")
	}

	q2 := &MemoryQueue{}
	q2.Register("test", func(job Job) error { return nil })
	q2.Start()
	q2.Stop()
	if err := q2.Push("test", nil); err == nil {
		t.Fatal("expected error on uninitialized queue push")
	}
}
