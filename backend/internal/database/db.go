package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InitDB initializes the PostgreSQL connection pool using environment variables.
func InitDB() *pgxpool.Pool {
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbHost := os.Getenv("DB_HOST")

	if dbHost == "" {
		dbHost = "db" // Default to 'db' for docker-compose environment
	}

	connString := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", dbUser, dbPass, dbHost, dbName)

	// Create connection pool
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		slog.Error("無法建立資料庫連線池", "err", err)
		os.Exit(1)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		slog.Error("資料庫連線失敗 (Ping 失敗)", "err", err)
		os.Exit(1)
	}

	slog.Info("成功連線至 PostgreSQL!")
	return pool
}
