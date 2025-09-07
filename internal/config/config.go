package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// HTTP
	Addr string // e.g. ":8080"

	// DB
	DatabaseURL string

	// Rate Limiting
	IPRatePerMinute   int
	UserRatePerMinute int

	// gRPC
	GRPCAddr string // e.g. ":9090"
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func Load() (*Config, error) {
	cfg := &Config{
		Addr:              getenv("ADDR", ":6080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		IPRatePerMinute:   getenvInt("IP_RATE_PER_MINUTE", 60),
		UserRatePerMinute: getenvInt("USER_RATE_PER_MINUTE", 120),
		GRPCAddr:          getenv("GRPC_ADDR", ":6090"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}
