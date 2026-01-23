package middleware

import (
	"github.com/go-kvolt/kvolt/context"
)

// Config for CORS
type CORSConfig struct {
	AllowOrigins string // "*" or specific
	AllowMethods string
	AllowHeaders string
}

func CORS() func(c *context.Context) error {
	// Default Config
	cfg := CORSConfig{
		AllowOrigins: "*",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}

	return func(c *context.Context) error {
		c.Writer.Header().Set("Access-Control-Allow-Origin", cfg.AllowOrigins)
		c.Writer.Header().Set("Access-Control-Allow-Methods", cfg.AllowMethods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", cfg.AllowHeaders)

		if c.Request.Method == "OPTIONS" {
			c.Status(204) // No Content
			return nil
		}

		c.Next()
		return nil
	}
}
