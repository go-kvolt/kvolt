package queue

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryQueue_Push(t *testing.T) {
	q := NewMemoryQueue(10, 1)
	defer q.Stop()

	q.Start()
	time.Sleep(10 * time.Millisecond)

	err := q.Push("test", "payload")
	if err != nil {
		t.Errorf("Push: %v", err)
	}
}

func TestMemoryQueue_PushFull(t *testing.T) {
	q := NewMemoryQueue(1, 0) // buffer 1, no workers so channel never drained
	err := q.Push("first", nil)
	if err != nil {
		t.Fatalf("first Push: %v", err)
	}
	err = q.Push("second", nil) // buffer full
	if err == nil {
		t.Error("Push when full: want error")
	}
}

func TestMemoryQueue_RetryAndDeadLetter(t *testing.T) {
	var dead Job
	var deadErr error
	done := make(chan struct{})
	attempts := 0
	q := NewMemoryQueue(10, 1,
		WithDeadLetter(func(job Job, err error) {
			dead = job
			deadErr = err
			close(done)
		}),
		WithRetryWait(5*time.Millisecond),
	)
	q.Register("flaky", func(job Job) error {
		attempts++
		return errFail
	})
	q.Start()
	defer q.Stop()

	if err := q.PushWithOptions("flaky", "x", PushOptions{MaxRetries: 2}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dead letter not called")
	}
	if attempts != 3 { // 1 + 2 retries
		t.Fatalf("attempts want 3 got %d", attempts)
	}
	if deadErr == nil || dead.Name != "flaky" {
		t.Fatalf("dead letter: %+v %v", dead, deadErr)
	}
}

var errFail = errString("fail")

type errString string

func (e errString) Error() string { return string(e) }

func TestMemoryQueue_RegisterAndProcess(t *testing.T) {
	q := NewMemoryQueue(10, 1)
	var (
		mu    sync.Mutex
		seen  []Job
		done  = make(chan struct{})
		count int
	)
	q.Register("echo", func(job Job) error {
		mu.Lock()
		seen = append(seen, job)
		count++
		if count >= 2 {
			close(done)
		}
		mu.Unlock()
		return nil
	})
	q.Start()
	defer q.Stop()

	q.Push("echo", "a")
	q.Push("echo", "b")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler not called in time")
	}
	mu.Lock()
	n := len(seen)
	mu.Unlock()
	if n < 2 {
		t.Errorf("want at least 2 jobs processed, got %d", n)
	}
}
