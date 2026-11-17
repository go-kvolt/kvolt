package prometheus

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

// net/http accepts any token as a method. Unknown methods must share one label value,
// or a client could create an unlimited number of time series.
func TestMiddleware_UnknownMethodsShareOneLabel(t *testing.T) {
	for _, method := range []string{"FUZZA", "FUZZB", "FUZZC"} {
		c := context.New(httptest.NewRecorder(), httptest.NewRequest(method, "/label-test", nil))
		c.RoutePath = "/label-test"
		c.Handlers = []context.HandlerFunc{
			Middleware(),
			func(c *context.Context) error { return c.String(http.StatusOK, "ok") },
		}
		c.Next()
	}

	rec := httptest.NewRecorder()
	c := context.New(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	c.Handlers = []context.HandlerFunc{Handler()}
	c.Next()
	body, _ := io.ReadAll(rec.Body)
	out := string(body)

	if strings.Contains(out, "FUZZ") {
		t.Fatalf("raw method used as a label value")
	}
	want := `kvolt_http_requests_total{method="OTHER",path="/label-test",status="200"} 3`
	if !strings.Contains(out, want) {
		t.Fatalf("missing %q in scrape output", want)
	}
}

func TestMetricMethod(t *testing.T) {
	for _, m := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE"} {
		if got := metricMethod(m); got != m {
			t.Errorf("metricMethod(%q) = %q", m, got)
		}
	}
	for _, m := range []string{"", "get", "PROPFIND", "X"} {
		if got := metricMethod(m); got != "OTHER" {
			t.Errorf("metricMethod(%q) = %q, want OTHER", m, got)
		}
	}
}
