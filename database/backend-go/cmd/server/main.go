package main

import (
	"converter-backend/internal/handlers"
	"converter-backend/internal/models"
	"converter-backend/internal/services"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found, using defaults")
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "converter-dev-secret-32-characters-long!!!"
	}
	if err := handlers.SetJWTSecret(secret); err != nil {
		log.Fatalf("JWT secret is invalid: %v", err)
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "host=localhost user=user password=password dbname=converter_db port=5439 sslmode=disable"
	}

	// Init Service & Handler
	service := services.NewSpreadsheetService(dsn)
	handler := handlers.NewHandler(service)

	// Init Auth
	authHandler := handlers.NewAuthHandler(service.DB)
	service.DB.AutoMigrate(&models.User{}, &models.SheetFileShare{})
	fileHandler := handlers.NewFileHandler(service)

	r := gin.Default()

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition", "X-Db-Saved"},
		AllowCredentials: true,
	}))

	// Health check endpoint for production monitoring
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "sheetmaster-api",
			"version": "1.0.0",
		})
	})

	r.POST("/convert", handler.Convert)
	r.POST("/register", authHandler.Register)
	r.POST("/login", authHandler.Login)

	// API-prefixed routes for frontend
	api := r.Group("/api")
	{
		api.POST("/register", authHandler.Register)
		api.POST("/login", authHandler.Login)
		api.GET("/me", authHandler.AuthMiddleware(), authHandler.Me)
		api.POST("/api-key", authHandler.AuthMiddleware(), authHandler.GenerateAPIKey)
		api.Use(authHandler.AuthMiddleware())
		{
			api.GET("/files", fileHandler.List)
			api.POST("/files", fileHandler.Save)
			api.GET("/files/:id", fileHandler.Get)
			api.DELETE("/files/:id", fileHandler.Delete)
			api.PATCH("/files/:id/cells", fileHandler.PatchCells)
			api.POST("/files/:id/realtime/token", fileHandler.FileRealtimeToken)
			api.GET("/files/:id/shares", fileHandler.ListShares)
			api.POST("/files/:id/shares", fileHandler.CreateShare)
			api.DELETE("/files/:id/shares/:userId", fileHandler.DeleteShare)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
