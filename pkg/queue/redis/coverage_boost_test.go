package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/pkg/queue"
	goredis "github.com/redis/go-redis/v9"
)

func TestNew_DefaultsAndPushEdges(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	q := New(client, 0, // bump to 1
		WithKeyPrefix(""),
		WithBRPopTimeout(0),
		WithRetryWait(0),
		WithVisibilityTimeout(0),
	)
	if q.workerCount != 1 {
		t.Fatalf("workerCount=%d", q.workerCount)
	}
	if q.prefix != defaultPrefix {
		t.Fatalf("prefix=%q", q.prefix)
	}

	// unmarshalable payload
	if err := q.Push("x", make(chan int)); err == nil {
		t.Fatal("want marshal error")
	}
	if err := q.PushWithOptions("x", "ok", queue.PushOptions{MaxRetries: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestReclaimOrphans_Edges(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	q := New(client, 1, WithKeyPrefix("edge:"), WithVisibilityTimeout(time.Second))
	ctx := context.Background()

	// empty processing list
	q.reclaimOrphans(ctx)

	// has inflight deadline — leave alone
	keep := `{"id":"keep"}`
	_ = client.LPush(ctx, q.processingKey(), keep).Err()
	_ = client.ZAdd(ctx, q.inflightKey(), goredis.Z{
		Score:  float64(time.Now().Add(time.Hour).UnixMilli()),
		Member: keep,
	}).Err()
	q.reclaimOrphans(ctx)
	if n, _ := client.LLen(ctx, q.processingKey()).Result(); n != 1 {
		t.Fatalf("inflight job removed, len=%d", n)
	}

	// orphan without inflight — reclaimed to ready
	orphan := `{"id":"orphan"}`
	_ = client.LPush(ctx, q.processingKey(), orphan).Err()
	q.reclaimOrphans(ctx)
	if n, _ := client.LLen(ctx, q.readyKey()).Result(); n < 1 {
		t.Fatal("orphan not pushed to ready")
	}

	// ZScore error other than Nil (WRONGTYPE) — continue
	_ = client.LPush(ctx, q.processingKey(), `{"id":"wt"}`).Err()
	_ = client.Del(ctx, q.inflightKey()).Err()
	_ = client.Set(ctx, q.inflightKey(), "not-a-zset", 0).Err()
	q.reclaimOrphans(ctx)

	// LRem n==0: list entry already gone between LRange and LRem is hard;
	// simulate by making processing a non-list so LRem errors (err != nil branch).
	_ = client.Del(ctx, q.processingKey()).Err()
	_ = client.RPush(ctx, q.processingKey(), `{"id":"x1"}`).Err()
	// Force LRange success then LRem failure: convert key after a custom approach —
	// instead, put orphan and delete list item via LPop before reclaim by racing:
	// use two identical members; after first reclaim removes one, second iteration
	// still finds the member string in the prior LRange snapshot... actually LRange
	// is taken once. Put member, LRange inside reclaim sees it; we can't interleave.
	// Cover LRange error path:
	mr.Close()
	q.reclaimOrphans(ctx)
}

func TestProcessAndAck_Edges(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	var dead bool
	q := New(client, 1,
		WithKeyPrefix("pa:"),
		WithRetryWait(time.Millisecond),
		WithDeadLetter(func(job queue.Job, err error) { dead = true }),
	)
	ctx := context.Background()

	q.processAndAck(ctx, "not-json")
	q.processAndAck(ctx, `{"id":"1","name":"missing","payload":"1","created_at":"2020-01-01T00:00:00Z","attempt":1,"max_retries":0}`)

	q.Register("fail", func(job queue.Job) error { return errFail })
	q.processAndAck(ctx, `{"id":"2","name":"fail","payload":"null","created_at":"2020-01-01T00:00:00Z","attempt":1,"max_retries":0}`)
	if !dead {
		t.Fatal("want dead letter")
	}

	dead = false
	q.Register("retry", func(job queue.Job) error { return errFail })
	q.processAndAck(ctx, `{"id":"3","name":"retry","payload":"\"x\"","created_at":"2020-01-01T00:00:00Z","attempt":1,"max_retries":2}`)
	// requeued to delayed; close redis so enqueueDelayed fails on next fail after retries exhausted with onDead
	mr.Close()
	q.processAndAck(ctx, `{"id":"4","name":"retry","payload":"\"x\"","created_at":"2020-01-01T00:00:00Z","attempt":1,"max_retries":2}`)
}

func TestReclaimExpired_ZRemRace(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	q := New(client, 1, WithKeyPrefix("re:"), WithVisibilityTimeout(time.Second))
	ctx := context.Background()
	payload := `{"id":"1","name":"echo","payload":"\"x\"","created_at":"2020-01-01T00:00:00Z","attempt":1,"max_retries":0}`
	_ = client.LPush(ctx, q.processingKey(), payload).Err()
	_ = client.ZAdd(ctx, q.inflightKey(), goredis.Z{Score: 1, Member: payload}).Err()
	q.reclaimExpired(ctx)

	_ = client.ZAdd(ctx, q.inflightKey(), goredis.Z{Score: 1, Member: "gone"}).Err()
	_ = client.ZRem(ctx, q.inflightKey(), "gone").Err()
	q.promoteDelayed(ctx)
	_ = client.ZAdd(ctx, q.delayedKey(), goredis.Z{Score: 1, Member: "due"}).Err()
	q.promoteDelayed(ctx)
}

func TestWorkerLoop_RedisErrors(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	q := New(client, 1,
		WithKeyPrefix("we:"),
		WithBRPopTimeout(50*time.Millisecond),
		WithVisibilityTimeout(time.Second),
	)
	q.Register("echo", func(job queue.Job) error { return nil })

	// ZADD inflight failure while a normal worker is running.
	q.Start()
	_ = client.Set(context.Background(), q.inflightKey(), "not-zset", 0).Err()
	_ = q.Push("echo", "x")
	time.Sleep(200 * time.Millisecond)
	q.Stop()

	// BRPOPLPUSH error → log/sleep/continue (quit still open).
	mr2, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr2.Close()
	client2 := goredis.NewClient(&goredis.Options{Addr: mr2.Addr()})
	defer client2.Close()
	q2 := New(client2, 1,
		WithKeyPrefix("we2:"),
		WithBRPopTimeout(50*time.Millisecond),
		WithVisibilityTimeout(time.Second),
	)
	// Cover log/sleep/continue while quit is still open.
	mr2.SetError("forced brpop failure")
	done := make(chan struct{})
	go func() {
		defer close(done)
		q2.wg.Add(1)
		q2.workerLoop(0)
	}()
	time.Sleep(200 * time.Millisecond)
	mr2.SetError("")
	close(q2.quit)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("workerLoop did not exit after quit")
	}

	// Cover err!=nil select case <-quit: close quit while BRPOP is blocked, then
	// kill redis so BRPOP returns a non-Nil error and the select takes the quit path.
	mr3, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client3 := goredis.NewClient(&goredis.Options{Addr: mr3.Addr()})
	defer client3.Close()
	q3 := New(client3, 1,
		WithKeyPrefix("we3:"),
		WithBRPopTimeout(5*time.Second), // block long enough to interleave
	)
	done3 := make(chan struct{})
	go func() {
		defer close(done3)
		q3.wg.Add(1)
		q3.workerLoop(0)
	}()
	time.Sleep(50 * time.Millisecond) // allow BRPOP to block
	close(q3.quit)
	mr3.Close() // BRPOP errors; quit already closed → line return
	select {
	case <-done3:
	case <-time.After(3 * time.Second):
		t.Fatal("workerLoop 3 timeout")
	}
}

func TestEnqueue_WireMarshalErrors(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	q := New(client, 1, WithKeyPrefix("wm:"))
	prev := wireMarshal
	wireMarshal = func(v interface{}) ([]byte, error) { return nil, errFail }
	defer func() { wireMarshal = prev }()

	if err := q.enqueueReady(jobWire{ID: "1", Name: "n"}); err == nil {
		t.Fatal("want enqueueReady marshal error")
	}
	if err := q.enqueueDelayed(jobWire{ID: "2", Name: "n"}, time.Millisecond); err == nil {
		t.Fatal("want enqueueDelayed marshal error")
	}
}

func TestReclaimExpired_ZRemZero(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	q := New(client, 1, WithKeyPrefix("zr:"), WithVisibilityTimeout(time.Second))
	ctx := context.Background()
	payload := `{"id":"race"}`

	// Concurrent reclaim/promote so one ZRem wins (n=1) and the other sees n=0.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_ = client.ZAdd(ctx, q.inflightKey(), goredis.Z{Score: 1, Member: payload}).Err()
				_ = client.LPush(ctx, q.processingKey(), payload).Err()
				q.reclaimExpired(ctx)
				_ = client.ZAdd(ctx, q.delayedKey(), goredis.Z{Score: 1, Member: payload}).Err()
				// Pre-remove so promoteDelayed's ZRem often returns n=0.
				_, _ = client.ZRem(ctx, q.delayedKey(), payload).Result()
				q.promoteDelayed(ctx)
			}
		}()
	}
	wg.Wait()
}

func TestReclaimOrphans_LRemZero(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	q := New(client, 1, WithKeyPrefix("lz:"))
	ctx := context.Background()
	payload := `{"id":"dup"}`
	// Two identical members; first reclaim LRem removes one.
	// Second call: LRange still may see remaining; exercise n==0 by removing
	// between snapshot and LRem via concurrent LRem of the only element.
	_ = client.RPush(ctx, q.processingKey(), payload).Err()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_ = client.LRem(ctx, q.processingKey(), 1, payload).Err()
			_ = client.RPush(ctx, q.processingKey(), payload).Err()
			_ = client.LRem(ctx, q.processingKey(), 1, payload).Err()
		}
	}()
	for i := 0; i < 30; i++ {
		_ = client.RPush(ctx, q.processingKey(), payload).Err()
		q.reclaimOrphans(ctx)
	}
	<-done
}
