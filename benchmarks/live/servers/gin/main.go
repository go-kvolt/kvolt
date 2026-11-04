package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	addr := env("ADDR", "127.0.0.1:18081")
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})
	log.Fatal(r.Run(addr))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
