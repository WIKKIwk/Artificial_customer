package handlers

import (
	"bytes"
	"converter-backend/internal/models"
	"converter-backend/internal/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&models.User{})
	return db
}

func Test_Register(t *testing.T) {
	db := setupTestDB()
	err := SetJWTSecret("test-secret-32-characters-long-value")
	assert.NoError(t, err)
	handler := NewAuthHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	// Test successful registration
	t.Run("Success", func(t *testing.T) {
		body := map[string]string{
			"name":     "Test User",
			"email":    "test@example.com",
			"password": "Password123!",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["token"])
		assert.NotNil(t, response["user"])
	})

	// Test duplicate email
	t.Run("Duplicate Email", func(t *testing.T) {
		body := map[string]string{
			"name":     "Test User 2",
			"email":    "test@example.com", // same email
			"password": "Password123!",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// Test invalid input
	t.Run("Invalid Input", func(t *testing.T) {
		body := map[string]string{
			"name":     "Test",
			"email":    "invalid-email",
			"password": "123", // too short
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func Test_Login(t *testing.T) {
	db := setupTestDB()
	err := SetJWTSecret("test-secret-32-characters-long-value")
	assert.NoError(t, err)
	handler := NewAuthHandler(db)

	// Create a test user first
	handler.DB.Create(&models.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "$2a$10$YourHashedPasswordHere", // This would be a real bcrypt hash
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", handler.Login)

	t.Run("Invalid Credentials", func(t *testing.T) {
		body := map[string]string{
			"email":    "test@example.com",
			"password": "wrongpassword",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("User Not Found", func(t *testing.T) {
		body := map[string]string{
			"email":    "nonexistent@example.com",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func Test_EmailService_GenerateToken(t *testing.T) {
	token1, err1 := services.GenerateToken()
	token2, err2 := services.GenerateToken()

	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.NotEqual(t, token1, token2)
	assert.Equal(t, 64, len(token1)) // 32 bytes = 64 hex characters
}
