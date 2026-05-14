package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"backend/internal/middleware"
	"backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Handler handles all API requests and holds dependencies like the database pool
type Handler struct {
	DB *pgxpool.Pool
}

// NewHandler creates a new Handler instance
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

// --- Helper Functions ---

func (h *Handler) generateTokens(userUUID string, role string) (string, string, error) {
	jwtKey := middleware.GetJWTKey()

	// Access Token: 15 minutes
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

	// Refresh Token: 1 day
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

// --- Public Handlers ---

// GetEquipment godoc
// @Summary      查詢設備詳情
// @Description  透過 asset_code 取得特定設備的資訊
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        asset_code   query     string  true  "設備資產編號"
// @Success      200  {object}  models.EquipmentPublicResponse "成功回傳設備資訊"
// @Failure      400  {object}  models.ErrorResponse "資產編號必填"
// @Failure      404  {object}  models.ErrorResponse "查詢不到設備"
// @Router       /api/public/equipment [get]
func (h *Handler) GetEquipment(c *gin.Context) {
	assetCode := c.Query("asset_code")
	if assetCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset_code is required"})
		return
	}

	var equipment struct {
		LID             string `json:"lid"`
		AssetCode       string `json:"asset_code"`
		Name            string `json:"name"`
		Category        string `json:"category"`
		Status          string `json:"status"`
		HasActiveReport bool   `json:"has_active_report"`
	}

	// Optimized query using EXISTS for better performance
	query := `
		SELECT
			e.lid, e.asset_code, e.name, e.category, e.status,
			EXISTS (
				SELECT 1 FROM maintenance_records m
				WHERE m.equipment_id = e.lid AND m.is_resolved = false
			) AS has_active_report
		FROM equipments e
		WHERE e.asset_code = $1`

	err := h.DB.QueryRow(c.Request.Context(), query, assetCode).Scan(
		&equipment.LID, &equipment.AssetCode, &equipment.Name,
		&equipment.Category, &equipment.Status, &equipment.HasActiveReport,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Equipment not found"})
		} else {
			slog.Error("Database query error", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	c.JSON(http.StatusOK, equipment)
}

// PostMaintenanceRecord godoc
// @Summary      提交報修紀錄
// @Description  建立一個新的報修紀錄，並將設備狀態更新為 faulty。會檢查是否已有未處理的紀錄。
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        request  body      models.MaintenanceRequest  true  "報修資訊"
// @Success      200      {object}  models.MessageResponse "成功提交報修紀錄"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤或已有未處理紀錄"
// @Failure      404      {object}  models.ErrorResponse "設備不存在"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Router       /api/public/report [post]
func (h *Handler) PostMaintenanceRecord(c *gin.Context) {
	var req models.MaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	tx, err := h.DB.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	// Check if equipment exists and has active reports
	var exists bool
	var hasActive bool
	checkQuery := `
		SELECT
			true,
			EXISTS(SELECT 1 FROM maintenance_records WHERE equipment_id = $1 AND is_resolved = false)
		FROM equipments WHERE lid = $1`

	err = tx.QueryRow(c.Request.Context(), checkQuery, req.EquipmentID).Scan(&exists, &hasActive)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Equipment not found"})
		} else if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "22P02" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format for equipment_id"})
		} else {
			slog.Error("Database query error", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	if hasActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "An unresolved record already exists"})
		return
	}

	// Insert record
	insertQuery := `
		INSERT INTO maintenance_records (equipment_id, reporter_type, description, is_resolved, resolve_note, created_at)
		VALUES ($1, $2, $3, false, '', $4)`
	_, err = tx.Exec(c.Request.Context(), insertQuery, req.EquipmentID, req.ReporterType, req.Description, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create record"})
		return
	}

	// Update status
	_, err = tx.Exec(c.Request.Context(), "UPDATE equipments SET status = 'faulty' WHERE lid = $1", req.EquipmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update equipment status"})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Maintenance record submitted successfully"})
}

// CreateUser godoc
// @Summary      新增使用者
// @Description  新增一個使用者 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      models.CreateUserRequest  true  "新增使用者請求"
// @Success      201      {object}  models.MessageResponse "成功新增使用者"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      409      {object}  models.ErrorResponse "使用者名稱已存在"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/user [post]
func (h *Handler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	query := `INSERT INTO users (username, password_hash, name, role) VALUES ($1, $2, $3, $4)`
	_, err = h.DB.Exec(c.Request.Context(), query, req.Username, string(hashedPassword), req.Name, req.Role)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		} else {
			slog.Error("Create user failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Insert failed"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

// UpdateUser godoc
// @Summary      修改使用者資料
// @Description  修改使用者資料 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      models.UpdateUserRequest  true  "修改使用者請求"
// @Success      200      {object}  models.MessageResponse "成功修改使用者"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      404      {object}  models.ErrorResponse "使用者不存在"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/user [patch]
func (h *Handler) UpdateUser(c *gin.Context) {
	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 構建動態 SQL
	query := "UPDATE users SET "
	args := []interface{}{}
	argCount := 1

	if req.Name != nil {
		query += "name = $" + fmt.Sprint(argCount) + ", "
		args = append(args, *req.Name)
		argCount++
	}
	if req.Role != nil {
		query += "role = $" + fmt.Sprint(argCount) + ", "
		args = append(args, *req.Role)
		argCount++
	}
	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		query += "password_hash = $" + fmt.Sprint(argCount) + ", "
		args = append(args, string(hashedPassword))
		argCount++
	}

	if argCount == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	// 移除最後的逗號與空白
	query = query[:len(query)-2]
	query += " WHERE lid = $" + fmt.Sprint(argCount)
	args = append(args, req.LID)

	result, err := h.DB.Exec(c.Request.Context(), query, args...)
	if err != nil {
		slog.Error("Update user failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// DeleteUser godoc
// @Summary      刪除使用者
// @Description  刪除使用者 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      models.DeleteUserRequest  true  "刪除使用者請求"
// @Success      200      {object}  models.MessageResponse "成功刪除使用者"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      404      {object}  models.ErrorResponse "使用者不存在"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/user [delete]
func (h *Handler) DeleteUser(c *gin.Context) {
	var req models.DeleteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LID required"})
		return
	}

	result, err := h.DB.Exec(c.Request.Context(), "DELETE FROM users WHERE lid = $1", req.LID)
	if err != nil {
		slog.Error("Delete user failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// GetUsers godoc
// @Summary      查看使用者列表
// @Description  取得所有使用者資料 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Success      200      {array}   models.UserResponse "使用者列表"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/users [get]
func (h *Handler) GetUsers(c *gin.Context) {
	query := `SELECT lid, username, name, role FROM users`
	rows, err := h.DB.Query(c.Request.Context(), query)
	if err != nil {
		slog.Error("Query users failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}
	defer rows.Close()

	var users []models.UserResponse
	for rows.Next() {
		var u models.UserResponse
		if err := rows.Scan(&u.LID, &u.Username, &u.Name, &u.Role); err != nil {
			slog.Error("Scan user failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan failed"})
			return
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		slog.Error("Rows error", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Rows iteration error"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// --- Auth Handlers ---

// AuthenticateUser godoc
// @Summary      使用者登入
// @Description  驗證使用者名稱與密碼，並回傳 Access Token 與 Refresh Token。
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      models.LoginRequest  true  "登入資訊"
// @Success      200      {object}  models.TokenResponse "登入成功"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      401      {object}  models.ErrorResponse "帳號或密碼錯誤"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Router       /api/auth/login [post]
func (h *Handler) AuthenticateUser(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	var stored struct {
		passwordHash string
		userUUID     string
		userRole     string
	}

	query := `SELECT password_hash, lid, role FROM users WHERE username = $1`
	err := h.DB.QueryRow(c.Request.Context(), query, req.Username).Scan(&stored.passwordHash, &stored.userUUID, &stored.userRole)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored.passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	accessToken, refreshToken, err := h.generateTokens(stored.userUUID, stored.userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// RefreshTokenHandler godoc
// @Summary      刷新 Access Token
// @Description  使用 Refresh Token 換取新的 Access Token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      models.RefreshRequest  true  "Refresh Token"
// @Success      200      {object}  models.TokenResponse "刷新成功"
// @Failure      401      {object}  models.ErrorResponse "Token 無效或過期"
// @Router       /api/auth/refresh [post]
func (h *Handler) RefreshTokenHandler(c *gin.Context) {
	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return middleware.GetJWTKey(), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token type"})
		return
	}

	userUUID := claims["userUUID"].(string)
	var role string
	err = h.DB.QueryRow(c.Request.Context(), "SELECT role FROM users WHERE lid = $1", userUUID).Scan(&role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User lookup failed"})
		return
	}

	accessToken, _, _ := h.generateTokens(userUUID, role)
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken})
}

// --- Private Handlers (Staff/Admin) ---

// GetDetailEquipment godoc
// @Summary      獲取所有設備詳情
// @Description  回傳資料庫中所有設備的完整資訊 (僅限管理員與維修人員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Success      200  {array}   models.EquipmentDetail
// @Failure      500  {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/equipments [get]
func (h *Handler) GetDetailEquipment(c *gin.Context) {
	query := `SELECT lid, asset_code, name, category, last_maint_date, maint_interval, status, location FROM equipments`
	rows, err := h.DB.Query(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}
	defer rows.Close()

	var list []models.EquipmentDetail
	for rows.Next() {
		var e models.EquipmentDetail
		var lastMaint time.Time // 使用 time.Time 來接收資料庫的 DATE 型別

		err := rows.Scan(
			&e.LID, &e.AssetCode, &e.Name, &e.Category,
			&lastMaint, &e.MaintInterval, &e.Status, &e.Location,
		)

		if err != nil {
			slog.Error("Scan equipment failed", "err", err)
			continue // 或是回傳 500，視你的需求而定
		}

		// 格式化日期為字串
		e.LastMaintDate = lastMaint.Format("2006-01-02")
		list = append(list, e)
	}

	// 檢查迴圈結束後是否有錯誤
	if err := rows.Err(); err != nil {
		slog.Error("Rows iteration error", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database cursor error"})
		return
	}

	c.JSON(http.StatusOK, list)
}

// PostEquipment godoc
// @Summary      新增設備
// @Description  建立一個新的設備紀錄 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      models.CreateEquipmentRequest  true  "新增設備請求"
// @Success      201      {object}  models.MessageResponse "成功建立設備"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/equipment [post]
func (h *Handler) PostEquipment(c *gin.Context) {
	var req models.CreateEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Default last maintenance date to today if not provided
	if req.LastMaintDate == "" {
		req.LastMaintDate = time.Now().Format("2006-01-02")
	}

	// Status is always "normal" for new equipment
	status := "normal"

	query := `INSERT INTO equipments (asset_code, name, category, last_maint_date, maint_interval, status, location) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := h.DB.Exec(c.Request.Context(), query, req.AssetCode, req.Name, req.Category, req.LastMaintDate, req.MaintInterval, status, req.Location)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Insert failed"})
		return
	}
	c.JSON(http.StatusCreated, models.MessageResponse{Message: "Equipment created"})
}

// UpdateEquipment godoc
// @Summary      修改設備
// @Description  修改一個設備紀錄 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      models.UpdateEquipmentRequest  true  "修改設備請求"
// @Success      200      {object}  models.MessageResponse "成功修改設備"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/equipment [patch]
func (h *Handler) UpdateEquipment(c *gin.Context) {
	var req models.UpdateEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := "UPDATE equipments SET "
	args := []interface{}{}
	argCount := 1

	if req.AssetCode != nil {
		query += "asset_code = $" + fmt.Sprint(argCount) + ", "
		args = append(args, *req.AssetCode)
		argCount++
	}
	if req.Name != nil {
		query += "name = $" + fmt.Sprint(argCount) + ", "
		args = append(args, *req.Name)
		argCount++
	}
	if req.Category != nil {
		query += "category = $" + fmt.Sprint(argCount) + ", "
		args = append(args, *req.Category)
		argCount++
	}
	if req.MaintInterval != nil {
		query += "maint_interval = $" + fmt.Sprint(argCount) + ", "
		args = append(args, *req.MaintInterval)
		argCount++
	}
	if req.Location != nil {
		query += "location = $" + fmt.Sprint(argCount) + ", "
		args = append(args, *req.Location)
		argCount++
	}

	if argCount == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	query = query[:len(query)-2]
	query += " WHERE lid = $" + fmt.Sprint(argCount)
	args = append(args, req.LID)

	result, err := h.DB.Exec(c.Request.Context(), query, args...)
	if err != nil {
		slog.Error("Update equipment failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Equipment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Equipment updated successfully"})
}

// DeleteEquipment godoc
// @Summary      刪除設備
// @Description  刪除一個設備紀錄 (僅限管理員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      models.DeleteEquipmentRequest  true  "刪除設備請求"
// @Success      200      {object}  models.MessageResponse "成功刪除設備"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/equipment [delete]
func (h *Handler) DeleteEquipment(c *gin.Context) {
	var req models.DeleteEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LID required"})
		return
	}

	// 使用交易確保資料一致性
	tx, err := h.DB.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	// 1. 先刪除該設備的所有相關維修紀錄
	_, err = tx.Exec(c.Request.Context(), "DELETE FROM maintenance_records WHERE equipment_id = $1", req.LID)
	if err != nil {
		slog.Error("Delete associated records failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete related maintenance records"})
		return
	}

	// 2. 再刪除設備本身
	result, err := tx.Exec(c.Request.Context(), "DELETE FROM equipments WHERE lid = $1", req.LID)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "22P02" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format for lid"})
		} else {
			slog.Error("Delete equipment failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		}
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Equipment not found"})
		return
	}

	// 提交交易
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Equipment and associated records deleted successfully"})
}

// GetMaintenanceRecords godoc
// @Summary      取得維修紀錄列表
// @Description  取得所有維修紀錄 (僅限管理員與維修人員)
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        resolved  query     string  false  "是否顯示已解決 (true/false)"
// @Success      200  {array}   models.MaintenanceRecord
// @Failure      500  {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/maintenance-records [get]
func (h *Handler) GetMaintenanceRecords(c *gin.Context) {
	resolved := c.Query("resolved")
	query := `
		SELECT
			m.lid, m.equipment_id, e.name, e.asset_code,
			m.reporter_type, m.description, m.is_resolved, m.resolve_note, m.created_at
		FROM maintenance_records m
		JOIN equipments e ON m.equipment_id = e.lid`

	var args []interface{}
	if resolved != "" {
		isResolved := resolved == "true"
		query += " WHERE m.is_resolved = $1"
		args = append(args, isResolved)
	}

	query += " ORDER BY m.created_at DESC"

	rows, err := h.DB.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}
	defer rows.Close()

	records := []models.MaintenanceRecord{}
	for rows.Next() {
		var r models.MaintenanceRecord
		rows.Scan(&r.LID, &r.EquipmentID, &r.EquipmentName, &r.AssetCode, &r.ReporterType, &r.Description, &r.IsResolved, &r.ResolveNote, &r.CreatedAt)
		records = append(records, r)
	}
	c.JSON(http.StatusOK, records)
}

// ResolveMaintenanceRecord godoc
// @Summary      標記維修完成
// @Description  將維修紀錄標記為已解決，並將設備狀態恢復為 normal (僅限管理員與維修人員)。註：lid是維修紀錄列表的lid
// @Tags         private
// @Accept       json
// @Produce      json
// @Param        request  body      models.ResolveMaintenanceRequest  true  "維修完成請求"
// @Success      200      {object}  models.MessageResponse "成功修復"
// @Failure      400      {object}  models.ErrorResponse "請求格式錯誤"
// @Failure      404      {object}  models.ErrorResponse "紀錄不存在"
// @Failure      500      {object}  models.ErrorResponse "伺服器內部錯誤"
// @Security     BearerAuth
// @Router       /api/private/maintenance-records/resolve [patch]
func (h *Handler) ResolveMaintenanceRecord(c *gin.Context) {
	var req models.ResolveMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tx, err := h.DB.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction start failed"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	var equipID string
	err = tx.QueryRow(c.Request.Context(), "UPDATE maintenance_records SET is_resolved = true, resolve_note = $1 WHERE lid = $2 RETURNING equipment_id", req.ResolveNote, req.LID).Scan(&equipID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		} else if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "22P02" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format for record lid"})
		} else {
			slog.Error("Resolve record error", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Resolve failed"})
		}
		return
	}

	_, err = tx.Exec(c.Request.Context(), "UPDATE equipments SET status = 'normal', last_maint_date = CURRENT_DATE WHERE lid = $1", equipID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update equipment status"})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Record resolved"})
}
