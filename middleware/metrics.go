package middleware

import (
	"sync/atomic"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

// MetricsSnapshot is a point-in-time view of request counters.
type MetricsSnapshot struct {
	Requests   uint64 `json:"requests"`
	Errors5xx  uint64 `json:"errors_5xx"`
	Errors4xx  uint64 `json:"errors_4xx"`
	InFlight   int64  `json:"in_flight"`
	TotalNanos uint64 `json:"total_latency_ns"`
}

// MetricsStore holds process-local counters. Safe for concurrent use.
type MetricsStore struct {
	requests   atomic.Uint64
	errors5xx  atomic.Uint64
	errors4xx  atomic.Uint64
	inFlight   atomic.Int64
	totalNanos atomic.Uint64
}

// Snapshot returns current counters.
func (m *MetricsStore) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		Requests:   m.requests.Load(),
		Errors5xx:  m.errors5xx.Load(),
		Errors4xx:  m.errors4xx.Load(),
		InFlight:   m.inFlight.Load(),
		TotalNanos: m.totalNanos.Load(),
	}
}

// AvgLatency returns average latency since process start (0 if no requests).
func (m *MetricsStore) AvgLatency() time.Duration {
	n := m.requests.Load()
	if n == 0 {
		return 0
	}
	// #nosec G115 -- safe duration conversion from nanoseconds
	return time.Duration(m.totalNanos.Load() / n)
}

// MetricsConfig configures the metrics middleware.
type MetricsConfig struct {
	Store *MetricsStore
	// OnRequest is called after each request with status and duration (optional hook for OTel/Prometheus).
	OnRequest func(method, path string, status int, d time.Duration)
}

// Metrics records basic in-process request metrics. Pass a shared store to expose /metrics JSON yourself.
func Metrics(store *MetricsStore) func(c *context.Context) error {
	return MetricsWithConfig(MetricsConfig{Store: store})
}

// MetricsWithConfig records metrics and optionally invokes OnRequest.
func MetricsWithConfig(cfg MetricsConfig) func(c *context.Context) error {
	if cfg.Store == nil {
		cfg.Store = &MetricsStore{}
	}
	return func(c *context.Context) error {
		if c == nil {
			return nil
		}
		start := time.Now()
		cfg.Store.inFlight.Add(1)
		defer func() {
			cfg.Store.inFlight.Add(-1)
			d := time.Since(start)
			cfg.Store.requests.Add(1)
			// #nosec G115 -- duration nanoseconds is positive
			cfg.Store.totalNanos.Add(uint64(d.Nanoseconds()))
			status := c.StatusCode()
			if status >= 500 {
				cfg.Store.errors5xx.Add(1)
			} else if status >= 400 {
				cfg.Store.errors4xx.Add(1)
			}
			if cfg.OnRequest != nil {
				path := c.RoutePath
				if path == "" && c.Request != nil && c.Request.URL != nil {
					path = c.Request.URL.Path
				}
				method := ""
				if c.Request != nil {
					method = c.Request.Method
				}
				cfg.OnRequest(method, path, status, d)
			}
		}()
		c.Next()
		return nil
	}
}
