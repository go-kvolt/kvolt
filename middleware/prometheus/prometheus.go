// Package prometheus provides optional Prometheus metrics middleware.
// Import explicitly; core middleware does not pull client_golang.
package prometheus

import (
	"strconv"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	promRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kvolt_http_requests_total",
		Help: "Total HTTP requests processed by KVolt.",
	}, []string{"method", "path", "status"})

	promInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "kvolt_http_in_flight_requests",
		Help: "In-flight HTTP requests.",
	})

	promLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kvolt_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// metricMethod maps a request method to a bounded label value. net/http accepts any
// token as a method, so labelling by the raw value would let a client create an
// unlimited number of time series.
func metricMethod(method string) string {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		return method
	}
	return "OTHER"
}

// Middleware records request metrics in the default Prometheus registry.
func Middleware() func(c *context.Context) error {
	return func(c *context.Context) error {
		if c == nil {
			return nil
		}
		start := time.Now()
		promInFlight.Inc()
		defer func() {
			promInFlight.Dec()
			if r := recover(); r != nil {
				path := c.RoutePath
				if path == "" && c.Request != nil && c.Request.URL != nil {
					path = c.Request.URL.Path
				}
				method := ""
				if c.Request != nil {
					method = metricMethod(c.Request.Method)
				}
				promRequests.WithLabelValues(method, path, "500").Inc()
				promLatency.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
				panic(r)
			}
		}()

		c.Next()

		path := c.RoutePath
		if path == "" && c.Request != nil && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		method := ""
		if c.Request != nil {
			method = metricMethod(c.Request.Method)
		}
		status := c.StatusCode()
		if status == 0 {
			status = 200
		}
		promRequests.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
		promLatency.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
		return nil
	}
}

// Handler serves the default Prometheus registry (mount at GET /metrics).
func Handler() func(c *context.Context) error {
	h := promhttp.Handler()
	return func(c *context.Context) error {
		if c == nil || c.Writer == nil || c.Request == nil {
			return nil
		}
		h.ServeHTTP(c.Writer, c.Request)
		c.SetHeaderWritten()
		return nil
	}
}
