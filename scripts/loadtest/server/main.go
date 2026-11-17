// Soak-test server: a KVolt app with a realistic middleware stack, started the way
// a real service starts (Run / RunWithServer over TCP). Driven by ../driver.
package main

import (
	stdctx "context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
	"github.com/go-kvolt/kvolt/v2/pkg/cache"
	"github.com/go-kvolt/kvolt/v2/pkg/session"
)

type echoReq struct {
	Name string   `json:"name" validate:"required"`
	N    int      `json:"n"`
	Tags []string `json:"tags"`
}

func randHex() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// BigBody is the body /big returns for a nonce; the driver builds the same string.
func BigBody(nonce string) string {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "line %03d nonce=%s\n", i, nonce)
	}
	return sb.String()
}

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "listen address")
	flag.Parse()

	log.SetOutput(os.Stderr)
	kvolt.SetLogFunc(func(msg string) { log.Println("[kvolt]", msg) })

	metrics := &middleware.MetricsStore{}
	app := kvolt.New()
	app.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.LoggerWithConfig(middleware.LoggerConfig{Format: "json", Output: io.Discard}),
		middleware.Secure(),
		middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: []string{"https://app.example"}}),
		middleware.Metrics(metrics),
		middleware.MaxBodySize(1<<20),
		middleware.Gzip(),
	)

	app.GET("/ping", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"message": "pong"})
	})

	app.GET("/users/new", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"route": "new"})
	})
	app.GET("/users/:id", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"route": "id", "id": c.Param("id")})
	})
	app.GET("/users/:id/posts", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"route": "posts", "id": c.Param("id")})
	})
	app.GET("/files/*path", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"path": c.Param("path")})
	})

	app.POST("/echo", func(c *context.Context) error {
		var req echoReq
		if err := c.Bind(&req); err != nil {
			return err
		}
		return c.JSON(201, req)
	})

	app.GET("/big", func(c *context.Context) error {
		return c.String(200, BigBody(c.Query("nonce")))
	})

	timed := app.Group("/t")
	timed.Use(middleware.Timeout(400 * time.Millisecond))
	timed.GET("/fast", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"v": c.Query("v")})
	})
	timed.GET("/slow", func(c *context.Context) error { // honours the deadline
		select {
		case <-c.Request.Context().Done():
			return nil
		case <-time.After(2 * time.Second):
			return c.JSON(200, map[string]string{"late": "yes"})
		}
	})
	timed.GET("/late", func(c *context.Context) error { // ignores the deadline, writes late
		time.Sleep(700 * time.Millisecond)
		return c.JSON(200, map[string]string{"late": "yes"})
	})

	orders := app.Group("/orders")
	orders.Use(middleware.Idempotency(middleware.IdempotencyConfig{Store: middleware.NewMemoryIdempotencyStore()}))
	orders.POST("", func(c *context.Context) error {
		var req struct {
			Item string `json:"item"`
		}
		if err := c.BindJSON(&req); err != nil {
			return err
		}
		time.Sleep(2 * time.Millisecond) // widen the window for concurrent duplicates
		c.Writer.Header().Set("X-Order-Item", req.Item)
		return c.JSON(201, map[string]string{"order": randHex(), "item": req.Item})
	})

	store := cache.NewMemoryStore(time.Minute)
	sessions := session.New(store, time.Minute).WithMaxAge(10 * time.Minute)
	app.GET("/session-cycle", func(c *context.Context) error {
		u := c.Query("u")
		token, err := sessions.Create(u)
		if err != nil {
			return err
		}
		got, err := sessions.Get(token)
		if err != nil {
			return err
		}
		if err := sessions.Destroy(token); err != nil {
			return err
		}
		if _, err := sessions.Get(token); err == nil {
			return fmt.Errorf("session readable after Destroy")
		}
		return c.JSON(200, map[string]interface{}{"user": got})
	})

	app.GET("/panic", func(c *context.Context) error {
		panic("soak panic " + c.Query("v"))
	})

	app.GET("/sse", func(c *context.Context) error {
		n, _ := strconv.Atoi(c.Query("n"))
		tag := c.Query("tag")
		if err := c.SSE(); err != nil {
			return err
		}
		pad, _ := strconv.Atoi(c.Query("pad"))
		for i := 0; i < n; i++ {
			data := tag + ":" + strconv.Itoa(i) + ":" + strings.Repeat("x", pad)
			if err := c.SSEvent(context.SSEvent{Event: "tick", Data: data}); err != nil {
				return nil
			}
		}
		return nil
	})

	app.GET("/slow", func(c *context.Context) error {
		ms, _ := strconv.Atoi(c.Query("ms"))
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return c.JSON(200, map[string]int{"slept": ms})
	})

	limited := app.Group("/limited")
	limited.Use(middleware.LimiterWithConfig(middleware.LimiterConfig{
		RPS: 5, Burst: 5, MaxKeys: 200,
		KeyFunc: func(c *context.Context) string { return c.Request.Header.Get("X-Client") },
	}))
	limited.GET("/hit", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"ok": "yes"})
	})

	app.GET("/stats", func(c *context.Context) error {
		if c.Query("gc") == "1" {
			runtime.GC()
			runtime.GC()
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		snap := metrics.Snapshot()
		return c.JSON(200, map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"heap_alloc": m.HeapAlloc,
			"heap_inuse": m.HeapInuse,
			"num_gc":     m.NumGC,
			"requests":   snap.Requests,
			"errors_5xx": snap.Errors5xx,
			"errors_4xx": snap.Errors4xx,
			"in_flight":  snap.InFlight,
			"cache_keys": store.Len(),
		})
	})

	srv := app.HTTPServer(*addr)

	// Fallback for environments where a console signal cannot be delivered.
	var viaAdmin atomic.Bool
	adminDone := make(chan struct{})
	app.POST("/admin/shutdown", func(c *context.Context) error {
		if viaAdmin.CompareAndSwap(false, true) {
			go func() {
				time.Sleep(50 * time.Millisecond)
				ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 10*time.Second)
				defer cancel()
				if err := srv.Shutdown(ctx); err != nil {
					log.Printf("admin shutdown error: %v", err)
				}
				close(adminDone)
			}()
		}
		return c.JSON(http.StatusAccepted, map[string]string{"shutdown": "started"})
	})

	err := app.RunWithServer(srv)
	if viaAdmin.Load() {
		<-adminDone
	}
	middleware.FlushLogger()
	if err != nil {
		log.Printf("run error: %v", err)
		os.Exit(1)
	}
	log.Println("clean exit")
}
