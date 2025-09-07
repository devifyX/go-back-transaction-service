package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool
}

func NewPool(ctx context.Context, url string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	// Basic pool configuration
	cfg.MaxConns = 8
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 10 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Verify DB connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &Pool{Pool: pool}, nil
}
