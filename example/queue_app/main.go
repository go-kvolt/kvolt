package main

import (
	"fmt"
	"time"

	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/pkg/queue"
)

func main() {
	app := kvolt.New()

	// 1. Initialize Queue (100 buffer, 2 workers for demo)
	q := queue.NewMemoryQueue(100, 2)

	// 2. Register Handler
	q.Register("email_task", func(job queue.Job) error {
		payload := job.Payload.(map[string]string)
		fmt.Printf("Worker: Processing email for %s [ID: %s]\n", payload["email"], job.ID)
		time.Sleep(2 * time.Second) // Simulate work
		fmt.Printf("Worker: Done %s\n", job.ID)
		return nil
	})

	// 3. Start Queue
	q.Start()
	defer q.Stop()

	// 4. API Endpoints
	app.POST("/send", func(c *context.Context) error {
		// Non-blocking dispatch
		err := q.Push("email_task", map[string]string{
			"email": "user@example.com",
		})

		if err != nil {
			return c.Status(503).String(503, "Queue Full")
		}

		return c.String(200, "Job Dispatched!")
	})

	app.Run(":8080")
}
