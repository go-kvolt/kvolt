package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
)

func main() {
	addr := env("ADDR", "127.0.0.1:18085")
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "pong"})
	})
	log.Fatal(app.Listen(addr))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
