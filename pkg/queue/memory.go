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
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// NewMemoryQueue creates a new in-memory queue.
// bufferSize: How many jobs can be queued before blocking (e.g., 1000).
// workerCount: How many concurrent workers to run (e.g., 10).
func NewMemoryQueue(bufferSize int, workerCount int) *MemoryQueue {
	return &MemoryQueue{
		jobChan:     make(chan Job, bufferSize),
		handlers:    make(map[string]HandlerFunc),
		workerCount: workerCount,
		quit:        make(chan bool),
	}
}

// Push adds a job to the queue (Thread-Safe).
func (q *MemoryQueue) Push(name string, payload interface{}) error {
	job := Job{
		ID:        uuid.New().String(),
		Name:      name,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	// Non-blocking push if full? Or blocking?
	// For "blazing fast", blocking is safer to apply backpressure.
	// But if we want it to never block user, we might drop or error.
	// Let's standard channel send.
	select {
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
	close(q.quit)
	q.wg.Wait()
	fmt.Println("🛑 Queue stopped")
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

	// Execute
	start := time.Now()
	if err := handler(job); err != nil {
		log.Printf("❌ Job %s failed: %v\n", job.ID, err)
	} else {
		// Log successes only in debug mode generally, but for now:
		// log.Printf("✅ Job %s completed in %v\n", job.ID, time.Since(start))
		_ = start
	}
}
