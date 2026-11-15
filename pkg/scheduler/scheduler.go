package scheduler

import (
	"fmt"
	"log"
	"strings"

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

func (s *Scheduler) ensureCron() {
	if s.c == nil {
		s.c = cron.New()
	}
}

// Add schedules a function to run at a specific interval/cron exp.
// Syntax:
// - Standard: "0 0 * * *" (Daily at midnight)
// - Interval: "@every 1h30m"
func (s *Scheduler) Add(spec string, job func()) (int, error) {
	if s == nil {
		return 0, fmt.Errorf("scheduler: not initialized")
	}
	spec = strings.TrimSpace(spec)
	s.ensureCron()
	if job == nil {
		return 0, fmt.Errorf("scheduler: job cannot be nil")
	}
	safeJob := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Scheduler] panic in job: %v", r)
			}
		}()
		job()
	}
	id, err := s.c.AddFunc(spec, safeJob)
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// Remove removes a scheduled job by its ID.
func (s *Scheduler) Remove(id int) {
	if s == nil || s.c == nil {
		return
	}
	s.c.Remove(cron.EntryID(id))
}

// Start starts the scheduler in a background goroutine.
func (s *Scheduler) Start() {
	if s == nil {
		return
	}
	s.ensureCron()
	s.c.Start()
	log.Println("⏰ Scheduler started")
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	if s == nil || s.c == nil {
		return
	}
	s.c.Stop()
	log.Println("🛑 Scheduler stopped")
}
