package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/joho/godotenv"

	"transaction-service/internal/config"
	"transaction-service/internal/db"
	"transaction-service/internal/grpcapi"
	transactionsv1 "transaction-service/proto"

	"google.golang.org/grpc"
)

func main() {
	// Load .env if present (non-fatal if missing)
	_ = godotenv.Load()

	// Load config (reads DATABASE_URL, GRPC_ADDR, etc. from env)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Init DB (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	// Repo + gRPC service
	repo := db.NewTransactionRepo(pool)
	svc := grpcapi.NewServer(repo, cfg.IPRatePerMinute, cfg.UserRatePerMinute)

	// Start listener
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
	}

	// gRPC server
	gs := grpc.NewServer()
	transactionsv1.RegisterTransactionsServer(gs, svc)

	log.Printf("gRPC listening on %s", cfg.GRPCAddr)
	if err := gs.Serve(lis); err != nil {
		log.Println("grpc server exited with error:", err)
		os.Exit(1)
	}
}
