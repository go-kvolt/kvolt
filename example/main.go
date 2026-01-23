package main

import (
	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/middleware"
)

func main() {
	app := kvolt.New()
	app.Use(middleware.Logger())

	app.GET("/", func(c *context.Context) error {
		return c.JSON(200, map[string]string{
			"message": "Hello from KVolt!",
		})
	})

	app.GET("/ping", func(c *context.Context) error {
		return c.String(200, "pong")
	})

	app.Run(":8080")
}
