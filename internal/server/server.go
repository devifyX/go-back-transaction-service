package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"transaction-service/internal/config"
	"transaction-service/internal/db"
	"transaction-service/internal/graph"
	"transaction-service/internal/middleware"
	"github.com/graphql-go/handler"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	repo := db.NewTransactionRepo(pool)
	resolver := &graph.Resolver{Repo: repo}
	schema, err := graph.NewSchema(resolver)
	if err != nil {
		return err
	}

	// Enable built-in GraphiQL UI at GET /graphql
	h := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: true,
	})

	limStore := middleware.NewLimiterStore(cfg.IPRatePerMinute, cfg.UserRatePerMinute)
	mux := http.NewServeMux()

	mux.Handle("/graphql", limStore.RateLimit(h))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	log.Printf("listening on %s", cfg.Addr)
	return srv.ListenAndServe()
}
