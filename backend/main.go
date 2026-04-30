package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	// 重要：這裡要引入你專案生成的 docs 資料夾，假設你的 module 名稱是 backend
	_ "backend/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var dbPool *pgxpool.Pool

func InitDB() *pgxpool.Pool {
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	connString := fmt.Sprintf("postgres://%s:%s@db:5432/%s?sslmode=disable", dbUser, dbPass, dbName)

	// 建立連線池
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		slog.Error("無法建立連線池", "err", err)
		os.Exit(1)
	}

	// 測試連線
	if err := pool.Ping(context.Background()); err != nil {
		slog.Error("資料庫連線失敗", "err", err)
		os.Exit(1)
	}

	slog.Info("成功連線至 PostgreSQL!")
	return pool // 回傳 pool，不要在這裡 defer Close
}

// GetEquipment godoc
// @Summary      查詢設備詳情
// @Description  透過 asset_code 取得特定設備的資訊
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        asset_code   query     string  true  "設備資產編號"
// @Success      200  {object}  map[string]interface{} "成功回傳設備資訊"
// @Failure      400  {object}  map[string]interface{} "資產編號必填"
// @Failure      404  {object}  map[string]interface{} "查詢不到設備"
// @Router       /api/public/equipment [get]
func GetEquipment(c *gin.Context) {
	assetCode := c.Query("asset_code")
	if assetCode == "" {
		c.JSON(400, gin.H{"error": "asset_code is required"})
		return
	}

	var equipment struct {
		LID             string      `json:"lid"`
		AssetCode       interface{} `json:"asset_code"`
		Name            string      `json:"name"`
		Category        string      `json:"category"`
		Status          string      `json:"status"`
		HasActiveReport bool        `json:"has_active_report"`
	}

	query := `
		SELECT
			e.lid,
			e.asset_code,
			e.name,
			e.category,
			e.status,
			COALESCE(bool_or(NOT m.is_resolved), false) AS has_active_report
		FROM
			equipments e
		LEFT JOIN
			maintenance_records m ON e.lid = m.equipment_id
		WHERE
			e.asset_code = $1
		GROUP BY
			e.lid;
	`

	err := dbPool.QueryRow(context.Background(), query, assetCode).Scan(
		&equipment.LID,
		&equipment.AssetCode,
		&equipment.Name,
		&equipment.Category,
		&equipment.Status,
		&equipment.HasActiveReport,
	)

	if err != nil {
		slog.Error("查詢設備失敗", "asset_code", assetCode, "err", err)
		c.JSON(404, gin.H{"error": "Equipment not found or database error"})
		return
	}

	c.JSON(200, gin.H{
		"message": "Equipment details found",
		"data":    equipment,
	})
}

// MaintenanceRequest represents the data structure for a maintenance report
type MaintenanceRequest struct {
	EquipmentID  string `json:"equipment_id" binding:"required" example:"6f17afc0-1759-45ba-9081-da035eaeea60"`
	ReporterType string `json:"reporter_type" binding:"required" example:"public"`
	Description  string `json:"description" binding:"required" example:"有異音"`
}

// PostMaintenanceRecord godoc
// @Summary      Post maintenance record
// @Description  Create a new maintenance record for a specific equipment. Checks if equipment exists and has no unresolved reports.
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        request  body      MaintenanceRequest  true  "Maintenance Information"
// @Success      200      {object}  map[string]interface{} "Maintenance record submitted successfully"
// @Failure      400      {object}  map[string]interface{} "Invalid parameters or existing unresolved record"
// @Failure      404      {object}  map[string]interface{} "Equipment not found"
// @Failure      500      {object}  map[string]interface{} "Database error"
// @Router       /api/public/report [post]
func PostMaintenanceRecord(c *gin.Context) {
	var req MaintenanceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	// 檢查設備是否存在
	var equipmentExists bool
	checkEquipQuery := `SELECT EXISTS(SELECT 1 FROM equipments WHERE lid = $1)`
	err := dbPool.QueryRow(context.Background(), checkEquipQuery, req.EquipmentID).Scan(&equipmentExists)
	if err != nil {
		slog.Error("檢查設備是否存在失敗", "err", err)
		c.JSON(500, gin.H{"error": "please check id format"})
		return
	}
	if !equipmentExists {
		c.JSON(404, gin.H{"error": "equipment not found"})
		return
	}

	// 預先檢查是否有紀錄在 maintenance_records 並且 is_resolved 為 false
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM maintenance_records WHERE equipment_id = $1 AND is_resolved = false)`
	err = dbPool.QueryRow(context.Background(), checkQuery, req.EquipmentID).Scan(&exists)
	if err != nil {
		slog.Error("檢查現有紀錄失敗", "err", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	if exists {
		c.JSON(400, gin.H{"error": "An unresolved record already exists"})
		return
	}

	query := `
		INSERT INTO maintenance_records (equipment_id,reporter_type,description,is_resolved,resolve_note,created_at) values ($1,$2,$3,$4,$5,$6);
	`
	_, err = dbPool.Exec(context.Background(), query, req.EquipmentID, req.ReporterType, req.Description, false, "", time.Now())
	if err != nil {
		slog.Error("寫入報修紀錄失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to create maintenance record"})
		return
	}

	c.JSON(200, gin.H{"message": "Maintenance record submitted successfully"})
}

// @title           設備管理 API
// @version         1.0
// @description     這是一個整合 PostgreSQL 與 Gin 的設備管理系統。
// @host            jupiterhsu.ddns.net
// @BasePath        /
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 注意：你的 InitDB 裡面有 defer dbpool.Close()，這會導致 main 還在跑但連線就斷了
	// 建議將 pool 回傳給 main 使用
	dbPool = InitDB()
	defer dbPool.Close()

	r := gin.Default()

	// Swagger 路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API 路由
	r.GET("/public/equipment", GetEquipment)
	r.POST("/public/report", PostMaintenanceRecord)

	r.Run(":8080")
}
