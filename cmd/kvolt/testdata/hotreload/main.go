package main

import (
	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
)

const version = "v1"

func main() {
	app := kvolt.New()
	app.GET("/version", func(c *context.Context) error {
		return c.String(200, version)
	})
	app.Run(":19876")
}
