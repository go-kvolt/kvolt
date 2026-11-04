package main

import (
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

func main() {
	addr := env("ADDR", "127.0.0.1:18082")
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})
	log.Fatal(e.Start(addr))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
