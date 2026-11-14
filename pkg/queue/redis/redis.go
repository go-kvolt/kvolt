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
	return q.PushWithOptions(name, payload, queue.PushOptions{})
}

// PushWithOptions implements queue.Queue.
func (q *Queue) PushWithOptions(name string, payload interface{}, opts queue.PushOptions) error {
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
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[name] = handler
}

// Start implements queue.Worker.
func (q *Queue) Start() {
	q.wg.Add(1)
	go q.reaperLoop()
	for i := 0; i < q.workerCount; i++ {
		q.wg.Add(1)
		go q.workerLoop(i)
	}
	fmt.Printf("Queue (redis) started with %d workers (visibility=%s)\n", q.workerCount, q.visibility)
}

// Stop implements queue.Worker.
func (q *Queue) Stop() {
	close(q.quit)
	q.wg.Wait()
	fmt.Println("Queue (redis) stopped")
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
	ticker := time.NewTicker(q.visibility / 2)
	if q.visibility/2 < time.Second {
		ticker.Reset(time.Second)
	}
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
	now := float64(time.Now().UnixMilli())
	members, err := q.client.ZRangeByScore(ctx, q.inflightKey(), &goredis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	if err == nil {
		for _, m := range members {
			n, err := q.client.ZRem(ctx, q.inflightKey(), m).Result()
			if err != nil || n == 0 {
				continue
			}
			_ = q.client.LRem(ctx, q.processingKey(), 1, m).Err()
			_ = q.client.LPush(ctx, q.readyKey(), m).Err()
			log.Printf("queue redis: reclaimed expired job\n")
		}
	}
	// Jobs stuck in processing without an inflight deadline (crash between
	// BRPOPLPUSH and ZADD) would never be reaped — recover them here.
	q.reclaimOrphans(ctx)
}

// reclaimOrphans moves processing-list entries that have no inflight score back to ready.
func (q *Queue) reclaimOrphans(ctx context.Context) {
	items, err := q.client.LRange(ctx, q.processingKey(), 0, -1).Result()
	if err != nil || len(items) == 0 {
		return
	}
	for _, m := range items {
		_, err := q.client.ZScore(ctx, q.inflightKey(), m).Result()
		if err == nil {
			continue // has a deadline — leave for score-based reclaim
		}
		if err != goredis.Nil {
			continue
		}
		if n, err := q.client.LRem(ctx, q.processingKey(), 1, m).Result(); err != nil || n == 0 {
			continue
		}
		_ = q.client.LPush(ctx, q.readyKey(), m).Err()
		log.Printf("queue redis: reclaimed orphan processing job\n")
	}
}

func (q *Queue) promoteDelayed(ctx context.Context) {
	now := float64(time.Now().UnixMilli())
	members, err := q.client.ZRangeByScore(ctx, q.delayedKey(), &goredis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	if err != nil || len(members) == 0 {
		return
	}
	for _, m := range members {
		n, err := q.client.ZRem(ctx, q.delayedKey(), m).Result()
		if err != nil || n == 0 {
			continue
		}
		_ = q.client.LPush(ctx, q.readyKey(), m).Err()
	}
}

func (q *Queue) ack(ctx context.Context, data string) {
	_ = q.client.LRem(ctx, q.processingKey(), 1, data).Err()
	_ = q.client.ZRem(ctx, q.inflightKey(), data).Err()
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
	q.ack(ctx, data) // always ack processing slot; retry uses delayed/ready

	if err != nil {
		log.Printf("Job %s attempt %d failed: %v\n", job.ID, job.Attempt, err)
		if job.Attempt <= job.MaxRetries {
			wire.Attempt++
			if e := q.enqueueDelayed(wire, q.retryWait); e != nil {
				log.Printf("Job %s requeue failed: %v\n", job.ID, e)
				if q.onDead != nil {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("queue redis onDead panic: %v\n", r)
						}
					}()
					q.onDead(job, err)
				}
			}
			return
		}
		if q.onDead != nil {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("queue redis onDead panic: %v\n", r)
				}
			}()
			q.onDead(job, err)
		}
	}
}

var (
	_ queue.Queue  = (*Queue)(nil)
	_ queue.Worker = (*Queue)(nil)
)
