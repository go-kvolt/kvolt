package main

import (
	"log"
	"net/http"
	"os"

	"github.com/uptrace/bunrouter"
)

func main() {
	addr := env("ADDR", "127.0.0.1:18086")
	router := bunrouter.New()
	router.GET("/ping", func(w http.ResponseWriter, req bunrouter.Request) error {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"message":"pong"}`))
		return err
	})
	log.Fatal(http.ListenAndServe(addr, router))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
