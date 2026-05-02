package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	// 重要：這裡要引入你專案生成的 docs 資料夾，假設你的 module 名稱是 backend
	_ "backend/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var dbPool *pgxpool.Pool
var jwtKey []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default_secret_key_for_development"
		slog.Warn("JWT_SECRET 未設定，使用預設金鑰 (僅供開發使用)")
	}
	jwtKey = []byte(secret)
}

func InitDB() *pgxpool.Pool {
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbHost := os.Getenv("DB_HOST")

	connString := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", dbUser, dbPass, dbHost, dbName)

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

type ResolveMaintenanceRequest struct {
	LID         int    `json:"lid" binding:"required" example:"1"`
	ResolveNote string `json:"resolve_note" binding:"required" example:"更換零件完成"`
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

	// 開始交易
	tx, err := dbPool.Begin(context.Background())
	if err != nil {
		slog.Error("開始交易失敗", "err", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback(context.Background())

	// 檢查設備是否存在
	var equipmentExists bool
	checkEquipQuery := `SELECT EXISTS(SELECT 1 FROM equipments WHERE lid = $1)`
	err = tx.QueryRow(context.Background(), checkEquipQuery, req.EquipmentID).Scan(&equipmentExists)
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
	err = tx.QueryRow(context.Background(), checkQuery, req.EquipmentID).Scan(&exists)
	if err != nil {
		slog.Error("檢查現有紀錄失敗", "err", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	if exists {
		c.JSON(400, gin.H{"error": "An unresolved record already exists"})
		return
	}

	// 1. 插入報修紀錄
	insertQuery := `
		INSERT INTO maintenance_records (equipment_id, reporter_type, description, is_resolved, resolve_note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.Exec(context.Background(), insertQuery, req.EquipmentID, req.ReporterType, req.Description, false, "", time.Now())
	if err != nil {
		slog.Error("寫入報修紀錄失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to create maintenance record"})
		return
	}

	// 2. 更新設備狀態為 'faulty'
	updateQuery := `UPDATE equipments SET status = 'faulty' WHERE lid = $1`
	_, err = tx.Exec(context.Background(), updateQuery, req.EquipmentID)
	if err != nil {
		slog.Error("更新設備狀態失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to update equipment status"})
		return
	}

	// 提交交易
	if err := tx.Commit(context.Background()); err != nil {
		slog.Error("提交交易失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(200, gin.H{"message": "Maintenance record submitted successfully and equipment status updated to faulty"})
}

// GenerateTokens 產生雙 JWT：Access Token (15m) 與 Refresh Token (1d)
func GenerateTokens(userUUID string, role string) (string, string, error) {
	// Access Token: 15 分鐘
	accessTokenClaims := jwt.MapClaims{
		"userUUID": userUUID,
		"role":     role,
		"exp":      time.Now().Add(time.Minute * 15).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtKey)
	if err != nil {
		return "", "", err
	}

	// Refresh Token: 1 天
	refreshTokenClaims := jwt.MapClaims{
		"userUUID": userUUID,
		"type":     "refresh",
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString(jwtKey)
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}

// LoginRequest represents the data structure for user authentication
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"admin"`
}

// AuthenticateUser godoc
// @Summary      使用者登入
// @Description  驗證使用者名稱與密碼，並回傳 Access Token 與 Refresh Token。
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      LoginRequest  true  "登入資訊"
// @Success      200      {object}  map[string]interface{} "登入成功"
// @Failure      400      {object}  map[string]interface{} "請求格式錯誤"
// @Failure      401      {object}  map[string]interface{} "帳號或密碼錯誤"
// @Failure      500      {object}  map[string]interface{} "伺服器內部錯誤"
// @Router       /api/auth/login [post]
func AuthenticateUser(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	var stored struct {
		passwordHash string
		userUUID     string
		userRole     string
	}

	query := `SELECT password_hash, lid, role FROM users WHERE username = $1`
	err := dbPool.QueryRow(context.Background(), query, req.Username).Scan(&stored.passwordHash, &stored.userUUID, &stored.userRole)

	if err != nil {
		slog.Warn("登入失敗", "username", req.Username, "err", err)
		c.JSON(401, gin.H{"error": "Invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored.passwordHash), []byte(req.Password)); err != nil {
		slog.Warn("密碼驗證失敗", "username", req.Username)
		c.JSON(401, gin.H{"error": "Invalid username or password"})
		return
	}

	accessToken, refreshToken, err := GenerateTokens(stored.userUUID, stored.userRole)
	if err != nil {
		slog.Error("產生 JWT 失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(200, gin.H{
		"message":       "Login successful",
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// RefreshRequest 代表刷新 Token 的請求
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenHandler godoc
// @Summary      刷新 Access Token
// @Description  使用有效的 Refresh Token 換取新的 Access Token (此操作不會更換 Refresh Token)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      RefreshRequest  true  "Refresh Token"
// @Success      200      {object}  map[string]interface{} "刷新成功，回傳新的 access_token"
// @Failure      401      {object}  map[string]interface{} "Token 無效、已過期或權限錯誤"
// @Router       /api/auth/refresh [post]
func RefreshTokenHandler(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	// 解析並驗證 Refresh Token
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		c.JSON(401, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(401, gin.H{"error": "Invalid token claims"})
		return
	}

	// 確認 Token 類型是否為 refresh
	if claims["type"] != "refresh" {
		c.JSON(401, gin.H{"error": "Invalid token type"})
		return
	}

	userUUID, ok := claims["userUUID"].(string)
	if !ok {
		c.JSON(401, gin.H{"error": "Invalid user identity in token"})
		return
	}

	// 從資料庫確認該使用者最新的 role
	var role string
	query := `SELECT role FROM users WHERE lid = $1`
	err = dbPool.QueryRow(context.Background(), query, userUUID).Scan(&role)
	if err != nil {
		slog.Warn("Token 刷新失敗：找不到使用者", "userUUID", userUUID, "err", err)
		c.JSON(401, gin.H{"error": "User not found or inactive"})
		return
	}

	// 僅產生新的 Access Token (15 分鐘)
	accessTokenClaims := jwt.MapClaims{
		"userUUID": userUUID,
		"role":     role,
		"exp":      time.Now().Add(time.Minute * 15).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtKey)
	if err != nil {
		slog.Error("產生新的 Access Token 失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to generate access token"})
		return
	}

	c.JSON(200, gin.H{
		"access_token": accessTokenString,
	})
}

// AuthMiddleware 驗證 Access Token 的中間層
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// 預期格式: "Bearer <token>"
		var tokenString string
		fmt.Sscanf(authHeader, "Bearer %s", &tokenString)

		if tokenString == "" {
			c.JSON(401, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(401, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// 將 userUUID 與 role 存入 Context，方便後續 Handler 使用
		c.Set("userUUID", claims["userUUID"])
		c.Set("role", claims["role"])

		c.Next()
	}
}

// RoleRequired 檢查角色權限的中間層
func RoleRequired(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(403, gin.H{"error": "Role not found in context"})
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.JSON(403, gin.H{"error": "Invalid role type"})
			c.Abort()
			return
		}

		var isAllowed bool
		isAllowed = slices.Contains(allowedRoles, roleStr)

		if !isAllowed {
			c.JSON(403, gin.H{"error": "Permission denied: insufficient role"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// EquipmentDetail 代表設備的詳細資訊
type EquipmentDetail struct {
	LID           string  `json:"lid"`
	AssetCode     string  `json:"asset_code"`
	Name          string  `json:"name"`
	Category      *string `json:"category"`
	LastMaintDate string  `json:"last_maint_date"`
	MaintInterval int     `json:"maint_interval"`
	Status        string  `json:"status"`
	Location      *string `json:"location"`
}

type CreateEquipmentRequest struct {
	AssetCode     string  `json:"asset_code" binding:"required" example:"tr-002"`
	Name          string  `json:"name" binding:"required" example:"跑步機-002"`
	Category      *string `json:"category" example:"有氧"`
	LastMaintDate string  `json:"last_maint_date" binding:"required" example:"2026-05-02"`
	MaintInterval int     `json:"maint_interval" binding:"required" example:"14"`
	Status        string  `json:"status" example:"normal"`
	Location      *string `json:"location"`
}

type DeleteEquipmentRequest struct {
	LID string `json:"lid" binding:"required"`
}

// GetDetailEquipment godoc
// @Summary      獲取所有設備詳情
// @Description  回傳資料庫中所有設備的完整資訊 (僅限管理員與維修人員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Success      200  {array}   EquipmentDetail
// @Failure      500  {object}  map[string]interface{} "資料庫查詢失敗"
// @Security     BearerAuth
// @Router       /api/private/equipments [get]
func GetDetailEquipment(c *gin.Context) {
	query := `SELECT lid, asset_code, name, category, last_maint_date, maint_interval, status, location FROM equipments`
	rows, err := dbPool.Query(context.Background(), query)
	if err != nil {
		slog.Error("查詢所有設備失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to fetch equipments"})
		return
	}
	defer rows.Close()

	equipments := []EquipmentDetail{}
	for rows.Next() {
		var e EquipmentDetail
		var lastMaint time.Time
		err := rows.Scan(
			&e.LID,
			&e.AssetCode,
			&e.Name,
			&e.Category,
			&lastMaint,
			&e.MaintInterval,
			&e.Status,
			&e.Location,
		)
		if err != nil {
			slog.Error("解析設備資料失敗", "err", err)
			c.JSON(500, gin.H{"error": "Failed to parse equipment data"})
			return
		}
		e.LastMaintDate = lastMaint.Format("2006-01-02")
		equipments = append(equipments, e)
	}

	if err := rows.Err(); err != nil {
		slog.Error("讀取設備列表過程中發生錯誤", "err", err)
		c.JSON(500, gin.H{"error": "Error during row iteration"})
		return
	}

	c.JSON(200, gin.H{
		"message": "Equipment details retrieved successfully",
		"data":    equipments,
	})
}

// PostEquipment godoc
// @Summary      新增設備
// @Description  建立一個新的設備紀錄 (僅限管理員與維修人員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      CreateEquipmentRequest  true  "新增設備請求"
// @Success      201      {object}  map[string]string       "message: Equipment created successfully"
// @Failure      400      {object}  map[string]string       "error: Invalid request payload"
// @Failure      500      {object}  map[string]string       "error: Failed to create equipment"
// @Security     BearerAuth
// @Router       /private/equipments [post]
func PostEquipment(c *gin.Context) {
	var req CreateEquipmentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	if req.Status == "" {
		req.Status = "normal"
	}

	query := `
		INSERT INTO equipments (asset_code, name, category, last_maint_date, maint_interval, status, location)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := dbPool.Exec(context.Background(), query,
		req.AssetCode, req.Name, req.Category, req.LastMaintDate, req.MaintInterval, req.Status, req.Location)

	if err != nil {
		slog.Error("新增設備失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to create equipment"})
		return
	}

	c.JSON(201, gin.H{"message": "Equipment created successfully"})
}

// DeleteEquipment godoc
// @Summary      刪除設備
// @Description  刪除一個設備紀錄及其相關維修紀錄 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      DeleteEquipmentRequest  true  "刪除設備請求"
// @Success      200      {object}  map[string]string       "message: Equipment and associated records deleted successfully"
// @Failure      400      {object}  map[string]string       "error: Invalid request payload"
// @Failure      404      {object}  map[string]string       "error: Equipment not found"
// @Failure      500      {object}  map[string]string       "error: Database error"
// @Security     BearerAuth
// @Router       /api/private/equipment [delete]
func DeleteEquipment(c *gin.Context) {
	var req DeleteEquipmentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	// Start a transaction because there might be foreign key constraints (maintenance_records)
	tx, err := dbPool.Begin(context.Background())
	if err != nil {
		slog.Error("開始交易失敗", "err", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback(context.Background())

	// 1. Delete associated maintenance records first
	_, err = tx.Exec(context.Background(), "DELETE FROM maintenance_records WHERE equipment_id = $1", req.LID)
	if err != nil {
		slog.Error("刪除相關維修紀錄失敗", "lid", req.LID, "err", err)
		c.JSON(500, gin.H{"error": "Failed to delete associated maintenance records"})
		return
	}

	// 2. Delete the equipment
	result, err := tx.Exec(context.Background(), "DELETE FROM equipments WHERE lid = $1", req.LID)
	if err != nil {
		slog.Error("刪除設備失敗", "lid", req.LID, "err", err)
		c.JSON(500, gin.H{"error": "Failed to delete equipment"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(404, gin.H{"error": "Equipment not found"})
		return
	}

	if err := tx.Commit(context.Background()); err != nil {
		slog.Error("提交交易失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(200, gin.H{"message": "Equipment and associated records deleted successfully"})
}

type MaintenanceRecord struct {
	LID           int       `json:"lid"`
	EquipmentID   string    `json:"equipment_id"`
	EquipmentName string    `json:"equipment_name"`
	AssetCode     string    `json:"asset_code"`
	ReporterType  string    `json:"reporter_type"`
	Description   string    `json:"description"`
	IsResolved    bool      `json:"is_resolved"`
	ResolveNote   *string   `json:"resolve_note"`
	CreatedAt     time.Time `json:"created_at"`
}

// GetMaintenanceRecords godoc
// @Summary      取得維修紀錄列表
// @Description  取得所有維修紀錄，可透過 show_resolved 參數決定是否包含已處理的項目
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        show_resolved  query     bool  false  "是否顯示已處理的紀錄 (true/false)"
// @Success      200  {object}  map[string]interface{} "成功回傳維修紀錄列表"
// @Failure      500  {object}  map[string]interface{} "資料庫查詢失敗"
// @Security     BearerAuth
// @Router       /api/private/maintenance-records [get]
func GetMaintenanceRecords(c *gin.Context) {
	showResolved := c.Query("show_resolved") == "true"

	query := `
		SELECT
			m.lid, m.equipment_id, e.name, e.asset_code, m.reporter_type,
			m.description, m.is_resolved, m.resolve_note, m.created_at
		FROM
			maintenance_records m
		JOIN
			equipments e ON m.equipment_id = e.lid
	`

	if !showResolved {
		query += " WHERE m.is_resolved = false"
	}

	query += " ORDER BY m.created_at DESC"

	rows, err := dbPool.Query(context.Background(), query)
	if err != nil {
		slog.Error("Querying maintenance records failed", "err", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	records := []MaintenanceRecord{}
	for rows.Next() {
		var r MaintenanceRecord
		err := rows.Scan(
			&r.LID, &r.EquipmentID, &r.EquipmentName, &r.AssetCode, &r.ReporterType,
			&r.Description, &r.IsResolved, &r.ResolveNote, &r.CreatedAt,
		)
		if err != nil {
			slog.Error("Scanning maintenance record failed", "err", err)
			c.JSON(500, gin.H{"error": "Internal server error"})
			return
		}
		records = append(records, r)
	}

	c.JSON(200, gin.H{
		"message": "Maintenance records retrieved successfully",
		"data":    records,
	})
}

// ResolveMaintenanceRecord godoc
// @Summary      標記維修完成
// @Description  將維修紀錄標記為已解決，並寫入維修備註
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      ResolveMaintenanceRequest  true  "維修完成請求"
// @Success      200      {object}  map[string]string          "message: Maintenance record resolved successfully"
// @Failure      400      {object}  map[string]string          "error: Invalid request payload"
// @Failure      404      {object}  map[string]string          "error: Maintenance record not found"
// @Failure      500      {object}  map[string]string          "error: Failed to update maintenance record"
// @Security     BearerAuth
// @Router       /private/maintenance-records/resolve [patch]
func ResolveMaintenanceRecord(c *gin.Context) {
	var req ResolveMaintenanceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	// 開始交易
	tx, err := dbPool.Begin(context.Background())
	if err != nil {
		slog.Error("開始交易失敗", "err", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback(context.Background())

	// 1. 更新維修紀錄並取得設備 ID
	var equipmentID string
	updateRecordQuery := `
		UPDATE maintenance_records
		SET is_resolved = true, resolve_note = $1
		WHERE lid = $2
		RETURNING equipment_id
	`
	err = tx.QueryRow(context.Background(), updateRecordQuery, req.ResolveNote, req.LID).Scan(&equipmentID)
	if err != nil {
		slog.Error("更新維修紀錄失敗或找不到紀錄", "err", err)
		c.JSON(404, gin.H{"error": "Maintenance record not found"})
		return
	}

	// 2. 更新設備狀態為 'normal'
	updateEquipQuery := `UPDATE equipments SET status = 'normal' WHERE lid = $1`
	_, err = tx.Exec(context.Background(), updateEquipQuery, equipmentID)
	if err != nil {
		slog.Error("更新設備狀態失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to update equipment status"})
		return
	}

	// 提交交易
	if err := tx.Commit(context.Background()); err != nil {
		slog.Error("提交交易失敗", "err", err)
		c.JSON(500, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(200, gin.H{"message": "Maintenance record resolved successfully and equipment status updated to normal"})
}

// @title           設備管理 API
// @version         1.0
// @description     這是一個整合 PostgreSQL 與 Gin 的設備管理系統。
// @host            jupiterhsu.ddns.net
// @BasePath        /
// @securityDefinitions.apiKey BearerAuth
// @in                         header
// @name                       Authorization
// @description                請輸入 "Bearer <Your_JWT_Token>"
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 建議將 pool 回傳給 main 使用
	dbPool = InitDB()
	defer dbPool.Close()

	r := gin.Default()

	// Swagger 路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(301, "/swagger/index.html")
	})

	// API 路由群組
	api := r.Group("/api")
	{
		// 公開路由
		public := api.Group("/public")
		{
			public.GET("/equipment", GetEquipment)
			public.POST("/report", PostMaintenanceRecord)
		}

		// 認證路由
		auth := api.Group("/auth")
		{
			auth.POST("/login", AuthenticateUser)
			auth.POST("/refresh", RefreshTokenHandler)
		}

		// 需要認證的私有路由
		private := api.Group("/private")
		private.Use(AuthMiddleware()) // 啟用驗證中間層
		{
			// 管理員與維修人員可以存取
			authorized := private.Group("/")
			authorized.Use(RoleRequired("admin", "staff"))
			{
				authorized.GET("/equipments", GetDetailEquipment)
				authorized.GET("/maintenance-records", GetMaintenanceRecords)
				authorized.PATCH("/maintenance-records/resolve", ResolveMaintenanceRecord)
			}
			admin := private.Group("/")
			admin.Use(RoleRequired("admin"))
			{
				admin.POST("/equipment", PostEquipment)
				admin.DELETE("/equipment", DeleteEquipment)
			}
		}
	}

	r.Run(":8080")
}
