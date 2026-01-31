package queue

import "time"

// Job represents a unit of work.
type Job struct {
	ID        string
	Name      string
	Payload   interface{}
	CreatedAt time.Time
}

// HandlerFunc is the function that processes a job.
type HandlerFunc func(job Job) error

// Queue is the interface for dispatching jobs.
type Queue interface {
	// Push adds a job to the queue.
	Push(name string, payload interface{}) error
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
