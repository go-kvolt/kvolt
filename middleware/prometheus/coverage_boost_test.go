package prometheus

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestMiddleware_StatusZeroDefaults200(t *testing.T) {
	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{
			Middleware(),
			func(c *context.Context) error { return nil }, // no write → status 0
		}
		c.Next()
	}
	req := httptest.NewRequest(http.MethodGet, "/zero", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}
