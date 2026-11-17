package kvolt

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
)

// A handler that outlives its deadline keeps running while the Engine reuses the
// request Context for other requests. It must keep seeing its own request's data, and
// the later requests must be unaffected by what it does. Run with -race.
func TestTimeout_LateHandlerIsIsolatedFromPooledContext(t *testing.T) {
	app := New()
	app.Use(middleware.RequestID(), middleware.Gzip())
	// Only the slow route gets a short deadline. The fast routes get a generous one: on a
	// busy machine (race detector, parallel packages) a tight deadline would fail them.
	slowGroup := app.Group("/slow")
	slowGroup.Use(middleware.Timeout(25 * time.Millisecond))
	fastGroup := app.Group("/fast")
	fastGroup.Use(middleware.Timeout(10 * time.Second))

	var mu sync.Mutex
	lateSaw := map[string]string{} // id the handler started with -> id it saw after waking
	var late sync.WaitGroup

	slowGroup.GET("/:id", func(c *context.Context) error {
		id := c.Param("id")
		late.Add(1)
		defer late.Done()
		time.Sleep(120 * time.Millisecond) // ignores the deadline
		mu.Lock()
		lateSaw[id] = c.Param("id") + "|" + c.Request.URL.Path
		mu.Unlock()
		c.Set("poison", id)
		c.Writer.Header().Set("X-Poison", id)
		return c.String(http.StatusOK, "late-"+id)
	})
	fastGroup.GET("/:id", func(c *context.Context) error {
		if v, ok := c.Get("poison"); ok {
			return c.String(http.StatusInternalServerError, "leaked key: "+v.(string))
		}
		return c.String(http.StatusOK, "fast-"+c.Param("id"))
	})

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Accept-Encoding", "gzip")
		app.ServeHTTP(rec, r)
		return rec
	}

	// Time out a few requests, then keep the pool busy while their handlers are still running.
	for _, id := range []string{"a", "b", "c"} {
		rec := get("/slow/" + id)
		if rec.Code != http.StatusGatewayTimeout || !strings.Contains(rec.Body.String(), "Gateway Timeout") {
			t.Fatalf("/slow/%s = %d %q, want 504", id, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Poison") != "" {
			t.Fatalf("/slow/%s: late header on the 504", id)
		}
	}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				id := string(rune('p'+w)) + strings.Repeat("x", i%5)
				rec := get("/fast/" + id)
				if rec.Code != http.StatusOK || rec.Body.String() != "fast-"+id || rec.Header().Get("X-Poison") != "" {
					t.Errorf("/fast/%s = %d %q poison=%q", id, rec.Code, rec.Body.String(), rec.Header().Get("X-Poison"))
					return
				}
			}
		}(w)
	}
	wg.Wait()
	late.Wait()

	mu.Lock()
	defer mu.Unlock()
	for _, id := range []string{"a", "b", "c"} {
		if want := id + "|/slow/" + id; lateSaw[id] != want {
			t.Fatalf("late handler for %q saw %q, want %q (its Context was reused)", id, lateSaw[id], want)
		}
	}
}
