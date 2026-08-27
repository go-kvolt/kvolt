package kvolt

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestEngine_ConcurrentPing(t *testing.T) {
	app := Default()
	app.GET("/ping", func(c *context.Context) error {
		return c.JSON(200, map[string]any{"ok": true})
	})

	const n = 500
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/ping", nil)
			app.ServeHTTP(w, r)
			if w.Code == 200 && strings.Contains(w.Body.String(), `"ok"`) {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != n {
		t.Fatalf("ok=%d want %d", ok.Load(), n)
	}
}

func TestEngine_ConcurrentBindJSON(t *testing.T) {
	app := Default()
	app.POST("/invoice", func(c *context.Context) error {
		var req struct {
			Party       string `json:"party"`
			AmountPaise int64  `json:"amount_paise"`
		}
		if err := c.BindJSON(&req); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{"ok": true, "party": req.Party, "amount_paise": req.AmountPaise})
	})

	const n = 200
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			party := fmt.Sprintf("p-%d", i)
			body := fmt.Sprintf(`{"party":%q,"amount_paise":%d}`, party, i)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/invoice", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			app.ServeHTTP(w, r)
			got := w.Body.String()
			if w.Code != 200 {
				errCh <- fmt.Errorf("status %d body %s", w.Code, got)
				return
			}
			if !strings.Contains(got, party) {
				errCh <- fmt.Errorf("party missing in %s", got)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestEngine_FiveParamRoute(t *testing.T) {
	app := New()
	app.GET("/a/:p1/:p2/:p3/:p4/:p5", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"p5": c.Param("p5")})
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/a/1/2/3/4/5", nil)
	app.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"p5":"5"`) {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestSoak(t *testing.T) {
	if os.Getenv("KVOLT_SOAK") == "" {
		t.Skip("set KVOLT_SOAK=1 for soak")
	}
	sec := 120
	if v := os.Getenv("KVOLT_SOAK_SEC"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			sec = n
		}
	}
	start := time.Now()
	deadline := start.Add(time.Duration(sec) * time.Second)
	app := Default()
	app.GET("/ping", func(c *context.Context) error {
		return c.JSON(200, map[string]bool{"ok": true})
	})
	app.POST("/echo", func(c *context.Context) error {
		var req map[string]any
		if err := c.BindJSON(&req); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, req)
	})

	var wg sync.WaitGroup
	var fails atomic.Int64
	var ops atomic.Int64
	stop := make(chan struct{})

	fmt.Fprintf(os.Stderr, "soak start %ds, 16 workers\n", sec)
	go func() {
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				fmt.Fprintf(os.Stderr, "soak %s elapsed, ops=%d fails=%d\n", time.Since(start).Round(time.Second), ops.Load(), fails.Load())
			}
		}
	}()

	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pingW := newSoakWriter()
			echoW := newSoakWriter()
			echoBody := `{"party":"Meera","amount_paise":1}`
			for time.Now().Before(deadline) {
				pingW.reset()
				preq := httptest.NewRequest(http.MethodGet, "/ping", nil)
				app.ServeHTTP(pingW, preq)
				if pingW.code != 200 {
					fails.Add(1)
				}
				echoW.reset()
				ereq := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(echoBody))
				ereq.Header.Set("Content-Type", "application/json")
				app.ServeHTTP(echoW, ereq)
				if echoW.code != 200 {
					fails.Add(1)
				}
				ops.Add(2)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	wait := time.Until(deadline) + 30*time.Second
	if wait < 30*time.Second {
		wait = 30 * time.Second
	}
	select {
	case <-done:
		fmt.Fprintf(os.Stderr, "soak workers stopped after %s, ops=%d fails=%d\n", time.Since(start).Round(time.Second), ops.Load(), fails.Load())
	case <-time.After(wait):
		close(stop)
		t.Fatalf("soak workers still running after deadline (elapsed=%s ops=%d)", time.Since(start).Round(time.Second), ops.Load())
	}
	close(stop)
	if fails.Load() != 0 {
		t.Fatalf("soak failures: %d", fails.Load())
	}
}

type soakWriter struct {
	hdr  http.Header
	buf  []byte
	code int
}

func newSoakWriter() *soakWriter {
	return &soakWriter{hdr: make(http.Header)}
}

func (w *soakWriter) Header() http.Header { return w.hdr }

func (w *soakWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.WriteHeader(http.StatusOK)
	}
	w.buf = append(w.buf, b...)
	return len(b), nil
}

func (w *soakWriter) WriteHeader(code int) {
	if w.code != 0 {
		return
	}
	w.code = code
}

func (w *soakWriter) reset() {
	w.code = 0
	w.buf = w.buf[:0]
	for k := range w.hdr {
		delete(w.hdr, k)
	}
}
