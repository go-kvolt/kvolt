package main

import (
	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
)

func main() {
	app := kvolt.Default()
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
