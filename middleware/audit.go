package middleware

import (
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// AuditEvent is emitted after a request finishes (success or handler error path).
type AuditEvent struct {
	Method    string
	Path      string
	RoutePath string
	Status    int
	Duration  time.Duration
	TenantID  string
	RequestID string
	ClientIP  string
	UserAgent string
	// Actor is an optional principal (e.g. subject from JWT); filled by ActorFunc when set.
	Actor string
	Err   error
}

// AuditConfig configures the audit hook middleware.
type AuditConfig struct {
	// OnAudit receives the event after the handler chain returns. Must not panic.
	OnAudit func(e AuditEvent)
	// ActorFunc extracts a principal string (optional).
	ActorFunc func(c *context.Context) string
	// Skipper skips audit when true.
	Skipper func(c *context.Context) bool
}

// Audit calls OnAudit after each request with method/path/status/tenant/request id.
func Audit(cfg AuditConfig) func(c *context.Context) error {
	if cfg.OnAudit == nil {
		panic("middleware: Audit OnAudit is required")
	}

	return func(c *context.Context) error {
		if cfg.Skipper != nil && cfg.Skipper(c) {
			c.Next()
			return nil
		}
		start := time.Now()
		c.Next()

		status := 200
		if c.StatusCode() > 0 {
			status = c.StatusCode()
		}
		actor := ""
		if cfg.ActorFunc != nil {
			actor = cfg.ActorFunc(c)
		}
		reqID := ""
		if v, ok := c.Get(RequestIDKey); ok {
			if s, ok := v.(string); ok {
				reqID = s
			}
		}
		path := ""
		ua := ""
		if c.Request != nil {
			if c.Request.URL != nil {
				path = c.Request.URL.Path
			}
			ua = c.Request.UserAgent()
		}
		cfg.OnAudit(AuditEvent{
			Method:    c.Request.Method,
			Path:      path,
			RoutePath: c.RoutePath,
			Status:    status,
			Duration:  time.Since(start),
			TenantID:  TenantID(c),
			RequestID: reqID,
			ClientIP:  c.ClientIP(),
			UserAgent: ua,
			Actor:     actor,
		})
		return nil
	}
}
