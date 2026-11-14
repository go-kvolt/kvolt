package queue

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryQueue is an in-memory implementation using Go Channels.
type MemoryQueue struct {
	jobChan     chan Job
	handlers    map[string]HandlerFunc
	workerCount int
	quit        chan bool
	stopOnce    sync.Once
	wg          sync.WaitGroup
	mu          sync.RWMutex
	onDead      DeadLetterFunc
	retryWait   time.Duration
}

// MemoryQueueOption configures MemoryQueue.
type MemoryQueueOption func(*MemoryQueue)

// WithDeadLetter sets a callback for jobs that fail after all retries.
func WithDeadLetter(fn DeadLetterFunc) MemoryQueueOption {
	return func(q *MemoryQueue) { q.onDead = fn }
}

// WithRetryWait sets delay before re-queueing a failed job (default 100ms).
func WithRetryWait(d time.Duration) MemoryQueueOption {
	return func(q *MemoryQueue) {
		if d > 0 {
			q.retryWait = d
		}
	}
}

// NewMemoryQueue creates a new in-memory queue.
// bufferSize: How many jobs can be queued before Push returns "queue is full".
// workerCount: How many concurrent workers to run.
func NewMemoryQueue(bufferSize int, workerCount int, opts ...MemoryQueueOption) *MemoryQueue {
	q := &MemoryQueue{
		jobChan:     make(chan Job, bufferSize),
		handlers:    make(map[string]HandlerFunc),
		workerCount: workerCount,
		quit:        make(chan bool),
		retryWait:   100 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

// Push adds a job to the queue (Thread-Safe).
func (q *MemoryQueue) Push(name string, payload interface{}) error {
	return q.PushWithOptions(name, payload, PushOptions{})
}

// PushWithOptions adds a job with retry settings.
func (q *MemoryQueue) PushWithOptions(name string, payload interface{}, opts PushOptions) error {
	job := Job{
		ID:         uuid.New().String(),
		Name:       name,
		Payload:    payload,
		CreatedAt:  time.Now(),
		Attempt:    1,
		MaxRetries: opts.MaxRetries,
	}
	return q.enqueue(job)
}

func (q *MemoryQueue) enqueue(job Job) error {
	select {
	case <-q.quit:
		return fmt.Errorf("queue is stopped")
	default:
	}
	select {
	case <-q.quit:
		return fmt.Errorf("queue is stopped")
	case q.jobChan <- job:
		return nil
	default:
		return fmt.Errorf("queue is full")
	}
}

// Register adds a handler.
func (q *MemoryQueue) Register(name string, handler HandlerFunc) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[name] = handler
}

// Start spawns the workers.
func (q *MemoryQueue) Start() {
	for i := 0; i < q.workerCount; i++ {
		q.wg.Add(1)
		go q.workerLoop(i)
	}
	fmt.Printf("🚀 Queue started with %d workers\n", q.workerCount)
}

// Stop gracefully shuts down workers.
func (q *MemoryQueue) Stop() {
	q.stopOnce.Do(func() {
		close(q.quit)
		q.wg.Wait()
		fmt.Println("🛑 Queue stopped")
	})
}

func (q *MemoryQueue) workerLoop(id int) {
	defer q.wg.Done()
	for {
		select {
		case job := <-q.jobChan:
			q.process(job)
		case <-q.quit:
			return
		}
	}
}

func (q *MemoryQueue) process(job Job) {
	q.mu.RLock()
	handler, exists := q.handlers[job.Name]
	q.mu.RUnlock()

	if !exists {
		log.Printf("⚠️ No handler for job: %s\n", job.Name)
		return
	}

	if err := handler(job); err != nil {
		log.Printf("❌ Job %s attempt %d failed: %v\n", job.ID, job.Attempt, err)
		if job.Attempt <= job.MaxRetries {
			next := job
			next.Attempt++
			time.AfterFunc(q.retryWait, func() {
				if e := q.enqueue(next); e != nil {
					log.Printf("❌ Job %s requeue failed: %v\n", job.ID, e)
					if q.onDead != nil {
						q.onDead(job, err)
					}
				}
			})
			return
		}
		if q.onDead != nil {
			q.onDead(job, err)
		}
		return
	}
}
