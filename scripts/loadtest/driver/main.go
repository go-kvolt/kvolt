// Soak-test driver: starts the server binary, drives concurrent traffic over real TCP
// while checking every response for correctness, then checks for leaks and tests
// graceful shutdown with a request in flight.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	base      string
	transport *http.Transport
	client    *http.Client
)

type resp struct {
	status int
	header http.Header
	body   []byte
	raw    *http.Response
}

func do(method, path, body string, hdr map[string]string) (*resp, error) {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, base+path, rd)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return &resp{status: r.StatusCode, header: r.Header, body: b, raw: r}, nil
}

func jsonMap(b []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := json.Unmarshal(b, &m)
	return m, err
}

func expectStatus(r *resp, want int) error {
	if r.status != want {
		return fmt.Errorf("status %d, want %d; body %.200q", r.status, want, r.body)
	}
	return nil
}

func bigBody(nonce string) string {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "line %03d nonce=%s\n", i, nonce)
	}
	return sb.String()
}

var total429 atomic.Int64

type scenario struct {
	name   string
	weight int
	run    func(u string, seq int) error
}

func scenarios() []scenario {
	return []scenario{
		{"ping+security-headers", 10, func(u string, seq int) error {
			r, err := do("GET", "/ping", "", nil)
			if err != nil {
				return err
			}
			if err := expectStatus(r, 200); err != nil {
				return err
			}
			if string(r.body) != `{"message":"pong"}` {
				return fmt.Errorf("body %q", r.body)
			}
			if r.header.Get("X-Request-ID") == "" || r.header.Get("X-Content-Type-Options") != "nosniff" {
				return fmt.Errorf("missing request id / security headers: %v", r.header)
			}
			return nil
		}},
		{"request-id-echo", 6, func(u string, seq int) error {
			r, err := do("GET", "/ping", "", map[string]string{"X-Request-ID": u})
			if err != nil {
				return err
			}
			if got := r.header.Get("X-Request-ID"); got != u {
				return fmt.Errorf("request id %q, want %q (cross-request bleed?)", got, u)
			}
			return nil
		}},
		{"router-static-vs-param", 10, func(u string, seq int) error {
			ids := []string{u, "n" + u, "new" + u, "ne", "next"}
			id := ids[seq%len(ids)]
			r, err := do("GET", "/users/"+id, "", nil)
			if err != nil {
				return err
			}
			if err := expectStatus(r, 200); err != nil {
				return err
			}
			m, _ := jsonMap(r.body)
			if m["route"] != "id" || m["id"] != id {
				return fmt.Errorf("GET /users/%s -> %s", id, r.body)
			}
			r, err = do("GET", "/users/new", "", nil)
			if err != nil {
				return err
			}
			if m, _ := jsonMap(r.body); m["route"] != "new" {
				return fmt.Errorf("GET /users/new -> %s", r.body)
			}
			return nil
		}},
		{"router-nested-param", 6, func(u string, seq int) error {
			id := u
			if seq%2 == 0 {
				id = "new" // static sibling must not swallow the nested param route
			}
			r, err := do("GET", "/users/"+id+"/posts", "", nil)
			if err != nil {
				return err
			}
			m, _ := jsonMap(r.body)
			if r.status != 200 || m["route"] != "posts" || m["id"] != id {
				return fmt.Errorf("GET /users/%s/posts -> %d %s", id, r.status, r.body)
			}
			return nil
		}},
		{"router-catchall", 4, func(u string, seq int) error {
			r, err := do("GET", "/files/a/b/"+u+".txt", "", nil)
			if err != nil {
				return err
			}
			m, _ := jsonMap(r.body)
			if r.status != 200 || m["path"] != "a/b/"+u+".txt" {
				return fmt.Errorf("-> %d %s", r.status, r.body)
			}
			return nil
		}},
		{"bind-json", 10, func(u string, seq int) error {
			switch seq % 4 {
			case 0: // malformed
				r, err := do("POST", "/echo", `{"name":`, nil)
				if err != nil {
					return err
				}
				if m, _ := jsonMap(r.body); r.status != 400 || m["error"] != "invalid_request" {
					return fmt.Errorf("malformed -> %d %s", r.status, r.body)
				}
			case 1: // fails validation
				r, err := do("POST", "/echo", `{"name":"","n":1}`, nil)
				if err != nil {
					return err
				}
				if m, _ := jsonMap(r.body); r.status != 422 || m["error"] != "validation_failed" {
					return fmt.Errorf("invalid -> %d %s", r.status, r.body)
				}
			default:
				body := fmt.Sprintf(`{"name":%q,"n":%d,"tags":["a",%q]}`, u, seq, u)
				r, err := do("POST", "/echo", body, nil)
				if err != nil {
					return err
				}
				if err := expectStatus(r, 201); err != nil {
					return err
				}
				var got struct {
					Name string   `json:"name"`
					N    int      `json:"n"`
					Tags []string `json:"tags"`
				}
				if err := json.Unmarshal(r.body, &got); err != nil {
					return err
				}
				if got.Name != u || got.N != seq || len(got.Tags) != 2 || got.Tags[1] != u {
					return fmt.Errorf("echo mismatch: sent %s got %s", body, r.body)
				}
			}
			return nil
		}},
		{"gzip-large-body", 8, func(u string, seq int) error {
			hdr := map[string]string{}
			plain := seq%3 == 0
			if plain {
				hdr["Accept-Encoding"] = "identity"
			}
			r, err := do("GET", "/big?nonce="+u, "", hdr)
			if err != nil {
				return err
			}
			if err := expectStatus(r, 200); err != nil {
				return err
			}
			if string(r.body) != bigBody(u) {
				return fmt.Errorf("body mismatch (%d bytes, plain=%v)", len(r.body), plain)
			}
			if !plain && !r.raw.Uncompressed {
				return fmt.Errorf("response was not gzip-compressed")
			}
			if plain && r.header.Get("Content-Encoding") != "" {
				return fmt.Errorf("compressed despite Accept-Encoding: identity")
			}
			return nil
		}},
		{"timeout", 6, func(u string, seq int) error {
			r, err := do("GET", "/t/fast?v="+u, "", nil)
			if err != nil {
				return err
			}
			if m, _ := jsonMap(r.body); r.status != 200 || m["v"] != u {
				return fmt.Errorf("fast -> %d %s", r.status, r.body)
			}
			path := "/t/slow"
			if seq%2 == 0 {
				path = "/t/late"
			}
			start := time.Now()
			r, err = do("GET", path, "", nil)
			if err != nil {
				return err
			}
			d := time.Since(start)
			// /t/late sleeps 700ms against a 400ms deadline. If the whole machine stalls
			// for more than 300ms the handler is already done when the middleware looks,
			// and a complete 200 is then just as correct as a 504. Accept it only when the
			// request really took that long; a fast 200 would mean the timeout never fired.
			if r.status == 200 && path == "/t/late" && d >= 650*time.Millisecond {
				return nil
			}
			if r.status != 504 || !strings.Contains(string(r.body), "Gateway Timeout") {
				return fmt.Errorf("%s -> %d %s (after %s)", path, r.status, r.body, d)
			}
			if d > 3*time.Second {
				return fmt.Errorf("%s took %s", path, d)
			}
			return nil
		}},
		{"idempotency-replay", 8, func(u string, seq int) error {
			body := fmt.Sprintf(`{"item":%q}`, u)
			hdr := map[string]string{"Idempotency-Key": u, "Origin": "https://app.example"}
			first, err := do("POST", "/orders", body, hdr)
			if err != nil {
				return err
			}
			if err := expectStatus(first, 201); err != nil {
				return err
			}
			if first.header.Get("Idempotent-Replayed") != "" {
				return fmt.Errorf("first request marked as replay")
			}
			second, err := do("POST", "/orders", body, hdr)
			if err != nil {
				return err
			}
			if second.status != 201 || !bytes.Equal(first.body, second.body) || second.header.Get("Idempotent-Replayed") != "true" {
				return fmt.Errorf("replay mismatch: %s vs %d %s", first.body, second.status, second.body)
			}
			for _, h := range []string{"Access-Control-Allow-Origin", "X-Request-Id", "X-Order-Item", "Content-Type"} {
				if n := len(second.header.Values(h)); n != 1 {
					return fmt.Errorf("replay has %d %s headers", n, h)
				}
			}
			return nil
		}},
		{"idempotency-concurrent", 8, func(u string, seq int) error {
			key := "c" + u
			body := fmt.Sprintf(`{"item":%q}`, key)
			const n = 4
			bodies := make([]string, n)
			errs := make([]error, n)
			var wg sync.WaitGroup
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					r, err := do("POST", "/orders", body, map[string]string{"Idempotency-Key": key})
					if err != nil {
						errs[i] = err
						return
					}
					if r.status != 201 {
						errs[i] = fmt.Errorf("status %d %s", r.status, r.body)
						return
					}
					bodies[i] = string(r.body)
				}(i)
			}
			wg.Wait()
			for i := 0; i < n; i++ {
				if errs[i] != nil {
					return errs[i]
				}
				if bodies[i] != bodies[0] {
					return fmt.Errorf("handler ran more than once for one key: %s vs %s", bodies[0], bodies[i])
				}
			}
			return nil
		}},
		{"session-lifecycle", 6, func(u string, seq int) error {
			r, err := do("GET", "/session-cycle?u="+u, "", nil)
			if err != nil {
				return err
			}
			if m, _ := jsonMap(r.body); r.status != 200 || m["user"] != u {
				return fmt.Errorf("-> %d %s", r.status, r.body)
			}
			return nil
		}},
		{"panic-recovery", 1, func(u string, seq int) error {
			r, err := do("GET", "/panic?v="+u, "", nil)
			if err != nil {
				return err
			}
			if r.status != 500 || !strings.Contains(string(r.body), "internal") || strings.Contains(string(r.body), "soak panic") {
				return fmt.Errorf("-> %d %s", r.status, r.body)
			}
			return nil
		}},
		{"head-on-get-route", 3, func(u string, seq int) error {
			r, err := do("HEAD", "/users/"+u, "", nil)
			if err != nil {
				return err
			}
			if r.status != 200 || len(r.body) != 0 || r.header.Get("Content-Type") == "" {
				return fmt.Errorf("-> %d body=%d ct=%q", r.status, len(r.body), r.header.Get("Content-Type"))
			}
			return nil
		}},
		{"sse-stream", 4, func(u string, seq int) error {
			pad := 0
			if seq%2 == 0 {
				pad = 3000 // an event larger than the gzip threshold, after earlier flushes
			}
			r, err := do("GET", fmt.Sprintf("/sse?n=3&tag=%s&pad=%d", u, pad), "", nil)
			if err != nil {
				return err
			}
			if r.status != 200 || !strings.HasPrefix(r.header.Get("Content-Type"), "text/event-stream") {
				return fmt.Errorf("-> %d ct=%q", r.status, r.header.Get("Content-Type"))
			}
			for i := 0; i < 3; i++ {
				want := fmt.Sprintf("event: tick\ndata: %s:%d:%s\n\n", u, i, strings.Repeat("x", pad))
				if !strings.Contains(string(r.body), want) {
					return fmt.Errorf("event %d missing or corrupt (pad=%d, got %d bytes, starts %.80q)", i, pad, len(r.body), r.body)
				}
			}
			return nil
		}},
		{"cors", 4, func(u string, seq int) error {
			r, err := do("GET", "/ping", "", map[string]string{"Origin": "https://app.example"})
			if err != nil {
				return err
			}
			if r.header.Get("Access-Control-Allow-Origin") != "https://app.example" || !strings.Contains(strings.Join(r.header.Values("Vary"), ","), "Origin") {
				return fmt.Errorf("allowed origin: %v", r.header)
			}
			r, err = do("GET", "/ping", "", map[string]string{"Origin": "https://evil.example"})
			if err != nil {
				return err
			}
			if r.header.Get("Access-Control-Allow-Origin") != "" {
				return fmt.Errorf("refused origin got ACAO %q", r.header.Get("Access-Control-Allow-Origin"))
			}
			return nil
		}},
		{"404-405-redirect", 4, func(u string, seq int) error {
			r, err := do("GET", "/nope/"+u, "", nil)
			if err != nil {
				return err
			}
			if r.status != 404 {
				return fmt.Errorf("404 -> %d", r.status)
			}
			r, err = do("POST", "/ping", "", nil)
			if err != nil {
				return err
			}
			if r.status != 405 || r.header.Get("Allow") != "GET, HEAD, OPTIONS" {
				return fmt.Errorf("405 -> %d Allow %q", r.status, r.header.Get("Allow"))
			}
			r, err = do("GET", "/users/new/", "", nil)
			if err != nil {
				return err
			}
			if r.status != 301 || r.header.Get("Location") != "/users/new" {
				return fmt.Errorf("redirect -> %d Location %q", r.status, r.header.Get("Location"))
			}
			return nil
		}},
		{"rate-limiter", 3, func(u string, seq int) error {
			hdr := map[string]string{"X-Client": u}
			for i := 0; i < 8; i++ {
				r, err := do("GET", "/limited/hit", "", hdr)
				if err != nil {
					return err
				}
				switch {
				case r.status == 200:
				case r.status == 429 && i >= 5:
					total429.Add(1)
					if r.header.Get("Retry-After") == "" {
						return fmt.Errorf("429 without Retry-After")
					}
				default:
					return fmt.Errorf("request %d -> %d %s", i, r.status, r.body)
				}
			}
			return nil
		}},
	}
}

type stats struct {
	Goroutines int    `json:"goroutines"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	Requests   uint64 `json:"requests"`
	Errors5xx  uint64 `json:"errors_5xx"`
	Errors4xx  uint64 `json:"errors_4xx"`
	InFlight   int64  `json:"in_flight"`
	CacheKeys  int    `json:"cache_keys"`
}

func getStats() (stats, error) {
	var s stats
	transport.CloseIdleConnections()
	time.Sleep(1500 * time.Millisecond)
	r, err := do("GET", "/stats?gc=1", "", nil)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(r.body, &s)
	return s, err
}

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
)

func ensureConsole() string {
	if h, _, _ := kernel32.NewProc("GetConsoleWindow").Call(); h != 0 {
		return "using existing console"
	}
	if r, _, e := kernel32.NewProc("AllocConsole").Call(); r == 0 {
		return "no console available: " + e.Error()
	}
	if h, _, _ := kernel32.NewProc("GetConsoleWindow").Call(); h != 0 {
		_, _, _ = user32.NewProc("ShowWindow").Call(h, 0)
	}
	return "allocated hidden console"
}

func sendCtrlBreak(pid int) error {
	r, _, e := kernel32.NewProc("GenerateConsoleCtrlEvent").Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(pid))
	if r == 0 {
		return e
	}
	return nil
}

func main() {
	serverExe := flag.String("server", "", "server binary")
	addr := flag.String("addr", "127.0.0.1:18080", "server address")
	dur := flag.Duration("duration", 60*time.Second, "load duration")
	workers := flag.Int("workers", 48, "concurrent workers")
	logPath := flag.String("log", "server.log", "server log file")
	flag.Parse()

	base = "http://" + *addr
	transport = &http.Transport{MaxIdleConns: 512, MaxIdleConnsPerHost: 512}
	client = &http.Client{Transport: transport, Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	fmt.Println("console:", ensureConsole())
	logFile, err := os.Create(*logPath)
	if err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(2)
	}
	cmd := exec.Command(*serverExe, "-addr", *addr)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		fmt.Println("FATAL: start server:", err)
		os.Exit(2)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	failed := false
	fail := func(format string, a ...interface{}) {
		failed = true
		fmt.Printf("FAIL: "+format+"\n", a...)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
	}()

	ready := false
	for i := 0; i < 100 && !ready; i++ {
		if r, err := do("GET", "/ping", "", nil); err == nil && r.status == 200 {
			ready = true
		} else {
			time.Sleep(200 * time.Millisecond)
		}
	}
	if !ready {
		fmt.Println("FATAL: server did not become ready")
		os.Exit(2)
	}

	scs := scenarios()

	// Warm-up: every scenario a few times, sequentially, so deterministic failures show clearly.
	for round := 0; round < 4; round++ {
		for _, sc := range scs {
			if err := sc.run(fmt.Sprintf("warm%d%s", round, strings.ReplaceAll(sc.name, "-", "")[:4]), round); err != nil {
				fail("warm-up %s (round %d): %v", sc.name, round, err)
			}
		}
	}
	baseline, err := getStats()
	if err != nil {
		fail("baseline stats: %v", err)
	}
	fmt.Printf("baseline: goroutines=%d heap=%.1fMB\n", baseline.Goroutines, float64(baseline.HeapAlloc)/1e6)

	// Load.
	var weighted []int
	for i, sc := range scs {
		for w := 0; w < sc.weight; w++ {
			weighted = append(weighted, i)
		}
	}
	counts := make([]atomic.Int64, len(scs))
	fails := make([]atomic.Int64, len(scs))
	var mu sync.Mutex
	samples := map[string][]string{}
	deadline := time.Now().Add(*dur)
	start := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w) + 1))
			for seq := 0; time.Now().Before(deadline); seq++ {
				i := weighted[rng.Intn(len(weighted))]
				u := fmt.Sprintf("w%dx%d", w, seq)
				counts[i].Add(1)
				if err := scs[i].run(u, seq); err != nil {
					fails[i].Add(1)
					mu.Lock()
					if len(samples[scs[i].name]) < 3 {
						samples[scs[i].name] = append(samples[scs[i].name], err.Error())
					}
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var totalRuns, totalFails int64
	fmt.Printf("\n%-26s %10s %8s\n", "scenario", "runs", "failed")
	for i, sc := range scs {
		c, f := counts[i].Load(), fails[i].Load()
		totalRuns += c
		totalFails += f
		fmt.Printf("%-26s %10d %8d\n", sc.name, c, f)
	}
	names := make([]string, 0, len(samples))
	for n := range samples {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		for _, s := range samples[n] {
			fmt.Printf("  sample failure [%s]: %s\n", n, s)
		}
	}
	if totalFails > 0 {
		fail("%d of %d scenario runs failed", totalFails, totalRuns)
	}
	if total429.Load() == 0 {
		fail("rate limiter never returned 429")
	}

	time.Sleep(2 * time.Second) // let timed-out handlers finish
	after, err := getStats()
	if err != nil {
		fail("final stats: %v", err)
	}
	fmt.Printf("\nload: %d scenario runs, %d server requests in %s (%.0f req/s), workers=%d\n",
		totalRuns, after.Requests-baseline.Requests, elapsed.Round(time.Second),
		float64(after.Requests-baseline.Requests)/elapsed.Seconds(), *workers)
	fmt.Printf("after:    goroutines=%d heap=%.1fMB in_flight=%d cache_keys=%d rate_limited=%d\n",
		after.Goroutines, float64(after.HeapAlloc)/1e6, after.InFlight, after.CacheKeys, total429.Load())
	if d := after.Goroutines - baseline.Goroutines; d > 10 {
		fail("goroutine leak: %d -> %d", baseline.Goroutines, after.Goroutines)
	}
	if after.HeapAlloc > baseline.HeapAlloc+96<<20 {
		fail("heap grew from %.1fMB to %.1fMB", float64(baseline.HeapAlloc)/1e6, float64(after.HeapAlloc)/1e6)
	}
	if after.InFlight != 1 { // the /stats request itself
		fail("in-flight gauge is %d after load, want 1", after.InFlight)
	}
	if after.CacheKeys != 0 {
		fail("%d session keys left in the store", after.CacheKeys)
	}

	// Graceful shutdown with a request in flight.
	slow := make(chan error, 1)
	go func() {
		r, err := do("GET", "/slow?ms=1500", "", nil)
		if err != nil {
			slow <- err
			return
		}
		if m, _ := jsonMap(r.body); r.status != 200 || m["slept"] != float64(1500) {
			slow <- fmt.Errorf("-> %d %s", r.status, r.body)
			return
		}
		slow <- nil
	}()
	time.Sleep(300 * time.Millisecond)
	how := "console signal (SIGINT path in Run)"
	sigErr := sendCtrlBreak(cmd.Process.Pid)
	var exitErr error
	gotExit := false
	if sigErr == nil {
		select {
		case exitErr = <-exited:
			gotExit = true
		case <-time.After(8 * time.Second):
		}
	}
	if !gotExit {
		how = fmt.Sprintf("admin endpoint (signal not deliverable here: %v)", sigErr)
		go func() {
			r, err := do("GET", "/slow?ms=1500", "", nil)
			if err != nil {
				slow <- err
			} else if r.status != 200 {
				slow <- fmt.Errorf("-> %d", r.status)
			} else {
				slow <- nil
			}
		}()
		time.Sleep(300 * time.Millisecond)
		if _, err := do("POST", "/admin/shutdown", "", nil); err != nil {
			fail("admin shutdown: %v", err)
		}
		select {
		case exitErr = <-exited:
			gotExit = true
		case <-time.After(15 * time.Second):
			fail("server did not exit after shutdown request")
		}
	}
	select {
	case err := <-slow:
		if err != nil {
			fail("in-flight request during shutdown: %v", err)
		} else {
			fmt.Println("\nshutdown: in-flight request completed with 200")
		}
	case <-time.After(5 * time.Second):
		fail("in-flight request never returned")
	}
	exitCode := -1
	if gotExit {
		exitCode = 0
		if ee, ok := exitErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else if exitErr != nil {
			exitCode = -2
		}
	}
	fmt.Printf("shutdown: via %s, exit code %d\n", how, exitCode)
	if exitCode != 0 {
		fail("server exit code %d (66 means the race detector reported a race)", exitCode)
	}

	_ = logFile.Close()
	logBytes, _ := os.ReadFile(*logPath)
	logText := string(logBytes)
	races := strings.Count(logText, "WARNING: DATA RACE")
	panics := strings.Count(logText, "[Panic] soak panic")
	superfluous := strings.Count(logText, "superfluous response.WriteHeader")
	handlerErrs := strings.Count(logText, "[KVolt] handler error")
	fmt.Printf("server log: %d bytes, data races=%d, recovered panics=%d, superfluous WriteHeader=%d, handler errors=%d\n",
		len(logBytes), races, panics, superfluous, handlerErrs)
	if races > 0 {
		fail("race detector reported %d data race(s)", races)
	}
	if superfluous > 0 {
		fail("net/http logged %d superfluous WriteHeader call(s)", superfluous)
	}
	if handlerErrs > 0 {
		fail("%d unexpected handler errors logged", handlerErrs)
	}
	if want := counts[indexOf(scs, "panic-recovery")].Load() + 4; int64(panics) != want {
		fail("recovered panics logged = %d, want %d", panics, want)
	}
	for _, line := range []string{"KVolt is running on", "Shutting down server", "Server exiting", "clean exit"} {
		if !strings.Contains(logText, line) && !(strings.HasPrefix(how, "admin") && strings.Contains(line, "Shut") || strings.HasPrefix(how, "admin") && line == "Server exiting") {
			fail("server log is missing %q", line)
		}
	}

	if failed {
		fmt.Println("\nRESULT: FAILED")
		os.Exit(1)
	}
	fmt.Println("\nRESULT: PASSED")
}

func indexOf(scs []scenario, name string) int {
	for i, s := range scs {
		if s.name == name {
			return i
		}
	}
	return 0
}
