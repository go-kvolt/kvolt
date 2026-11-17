// Package redis provides a Redis-backed job queue with at-least-once delivery.
// Import only when needed; in-memory queue stays in pkg/queue.
package redis

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-kvolt/kvolt/v2/internal/logx"
	"github.com/go-kvolt/kvolt/v2/pkg/queue"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultPrefix            = "kvolt:queue:"
	defaultBRPopTimeout      = time.Second
	defaultRetryWait         = 100 * time.Millisecond
	defaultVisibilityTimeout = 30 * time.Second
)

// Overridable in tests to exercise wire marshal failure paths.
var wireMarshal = sonic.Marshal

// moveBatch bounds how many jobs one script call moves, so a large backlog
// cannot block Redis for long.
const moveBatch = 100

// moveDueScript atomically moves members of a sorted set whose score is due
// back to the ready list. A job is never in neither place, so a crash between
// the steps cannot lose it.
//
//	KEYS[1] sorted set (delayed or inflight)   ARGV[1] now (ms)
//	KEYS[2] ready list                         ARGV[2] max members to move
//	KEYS[3] processing list (optional)
var moveDueScript = goredis.NewScript(`
local due = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
for _, m in ipairs(due) do
  redis.call('ZREM', KEYS[1], m)
  if KEYS[3] then
    redis.call('LREM', KEYS[3], 1, m)
  end
  redis.call('LPUSH', KEYS[2], m)
end
return #due
`)

// reclaimOrphanScript atomically moves one processing entry that has no inflight
// deadline back to ready.
//
//	KEYS[1] processing list   KEYS[2] inflight sorted set   KEYS[3] ready list
//	ARGV[1] job payload
var reclaimOrphanScript = goredis.NewScript(`
if redis.call('ZSCORE', KEYS[2], ARGV[1]) then
  return 0
end
if redis.call('LREM', KEYS[1], 1, ARGV[1]) == 0 then
  return 0
end
redis.call('LPUSH', KEYS[3], ARGV[1])
return 1
`)

type jobWire struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Payload    sonic.NoCopyRawMessage `json:"payload"`
	CreatedAt  time.Time              `json:"created_at"`
	Attempt    int                    `json:"attempt"`
	MaxRetries int                    `json:"max_retries"`
}

// Queue is a Redis LIST queue with processing list + visibility timeout (at-least-once).
//
// Flow: BRPOPLPUSH ready→processing, ZADD inflight deadline, handler, then ACK (LREM+ZREM).
// If a worker dies, a reaper moves expired inflight jobs back to ready.
type Queue struct {
	client      *goredis.Client
	prefix      string
	workerCount int
	brpopWait   time.Duration
	retryWait   time.Duration
	visibility  time.Duration
	onDead      queue.DeadLetterFunc
	handlers    map[string]queue.HandlerFunc
	quit        chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// Option configures Queue.
type Option func(*Queue)

// WithDeadLetter sets a callback for jobs that fail after all retries.
func WithDeadLetter(fn queue.DeadLetterFunc) Option {
	return func(q *Queue) { q.onDead = fn }
}

// WithRetryWait sets delay before re-queueing a failed job.
func WithRetryWait(d time.Duration) Option {
	return func(q *Queue) {
		if d > 0 {
			q.retryWait = d
		}
	}
}

// WithKeyPrefix sets the Redis key prefix.
func WithKeyPrefix(prefix string) Option {
	return func(q *Queue) {
		if prefix != "" {
			q.prefix = prefix
		}
	}
}

// WithBRPopTimeout sets BRPOPLPUSH timeout (minimum 1s for go-redis).
func WithBRPopTimeout(d time.Duration) Option {
	return func(q *Queue) {
		if d > 0 {
			q.brpopWait = d
		}
	}
}

// WithVisibilityTimeout is how long a job may stay in processing before reclaim (default 30s).
func WithVisibilityTimeout(d time.Duration) Option {
	return func(q *Queue) {
		if d > 0 {
			q.visibility = d
		}
	}
}

// New creates a Redis-backed queue.
func New(client *goredis.Client, workerCount int, opts ...Option) *Queue {
	if workerCount < 1 {
		workerCount = 1
	}
	q := &Queue{
		client:      client,
		prefix:      defaultPrefix,
		workerCount: workerCount,
		brpopWait:   defaultBRPopTimeout,
		retryWait:   defaultRetryWait,
		visibility:  defaultVisibilityTimeout,
		handlers:    make(map[string]queue.HandlerFunc),
		quit:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

func (q *Queue) readyKey() string      { return q.prefix + "ready" }
func (q *Queue) processingKey() string { return q.prefix + "processing" }
func (q *Queue) inflightKey() string   { return q.prefix + "inflight" }
func (q *Queue) delayedKey() string    { return q.prefix + "delayed" }

// Push implements queue.Queue.
func (q *Queue) Push(name string, payload interface{}) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("queue is nil or client is nil")
	}
	return q.PushWithOptions(name, payload, queue.PushOptions{})
}

// PushWithOptions implements queue.Queue.
func (q *Queue) PushWithOptions(name string, payload interface{}, opts queue.PushOptions) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("queue is nil or client is nil")
	}
	payloadBytes, err := sonic.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue redis marshal payload: %w", err)
	}
	job := jobWire{
		ID:         uuid.New().String(),
		Name:       name,
		Payload:    payloadBytes,
		CreatedAt:  time.Now().UTC(),
		Attempt:    1,
		MaxRetries: opts.MaxRetries,
	}
	return q.enqueueReady(job)
}

func (q *Queue) enqueueReady(job jobWire) error {
	data, err := wireMarshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(context.Background(), q.readyKey(), data).Err()
}

func (q *Queue) enqueueDelayed(job jobWire, delay time.Duration) error {
	data, err := wireMarshal(job)
	if err != nil {
		return err
	}
	score := float64(time.Now().Add(delay).UnixMilli())
	return q.client.ZAdd(context.Background(), q.delayedKey(), goredis.Z{
		Score:  score,
		Member: string(data),
	}).Err()
}

// Register implements queue.Worker.
func (q *Queue) Register(name string, handler queue.HandlerFunc) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.handlers == nil {
		q.handlers = make(map[string]queue.HandlerFunc)
	}
	q.handlers[name] = handler
}

// Start implements queue.Worker.
func (q *Queue) Start() {
	if q == nil || q.client == nil {
		return
	}
	q.wg.Add(1)
	go q.reaperLoop()
	for i := 0; i < q.workerCount; i++ {
		q.wg.Add(1)
		go q.workerLoop(i)
	}
	logx.Printf("Queue (redis) started with %d workers (visibility=%s)\n", q.workerCount, q.visibility)
}

// Stop implements queue.Worker.
func (q *Queue) Stop() {
	if q == nil {
		return
	}
	q.stopOnce.Do(func() {
		if q.quit != nil {
			close(q.quit)
		}
		q.wg.Wait()
		logx.Println("Queue (redis) stopped")
	})
}

func (q *Queue) workerLoop(id int) {
	defer q.wg.Done()
	ctx := context.Background()
	for {
		select {
		case <-q.quit:
			return
		default:
		}

		q.promoteDelayed(ctx)

		// Stop may have been called while delayed jobs were being promoted: do not
		// start a blocking pop (up to the BRPOP timeout) that Stop would then wait for.
		select {
		case <-q.quit:
			return
		default:
		}

		data, err := q.client.BRPopLPush(ctx, q.readyKey(), q.processingKey(), q.brpopWait).Result()
		if err == goredis.Nil {
			continue
		}
		if err != nil {
			select {
			case <-q.quit:
				return
			default:
				log.Printf("queue redis BRPOPLPUSH: %v\n", err)
				time.Sleep(50 * time.Millisecond)
				continue
			}
		}

		deadline := float64(time.Now().Add(q.visibility).UnixMilli())
		if err := q.client.ZAdd(ctx, q.inflightKey(), goredis.Z{Score: deadline, Member: data}).Err(); err != nil {
			// Leave job in processing; reaper/orphan reclaim will retry.
			log.Printf("queue redis ZADD inflight: %v\n", err)
			continue
		}
		q.processAndAck(ctx, data)
	}
}

func (q *Queue) reaperLoop() {
	defer q.wg.Done()
	if q == nil || q.client == nil {
		return
	}
	vis := q.visibility
	if vis < 2*time.Second {
		vis = 2 * time.Second
	}
	ticker := time.NewTicker(vis / 2)
	defer ticker.Stop()
	for {
		select {
		case <-q.quit:
			return
		case <-ticker.C:
			q.reclaimExpired(context.Background())
		}
	}
}

func (q *Queue) reclaimExpired(ctx context.Context) {
	if q == nil || q.client == nil {
		return
	}
	n, err := q.moveDue(ctx, q.inflightKey(), q.processingKey())
	if err != nil {
		log.Printf("queue redis: reclaim expired: %v\n", err)
	} else if n > 0 {
		log.Printf("queue redis: reclaimed %d expired job(s)\n", n)
	}
	// Jobs stuck in processing without an inflight deadline (crash between
	// BRPOPLPUSH and ZADD) would never be reaped — recover them here.
	q.reclaimOrphans(ctx)
}

// reclaimOrphans moves processing-list entries that have no inflight score back to ready.
func (q *Queue) reclaimOrphans(ctx context.Context) {
	if q == nil || q.client == nil {
		return
	}
	items, err := q.client.LRange(ctx, q.processingKey(), 0, -1).Result()
	if err != nil || len(items) == 0 {
		return
	}
	for _, m := range items {
		moved, err := reclaimOrphanScript.Run(ctx, q.client,
			[]string{q.processingKey(), q.inflightKey(), q.readyKey()}, m).Int()
		if err != nil || moved == 0 {
			continue // has a deadline (left for score-based reclaim), already gone, or Redis error
		}
		log.Printf("queue redis: reclaimed orphan processing job\n")
	}
}

// moveDue moves every due member of zsetKey to the ready list, in batches.
// processingKey is optional: when set, each member is also removed from that list.
func (q *Queue) moveDue(ctx context.Context, zsetKey, processingKey string) (int, error) {
	keys := []string{zsetKey, q.readyKey()}
	if processingKey != "" {
		keys = append(keys, processingKey)
	}
	total := 0
	for {
		now := time.Now().UnixMilli()
		n, err := moveDueScript.Run(ctx, q.client, keys, now, moveBatch).Int()
		if err != nil {
			return total, err
		}
		total += n
		if n < moveBatch {
			return total, nil
		}
	}
}

func (q *Queue) promoteDelayed(ctx context.Context) {
	if q == nil || q.client == nil {
		return
	}
	if _, err := q.moveDue(ctx, q.delayedKey(), ""); err != nil {
		select {
		case <-q.quit:
		default:
			log.Printf("queue redis: promote delayed: %v\n", err)
		}
	}
}

func (q *Queue) ack(ctx context.Context, data string) {
	if q == nil || q.client == nil {
		return
	}
	// One transaction: a job must not be left in only one of the two structures,
	// or the reaper would deliver it again after it already succeeded.
	pipe := q.client.TxPipeline()
	pipe.LRem(ctx, q.processingKey(), 1, data)
	pipe.ZRem(ctx, q.inflightKey(), data)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("queue redis: ack failed (job may be redelivered): %v\n", err)
	}
}

func (q *Queue) processAndAck(ctx context.Context, data string) {
	var wire jobWire
	if err := sonic.Unmarshal([]byte(data), &wire); err != nil {
		log.Printf("queue redis bad job json: %v\n", err)
		q.ack(ctx, data)
		return
	}

	var payload interface{}
	if len(wire.Payload) > 0 {
		_ = sonic.Unmarshal(wire.Payload, &payload)
	}
	job := queue.Job{
		ID:         wire.ID,
		Name:       wire.Name,
		Payload:    payload,
		CreatedAt:  wire.CreatedAt,
		Attempt:    wire.Attempt,
		MaxRetries: wire.MaxRetries,
	}

	q.mu.RLock()
	handler, exists := q.handlers[job.Name]
	q.mu.RUnlock()
	if !exists {
		log.Printf("No handler for job: %s\n", job.Name)
		q.ack(ctx, data)
		return
	}

	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in queue redis handler: %v", r)
			}
		}()
		err = handler(job)
	}()

	if err != nil {
		log.Printf("Job %s attempt %d failed: %v\n", job.ID, job.Attempt, err)
		if job.Attempt <= job.MaxRetries {
			wire.Attempt++
			if e := q.enqueueDelayed(wire, q.retryWait); e != nil {
				log.Printf("Job %s requeue failed: %v\n", job.ID, e)
				if q.onDead != nil {
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("queue redis onDead panic: %v\n", r)
							}
						}()
						q.onDead(job, err)
					}()
				}
			}
			q.ack(ctx, data)
			return
		}
		q.ack(ctx, data)
		if q.onDead != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("queue redis onDead panic: %v\n", r)
					}
				}()
				q.onDead(job, err)
			}()
		}
		return
	}
	q.ack(ctx, data)
}

var (
	_ queue.Queue  = (*Queue)(nil)
	_ queue.Worker = (*Queue)(nil)
)
