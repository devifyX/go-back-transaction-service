package main

import (
	"log"
	"os"

	"github.com/devifyX/go-back-transaction-service/internal/server"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present
	_ = godotenv.Load()

	if err := server.Run(); err != nil {
		log.Println("server exited with error:", err)
		os.Exit(1)
	}
}
