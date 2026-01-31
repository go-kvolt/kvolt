package scheduler

import (
	"log"

	"github.com/robfig/cron/v3"
)

// Scheduler wraps the cron library to provide a simple interface.
type Scheduler struct {
	c *cron.Cron
}

// New creates a new Scheduler.
func New() *Scheduler {
	return &Scheduler{
		c: cron.New(),
	}
}

// Add schedules a function to run at a specific interval/cron exp.
// Syntax:
// - Standard: "0 0 * * *" (Daily at midnight)
// - Interval: "@every 1h30m"
func (s *Scheduler) Add(spec string, job func()) (int, error) {
	id, err := s.c.AddFunc(spec, job)
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// Start starts the scheduler in a background goroutine.
func (s *Scheduler) Start() {
	s.c.Start()
	log.Println("⏰ Scheduler started")
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.c.Stop()
	log.Println("🛑 Scheduler stopped")
}
