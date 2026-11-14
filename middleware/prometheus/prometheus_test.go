package prometheus

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestMiddleware_Scrape(t *testing.T) {
	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			Middleware(),
			func(c *context.Context) error { return c.String(200, "ok") },
		}
		c.Next()
	}

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	c := context.New(metricsRec, metricsReq)
	c.Handlers = []context.HandlerFunc{Handler()}
	c.Next()

	body, _ := io.ReadAll(metricsRec.Body)
	if !strings.Contains(string(body), "kvolt_http_requests_total") {
		t.Fatalf("missing counter:\n%s", body)
	}
}

func TestMiddleware_PanicDoesNotLeakInFlight(t *testing.T) {
	c := context.New(httptest.NewRecorder(), httptest.NewRequest("GET", "/panic", nil))
	c.Handlers = []context.HandlerFunc{
		Middleware(),
		func(c *context.Context) error {
			panic("panic in handler")
		},
	}
	defer func() {
		_ = recover()
	}()
	c.Next()
}
