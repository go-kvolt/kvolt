package main

import (
	"log"
	"os"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
)

func main() {
	addr := env("ADDR", "127.0.0.1:18080")
	app := kvolt.New()
	app.GET("/ping", func(c *context.Context) error {
		return c.JSON(200, map[string]string{"message": "pong"})
	})
	log.Fatal(app.ListenAndServe(addr))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
