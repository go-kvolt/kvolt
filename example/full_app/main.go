package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
	kvprom "github.com/go-kvolt/kvolt/v2/middleware/prometheus"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/go-kvolt/kvolt/v2/pkg/queue"
	kvqredis "github.com/go-kvolt/kvolt/v2/pkg/queue/redis"
	"github.com/go-kvolt/kvolt/v2/pkg/swagger"
	"github.com/redis/go-redis/v9"
)

type CreateItem struct {
	Name string `json:"name"`
}

type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	app := kvolt.Default()
	app.Use(middleware.Logger())
	app.Use(kvprom.Middleware())
	// Default key is ClientIP(); prefer that over RemoteAddr (includes ephemeral port).
	app.Use(middleware.Limiter(50, 100))
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		app.Use(middleware.JWTOptional(middleware.JWTConfig{SigningKey: secret}))
	} else {
		log.Println("JWTOptional skipped (set JWT_SECRET to enable)")
	}
	// JSON API demo: CSRF is for cookie/browser apps — see docs/middleware.md.

	var q queue.Queue
	var worker queue.Worker
	if addr := os.Getenv("REDIS_URL"); addr != "" {
		opt, err := redis.ParseURL(addr)
		if err != nil {
			log.Fatalf("REDIS_URL: %v", err)
		}
		rdb := redis.NewClient(opt)
		rq := kvqredis.New(rdb, 2,
			kvqredis.WithKeyPrefix("kvolt:example:"),
			kvqredis.WithDeadLetter(func(job queue.Job, err error) {
				log.Printf("dead letter %s: %v", job.ID, err)
			}),
			kvqredis.WithRetryWait(100*time.Millisecond),
			kvqredis.WithVisibilityTimeout(30*time.Second),
		)
		q, worker = rq, rq
		log.Println("using Redis queue (at-least-once)")
	} else {
		mq := queue.NewMemoryQueue(100, 2,
			queue.WithDeadLetter(func(job queue.Job, err error) {
				log.Printf("dead letter %s: %v", job.ID, err)
			}),
			queue.WithRetryWait(50*time.Millisecond),
		)
		q, worker = mq, mq
		log.Println("using memory queue (set REDIS_URL for Redis)")
	}

	worker.Register("echo", func(job queue.Job) error {
		log.Printf("job %s payload=%v attempt=%d", job.ID, job.Payload, job.Attempt)
		return nil
	})
	worker.Start()
	defer worker.Stop()

	app.GET("/", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"app": "kvolt-full-example"})
	})

	app.POST("/items", func(c *context.Context) error {
		var in CreateItem
		if err := c.BindJSON(&in); err != nil {
			return httperr.BadRequest("invalid_json", err.Error())
		}
		if in.Name == "" {
			return httperr.BadRequest("name_required", "name is required")
		}
		return c.JSON(201, Item{ID: "1", Name: in.Name})
	}).
		Desc("Create an item").
		Tags("items").
		Request(CreateItem{}).
		Response(201, Item{})

	app.POST("/jobs", func(c *context.Context) error {
		if err := q.PushWithOptions("echo", map[string]string{"msg": "hi"}, queue.PushOptions{MaxRetries: 1}); err != nil {
			return httperr.New(503, "queue_full", err.Error())
		}
		return c.JSON(202, map[string]string{"status": "queued"})
	}).Desc("Enqueue echo job").Tags("jobs")

	app.GET("/metrics", kvprom.Handler())
	app.GET("/swagger/*any", swagger.Handler(swagger.Config{
		Title:          "KVolt Full Example",
		RoutesProvider: swagger.Adapter(app),
		Host:           "localhost:8080",
	}))

	fmt.Println("listening on :8080  (docs /swagger/index.html, metrics /metrics)")
	app.Run(":8080")
}
