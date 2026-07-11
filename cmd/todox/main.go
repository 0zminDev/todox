// Command todox runs the TodoX web server.
package main

import (
	"log"
	"os"

	"github.com/0zminDev/todox/internal/server"
)

func main() {
	dbPath := envOr("DB_PATH", "todos.db")
	port := envOr("PORT", "8080")

	if err := server.Run(dbPath, port); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
