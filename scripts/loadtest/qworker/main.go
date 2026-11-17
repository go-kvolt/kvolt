// Queue worker process for the kill test. Records each completed job in a Redis hash
// before returning, so the driver can tell which jobs ran and how many times.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-kvolt/kvolt/v2/pkg/queue"
	kredis "github.com/go-kvolt/kvolt/v2/pkg/queue/redis"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	addr := flag.String("redis", "127.0.0.1:6379", "redis address")
	prefix := flag.String("prefix", "", "key prefix")
	hangAfter := flag.Int("hang-after", -1, "block every handler forever after this many completions (-1: never)")
	vis := flag.Duration("vis", 2*time.Second, "visibility timeout")
	flag.Parse()
	log.SetOutput(os.Stderr)

	client := goredis.NewClient(&goredis.Options{Addr: *addr})
	defer client.Close()
	ctx := context.Background()

	var completed atomic.Int64
	q := kredis.New(client, 4,
		kredis.WithKeyPrefix(*prefix+"q:"),
		kredis.WithVisibilityTimeout(*vis),
		kredis.WithBRPopTimeout(time.Second),
		kredis.WithRetryWait(150*time.Millisecond),
		kredis.WithDeadLetter(func(job queue.Job, err error) {
			client.HIncrBy(ctx, *prefix+"dead", fmt.Sprint(job.Payload), 1)
		}),
	)
	q.Register("work", func(job queue.Job) error {
		idx, ok := job.Payload.(float64)
		if !ok {
			return fmt.Errorf("unexpected payload %T", job.Payload)
		}
		if *hangAfter >= 0 && completed.Load() >= int64(*hangAfter) {
			select {} // a stuck worker: holds the job until the process is killed
		}
		// Every tenth job fails on its first attempt and goes through the retry path.
		if int(idx)%10 == 3 && job.Attempt == 1 {
			return errors.New("transient failure")
		}
		time.Sleep(3 * time.Millisecond)
		if err := client.HIncrBy(ctx, *prefix+"done", strconv.Itoa(int(idx)), 1).Err(); err != nil {
			return err
		}
		completed.Add(1)
		return nil
	})
	q.Start()

	// Graceful stop when the driver closes stdin.
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	q.Stop()
	fmt.Println("worker stopped cleanly")
}
