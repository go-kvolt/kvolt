package redis

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/pkg/queue"
	goredis "github.com/redis/go-redis/v9"
)

func newTestQueue(t *testing.T, workers int, opts ...Option) (*Queue, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	opts = append([]Option{
		WithKeyPrefix("t:q:"),
		WithBRPopTimeout(time.Second),
		WithVisibilityTimeout(2 * time.Second),
	}, opts...)
	q := New(client, workers, opts...)
	t.Cleanup(func() {
		q.Stop()
		client.Close()
		mr.Close()
	})
	return q, mr
}

func TestQueue_RegisterAndProcess(t *testing.T) {
	q, _ := newTestQueue(t, 1)
	done := make(chan struct{})
	q.Register("echo", func(job queue.Job) error {
		s, _ := job.Payload.(string)
		if s != "hi" {
			t.Errorf("payload %#v", job.Payload)
		}
		close(done)
		return nil
	})
	q.Start()
	if err := q.Push("echo", "hi"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for job")
	}
}

func TestQueue_RetryAndDeadLetter(t *testing.T) {
	var attempts atomic.Int32
	done := make(chan struct{})
	q, _ := newTestQueue(t, 1,
		WithDeadLetter(func(job queue.Job, err error) { close(done) }),
		WithRetryWait(20*time.Millisecond),
	)
	q.Register("flaky", func(job queue.Job) error {
		attempts.Add(1)
		return errFail
	})
	q.Start()
	if err := q.PushWithOptions("flaky", "x", queue.PushOptions{MaxRetries: 2}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("dead letter not called")
	}
	if n := attempts.Load(); n != 3 {
		t.Fatalf("attempts want 3 got %d", n)
	}
}

func TestQueue_VisibilityReclaim(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	// Manually put a job in processing + expired inflight (simulates crashed worker).
	payload := `{"id":"1","name":"echo","payload":"\"x\"","created_at":"2020-01-01T00:00:00Z","attempt":1,"max_retries":0}`
	_ = client.LPush(context.Background(), "vis:processing", payload).Err()
	_ = client.ZAdd(context.Background(), "vis:inflight", goredis.Z{Score: 1, Member: payload}).Err() // expired

	q := New(client, 1,
		WithKeyPrefix("vis:"),
		WithVisibilityTimeout(time.Second),
		WithBRPopTimeout(time.Second),
	)
	done := make(chan struct{})
	q.Register("echo", func(job queue.Job) error {
		close(done)
		return nil
	})
	q.Start()
	defer q.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reclaimed job not processed")
	}
}

func TestQueue_OrphanReclaim(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	// Processing entry with NO inflight score (crash between BRPOPLPUSH and ZADD).
	payload := `{"id":"2","name":"echo","payload":"\"y\"","created_at":"2020-01-01T00:00:00Z","attempt":1,"max_retries":0}`
	_ = client.LPush(context.Background(), "orp:processing", payload).Err()

	q := New(client, 1,
		WithKeyPrefix("orp:"),
		WithVisibilityTimeout(time.Second),
		WithBRPopTimeout(time.Second),
	)
	done := make(chan struct{})
	q.Register("echo", func(job queue.Job) error {
		close(done)
		return nil
	})
	q.Start()
	defer q.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("orphan not reclaimed")
	}

	// Direct call covers empty/error early returns.
	q.reclaimOrphans(context.Background())
	mr.Close()
	q.reclaimOrphans(context.Background())
}

type errString string

func (e errString) Error() string { return string(e) }

var errFail = errString("fail")

func TestQueue_NilAndDoubleStop(t *testing.T) {
	var qNil *Queue
	qNil.Stop()
	qNil.Start()
	qNil.Register("test", nil)
	if err := qNil.Push("test", nil); err == nil {
		t.Fatal("expected error on nil queue push")
	}

	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	q := New(client, 1)
	q.Start()
	// Multiple Stop calls must not panic (close of closed channel)
	q.Stop()
	q.Stop()
}
