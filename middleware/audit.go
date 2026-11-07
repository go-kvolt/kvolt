package middleware

import (
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// AuditEvent is emitted after a request finishes (success, error, or panic recovery path).
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
	// Invoked asynchronously so it cannot block the request path.
	OnAudit func(e AuditEvent)
	// ActorFunc extracts a principal string (optional).
	ActorFunc func(c *context.Context) string
	// Skipper skips audit when true.
	Skipper func(c *context.Context) bool
	// Sync when true runs OnAudit on the request goroutine (tests).
	Sync bool
}

// Audit calls OnAudit after each request with method/path/status/tenant/request id.
// Uses defer so panicking requests are still audited when Recovery runs later in the chain.
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
		var handlerErr error
		defer func() {
			if r := recover(); r != nil {
				handlerErr = auditPanicErr(r)
				emitAudit(cfg, c, start, handlerErr)
				panic(r) // re-raise for Recovery
			}
			emitAudit(cfg, c, start, handlerErr)
		}()
		c.Next()
		return nil
	}
}

type panicError struct{ v interface{} }

func (p panicError) Error() string { return "panic" }

func auditPanicErr(r interface{}) error {
	if err, ok := r.(error); ok {
		return err
	}
	return panicError{v: r}
}

func emitAudit(cfg AuditConfig, c *context.Context, start time.Time, handlerErr error) {
	status := 200
	if c.StatusCode() > 0 {
		status = c.StatusCode()
	}
	if handlerErr != nil && status < 500 {
		status = 500
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
	method := ""
	if c.Request != nil {
		method = c.Request.Method
		if c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		ua = c.Request.UserAgent()
	}
	ev := AuditEvent{
		Method:    method,
		Path:      path,
		RoutePath: c.RoutePath,
		Status:    status,
		Duration:  time.Since(start),
		TenantID:  TenantID(c),
		RequestID: reqID,
		ClientIP:  c.ClientIP(),
		UserAgent: ua,
		Actor:     actor,
		Err:       handlerErr,
	}
	if cfg.Sync {
		cfg.OnAudit(ev)
		return
	}
	go func() {
		defer func() { _ = recover() }()
		cfg.OnAudit(ev)
	}()
}
