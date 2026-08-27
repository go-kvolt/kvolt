package context

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestBindJSON_ConcurrentDistinctBodies(t *testing.T) {
	const n = 200
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			party := fmt.Sprintf("party-%d", i)
			body := fmt.Sprintf(`{"party":%q,"amount_paise":%d}`, party, int64(i))
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/", strings.NewReader(body))
			c := New(w, r)
			var req struct {
				Party       string `json:"party"`
				AmountPaise int64  `json:"amount_paise"`
			}
			if err := c.BindJSON(&req); err != nil {
				errCh <- err
				return
			}
			if req.Party != party || req.AmountPaise != int64(i) {
				errCh <- fmt.Errorf("aliased or wrong bind: got %+v want party=%s amount=%d", req, party, i)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestJSON_PooledEncodeConcurrent(t *testing.T) {
	const n = 200
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			c := New(w, r)
			msg := fmt.Sprintf("id-%d", i)
			if err := c.JSON(200, map[string]string{"id": msg}); err != nil {
				errCh <- err
				return
			}
			if !strings.Contains(w.Body.String(), msg) {
				errCh <- fmt.Errorf("encode pool mixup: body=%s want %s", w.Body.String(), msg)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestSameOrigin(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.com/ws", nil)
	r.Host = "example.com"
	r.Header.Set("Origin", "http://evil.test")
	if sameOrigin(r) {
		t.Fatal("cross-origin must be rejected")
	}
	r.Header.Set("Origin", "http://example.com")
	if !sameOrigin(r) {
		t.Fatal("same origin must be allowed")
	}
	r.Header.Del("Origin")
	if !sameOrigin(r) {
		t.Fatal("missing Origin must be allowed (non-browser)")
	}
}

func FuzzBindJSON(f *testing.F) {
	f.Add(`{"party":"Meera","amount_paise":1}`)
	f.Add(`[]`)
	f.Add(`null`)
	f.Add(`{"a":`)
	f.Fuzz(func(t *testing.T, raw string) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(raw))
		c := New(w, r)
		var dest map[string]any
		_ = c.BindJSON(&dest)
	})
}

func FuzzQuery(f *testing.F) {
	f.Add("q=foo")
	f.Add("a=1&a=2")
	f.Add("")
	f.Fuzz(func(t *testing.T, rawQuery string) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/search", nil)
		r.URL.RawQuery = rawQuery
		c := New(w, r)
		_ = c.Query("q")
		_ = c.Query("a")
	})
}
