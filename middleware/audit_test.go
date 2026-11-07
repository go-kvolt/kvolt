package middleware

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestAudit_EmitsEvent(t *testing.T) {
	var (
		mu sync.Mutex
		ev AuditEvent
	)
	mw := Audit(AuditConfig{
		Sync: true,
		OnAudit: func(e AuditEvent) {
			mu.Lock()
			ev = e
			mu.Unlock()
		},
		ActorFunc: func(c *context.Context) string { return "user-1" },
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/items", nil)
	r.RemoteAddr = "203.0.113.9:1"
	c := context.New(w, r)
	c.Set(TenantIDKey, "acme")
	c.Handlers = []context.HandlerFunc{
		func(c *context.Context) error { return c.JSON(201, map[string]string{"ok": "1"}) },
	}
	_ = mw(c)

	mu.Lock()
	defer mu.Unlock()
	if ev.Method != "POST" || ev.Path != "/items" || ev.TenantID != "acme" || ev.Actor != "user-1" {
		t.Fatalf("event=%+v", ev)
	}
	if ev.Status != 201 {
		t.Fatalf("status=%d", ev.Status)
	}
	if ev.Duration <= 0 {
		t.Fatal("expected duration")
	}
}
