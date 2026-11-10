package scheduler

import "testing"

func TestAdd_InvalidSpec(t *testing.T) {
	s := New()
	if _, err := s.Add("not a cron", func() {}); err == nil {
		t.Fatal("want error for invalid cron spec")
	}
}
