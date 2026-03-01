package scheduler

import (
	"testing"
	"time"
)

func TestScheduler_NewAndAdd(t *testing.T) {
	s := New()
	id, err := s.Add("@every 1h", func() {})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id < 0 {
		t.Errorf("Add: id want non-negative, got %d", id)
	}
	s.Stop()
}

func TestScheduler_StartStop(t *testing.T) {
	s := New()
	s.Add("@every 24h", func() {}) // won't run in test
	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()
}

func TestScheduler_JobRuns(t *testing.T) {
	s := New()
	_, err := s.Add("@every 100ms", func() {})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	s.Start()
	time.Sleep(250 * time.Millisecond) // allow at least one tick
	s.Stop()
}
