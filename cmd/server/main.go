package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"transaction-service/internal/server"
)

func main() {
	// Load .env file if present
	_ = godotenv.Load()

	if err := server.Run(); err != nil {
		log.Println("server exited with error:", err)
		os.Exit(1)
	}
}
