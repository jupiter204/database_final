package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func database_init() {
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	// 1. 連線字串格式: postgres://用戶名:密碼@主機:埠號/資料庫名稱
	connString := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", dbUser, dbPass, dbHost, dbName)

	// 2. 建立連線池 (Concurrency Friendly)
	dbpool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		slog.Error("無法連線至資料庫", "err", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	// 3. 測試連線
	err = dbpool.Ping(context.Background())
	if err != nil {
		slog.Error("連線失敗: ", "err", err)
		os.Exit(1)
	}
	slog.Info("成功連線至 PostgreSQL!")
}

func main() {
	// 使用 slog 並明確指定輸出到 os.Stdout
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	database_init()
	r := gin.Default()

	r.GET("/public/equipment", func(c *gin.Context) {
		id := c.Query("id")
		if id == "123" {
			c.JSON(200, gin.H{"message": "Equipment details", "id": id})
			slog.Debug("TODO: implementation for equipments_table check", "id", id)
			return
		}
		c.JSON(400, gin.H{"error": "id is required"})
	})

	r.Run(":8080")
}
