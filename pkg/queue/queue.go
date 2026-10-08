package queue

import "time"

// Job represents a unit of work.
type Job struct {
	ID        string
	Name      string
	Payload   interface{}
	CreatedAt time.Time
	// Attempt is 1-based for the current try.
	Attempt int
	// MaxRetries is how many times to retry after the first failure (0 = no retry).
	MaxRetries int
}

// HandlerFunc is the function that processes a job.
type HandlerFunc func(job Job) error

// PushOptions customizes Push behavior.
type PushOptions struct {
	// MaxRetries is retries after the first failure. Default 0.
	MaxRetries int
}

// Queue is the interface for dispatching jobs.
type Queue interface {
	// Push adds a job to the queue.
	Push(name string, payload interface{}) error
	// PushWithOptions adds a job with retry settings.
	PushWithOptions(name string, payload interface{}, opts PushOptions) error
}

// Worker is the interface for processing jobs.
type Worker interface {
	// Register adds a handler for a specific job name.
	Register(name string, handler HandlerFunc)

	// Start starts the worker (non-blocking).
	Start()

	// Stop stops the worker gracefully.
	Stop()
}

// DeadLetterFunc is called when a job exhausts retries.
type DeadLetterFunc func(job Job, err error)
