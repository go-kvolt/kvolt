// Kill test for the Redis queue: a worker process is killed while holding jobs, and a
// second worker must finish every job. No job may be lost.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kvolt/kvolt/v2/pkg/queue"
	kredis "github.com/go-kvolt/kvolt/v2/pkg/queue/redis"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	addr := flag.String("redis", "", "redis address (empty: start an in-process miniredis)")
	workerExe := flag.String("worker", "", "worker binary")
	jobs := flag.Int("jobs", 300, "number of jobs")
	flag.Parse()

	failed := false
	fail := func(format string, a ...interface{}) {
		failed = true
		fmt.Printf("FAIL: "+format+"\n", a...)
	}

	backend := "real Redis at " + *addr
	if *addr == "" {
		mr, err := miniredis.Run()
		if err != nil {
			fmt.Println("FATAL:", err)
			os.Exit(2)
		}
		defer mr.Close()
		*addr = mr.Addr()
		backend = "miniredis at " + *addr
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	prefix := "kvolt-soak-" + hex.EncodeToString(b) + ":"
	fmt.Printf("backend: %s, key prefix %s, jobs %d\n", backend, prefix, *jobs)

	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: *addr})
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Println("FATAL: redis ping:", err)
		os.Exit(2)
	}
	cleanup := func() {
		var cursor uint64
		n := 0
		for {
			keys, next, err := client.Scan(ctx, cursor, prefix+"*", 200).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				n += len(keys)
				client.Del(ctx, keys...)
			}
			if cursor = next; cursor == 0 {
				break
			}
		}
		fmt.Printf("cleanup: removed %d test keys\n", n)
	}
	defer cleanup()

	qKeys := func() (ready, processing, inflight, delayed int64) {
		ready = client.LLen(ctx, prefix+"q:ready").Val()
		processing = client.LLen(ctx, prefix+"q:processing").Val()
		inflight = client.ZCard(ctx, prefix+"q:inflight").Val()
		delayed = client.ZCard(ctx, prefix+"q:delayed").Val()
		return
	}
	done := func() int64 { return client.HLen(ctx, prefix+"done").Val() }

	producer := kredis.New(client, 1, kredis.WithKeyPrefix(prefix+"q:"))
	for i := 0; i < *jobs; i++ {
		if err := producer.PushWithOptions("work", i, queue.PushOptions{MaxRetries: 2}); err != nil {
			fmt.Println("FATAL: push:", err)
			os.Exit(2)
		}
	}

	startWorker := func(name string, extra ...string) (*exec.Cmd, io.WriteCloser) {
		args := append([]string{"-redis", *addr, "-prefix", prefix, "-vis", "2s"}, extra...)
		cmd := exec.Command(*workerExe, args...)
		logf, _ := os.Create("worker-" + name + ".log")
		cmd.Stdout, cmd.Stderr = logf, logf
		stdin, _ := cmd.StdinPipe()
		if err := cmd.Start(); err != nil {
			fmt.Println("FATAL: start worker:", err)
			os.Exit(2)
		}
		return cmd, stdin
	}

	// Worker A processes some jobs, then every handler blocks: it is holding jobs.
	workerA, _ := startWorker("A", "-hang-after", "40")
	deadline := time.Now().Add(40 * time.Second)
	last, stable := int64(-1), 0
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		n := done()
		if n >= 40 && n == last {
			if stable++; stable >= 3 {
				break
			}
		} else {
			stable = 0
		}
		last = n
	}
	r, p, inf, d := qKeys()
	fmt.Printf("worker A stuck: completed=%d  ready=%d processing=%d inflight=%d delayed=%d\n", done(), r, p, inf, d)
	if p == 0 {
		fail("worker A is not holding any job; the kill would prove nothing")
	}
	held := p
	if err := workerA.Process.Kill(); err != nil {
		fail("kill worker A: %v", err)
	}
	_ = workerA.Wait()
	fmt.Printf("worker A killed (hard) while holding %d job(s)\n", held)

	// Worker B must finish everything, including what A was holding.
	workerB, stdinB := startWorker("B")
	deadline = time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) && done() < int64(*jobs) {
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(5 * time.Second) // more than the visibility timeout: let any redelivery happen

	counts, _ := client.HGetAll(ctx, prefix+"done").Result()
	var lost, dups []int
	for i := 0; i < *jobs; i++ {
		n, _ := strconv.Atoi(counts[strconv.Itoa(i)])
		switch {
		case n == 0:
			lost = append(lost, i)
		case n > 1:
			dups = append(dups, i)
		}
	}
	sort.Ints(lost)
	dead := client.HLen(ctx, prefix+"dead").Val()
	r, p, inf, d = qKeys()
	fmt.Printf("after worker B: completed=%d/%d lost=%d ran-twice=%d dead-lettered=%d  ready=%d processing=%d inflight=%d delayed=%d\n",
		len(counts), *jobs, len(lost), len(dups), dead, r, p, inf, d)
	if len(lost) > 0 {
		fail("%d job(s) were lost: %v", len(lost), lost)
	}
	if dead > 0 {
		fail("%d job(s) were dead-lettered although they succeed on retry", dead)
	}
	if int64(len(dups)) > held {
		fail("%d jobs ran twice, more than the %d that were in flight at the kill: %v", len(dups), held, dups)
	}
	if r+p+inf+d != 0 {
		fail("queue structures not empty after all jobs finished")
	}

	// Graceful stop.
	_ = stdinB.Close()
	exited := make(chan error, 1)
	go func() { exited <- workerB.Wait() }()
	select {
	case err := <-exited:
		if err != nil {
			fail("worker B exit: %v", err)
		} else {
			fmt.Println("worker B stopped gracefully (exit 0)")
		}
	case <-time.After(15 * time.Second):
		_ = workerB.Process.Kill()
		fail("worker B did not stop within 15s")
	}

	if failed {
		fmt.Println("RESULT: FAILED")
		cleanup()
		os.Exit(1)
	}
	fmt.Println("RESULT: PASSED")
}
