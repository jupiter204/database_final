package main

import (
	"log/slog"
	"os"

	"backend/internal/database"
	"backend/internal/handlers"
	"backend/internal/middleware"

	// 重要：這裡要引入你專案生成的 docs 資料夾，假設你的 module 名稱是 backend
	_ "backend/docs"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           設備管理系統 API
// @version         1.0
// @description     這是一個整合 PostgreSQL 與 Gin 的設備管理系統。
// @host            jupiterhsu.ddns.net
// @BasePath        /
// @securityDefinitions.apiKey BearerAuth
// @in                         header
// @name                       Authorization
// @description                請輸入 "Bearer <Your_JWT_Token>"
func main() {
	// 初始化 Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 載入 .env 檔案 (本地開發用)
	if err := godotenv.Load(); err != nil {
		slog.Info("未找到 .env 檔案或讀取失敗，將使用系統環境變數")
	}

	// 初始化資料庫連線
	dbPool := database.InitDB()
	defer dbPool.Close()

	// 初始化 Handler (依賴注入)
	h := handlers.NewHandler(dbPool)

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
			public.GET("/equipment", h.GetEquipment)
			public.POST("/report", h.PostMaintenanceRecord)
		}

		// 認證路由
		auth := api.Group("/auth")
		{
			auth.POST("/login", h.AuthenticateUser)
			auth.POST("/refresh", h.RefreshTokenHandler)
		}

		// 需要認證的私有路由
		private := api.Group("/private")
		private.Use(middleware.AuthMiddleware()) // 啟用驗證中間層
		{
			// 管理員與維修人員可以存取
			authorized := private.Group("/")
			authorized.Use(middleware.RoleRequired("admin", "staff"))
			{
				authorized.GET("/equipments", h.GetDetailEquipment)
				authorized.GET("/maintenance-records", h.GetMaintenanceRecords)
				authorized.PATCH("/maintenance-records/resolve", h.ResolveMaintenanceRecord)
			}

			// 僅限管理員
			admin := private.Group("/")
			admin.Use(middleware.RoleRequired("admin"))
			{
				admin.POST("/equipment", h.PostEquipment)
				admin.DELETE("/equipment", h.DeleteEquipment)

				// 使用者管理
				admin.GET("/users", h.GetUsers)
				admin.POST("/user", h.CreateUser)
				admin.PATCH("/user", h.UpdateUser)
				admin.DELETE("/user", h.DeleteUser)
			}
		}
	}

	// 啟動伺服器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("伺服器啟動中", "port", port)
	r.Run(":" + port)
}
