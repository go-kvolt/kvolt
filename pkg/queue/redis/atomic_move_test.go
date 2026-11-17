package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newBareQueue(t *testing.T, prefix string) (*Queue, *goredis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New(client, 1, WithKeyPrefix(prefix)), client
}

// Due delayed jobs move to ready; jobs that are not due yet stay delayed.
func TestPromoteDelayed_MovesOnlyDueJobs(t *testing.T) {
	q, client := newBareQueue(t, "pd:")
	ctx := context.Background()
	now := time.Now()
	client.ZAdd(ctx, q.delayedKey(),
		goredis.Z{Score: float64(now.Add(-time.Second).UnixMilli()), Member: "due-1"},
		goredis.Z{Score: float64(now.Add(-time.Minute).UnixMilli()), Member: "due-2"},
		goredis.Z{Score: float64(now.Add(time.Hour).UnixMilli()), Member: "later"},
	)

	q.promoteDelayed(ctx)

	if n := client.LLen(ctx, q.readyKey()).Val(); n != 2 {
		t.Fatalf("ready length = %d, want 2", n)
	}
	left := client.ZRange(ctx, q.delayedKey(), 0, -1).Val()
	if len(left) != 1 || left[0] != "later" {
		t.Fatalf("delayed = %v, want [later]", left)
	}
}

// More due jobs than one script batch are all moved, with none lost or duplicated.
func TestMoveDue_DrainsBacklogLargerThanBatch(t *testing.T) {
	q, client := newBareQueue(t, "mb:")
	ctx := context.Background()
	total := moveBatch*2 + 7
	for i := 0; i < total; i++ {
		client.ZAdd(ctx, q.delayedKey(), goredis.Z{Score: 1, Member: fmt.Sprintf("job-%d", i)})
	}

	n, err := q.moveDue(ctx, q.delayedKey(), "")
	if err != nil {
		t.Fatalf("moveDue: %v", err)
	}
	if n != total {
		t.Fatalf("moved %d, want %d", n, total)
	}
	if left := client.ZCard(ctx, q.delayedKey()).Val(); left != 0 {
		t.Fatalf("%d jobs left in delayed", left)
	}
	ready := client.LRange(ctx, q.readyKey(), 0, -1).Val()
	seen := make(map[string]bool, len(ready))
	for _, m := range ready {
		if seen[m] {
			t.Fatalf("job %s delivered twice", m)
		}
		seen[m] = true
	}
	if len(seen) != total {
		t.Fatalf("ready holds %d distinct jobs, want %d", len(seen), total)
	}
}

// An expired inflight job leaves both processing and inflight and lands in ready exactly once.
func TestReclaimExpired_MovesJobOutOfProcessing(t *testing.T) {
	q, client := newBareQueue(t, "re:")
	ctx := context.Background()
	client.LPush(ctx, q.processingKey(), "expired", "running")
	client.ZAdd(ctx, q.inflightKey(),
		goredis.Z{Score: 1, Member: "expired"},
		goredis.Z{Score: float64(time.Now().Add(time.Hour).UnixMilli()), Member: "running"},
	)

	q.reclaimExpired(ctx)

	if ready := client.LRange(ctx, q.readyKey(), 0, -1).Val(); len(ready) != 1 || ready[0] != "expired" {
		t.Fatalf("ready = %v, want [expired]", ready)
	}
	if proc := client.LRange(ctx, q.processingKey(), 0, -1).Val(); len(proc) != 1 || proc[0] != "running" {
		t.Fatalf("processing = %v, want [running]", proc)
	}
	if inflight := client.ZRange(ctx, q.inflightKey(), 0, -1).Val(); len(inflight) != 1 || inflight[0] != "running" {
		t.Fatalf("inflight = %v, want [running]", inflight)
	}
}

// Orphan reclaim moves only entries with no inflight deadline.
func TestReclaimOrphans_LeavesJobsWithDeadline(t *testing.T) {
	q, client := newBareQueue(t, "ro:")
	ctx := context.Background()
	client.LPush(ctx, q.processingKey(), "orphan", "tracked")
	client.ZAdd(ctx, q.inflightKey(), goredis.Z{Score: float64(time.Now().Add(time.Hour).UnixMilli()), Member: "tracked"})

	q.reclaimOrphans(ctx)

	if ready := client.LRange(ctx, q.readyKey(), 0, -1).Val(); len(ready) != 1 || ready[0] != "orphan" {
		t.Fatalf("ready = %v, want [orphan]", ready)
	}
	if proc := client.LRange(ctx, q.processingKey(), 0, -1).Val(); len(proc) != 1 || proc[0] != "tracked" {
		t.Fatalf("processing = %v, want [tracked]", proc)
	}
}

// Ack removes the job from both processing and inflight.
func TestAck_ClearsProcessingAndInflight(t *testing.T) {
	q, client := newBareQueue(t, "ak:")
	ctx := context.Background()
	client.LPush(ctx, q.processingKey(), "job")
	client.ZAdd(ctx, q.inflightKey(), goredis.Z{Score: 1, Member: "job"})

	q.ack(ctx, "job")

	if n := client.LLen(ctx, q.processingKey()).Val(); n != 0 {
		t.Fatalf("processing length = %d, want 0", n)
	}
	if n := client.ZCard(ctx, q.inflightKey()).Val(); n != 0 {
		t.Fatalf("inflight size = %d, want 0", n)
	}
}

// Redis errors are reported, not panicked on, and do not report moved jobs.
func TestMoveDue_RedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), MaxRetries: -1, DialTimeout: 200 * time.Millisecond})
	defer client.Close()
	q := New(client, 1, WithKeyPrefix("dn:"))
	mr.Close()

	ctx := context.Background()
	if n, err := q.moveDue(ctx, q.delayedKey(), ""); err == nil || n != 0 {
		t.Fatalf("moveDue = %d, %v; want 0 and an error", n, err)
	}
	q.promoteDelayed(ctx)
	q.reclaimExpired(ctx)
	q.ack(ctx, "job")
	close(q.quit)
	q.promoteDelayed(ctx) // stopping queue: error is not logged
}
