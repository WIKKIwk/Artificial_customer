package handlers

import (
	"converter-backend/internal/logger"
	"converter-backend/internal/models"
	"converter-backend/internal/services"
	"converter-backend/internal/utils"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB           *gorm.DB
	EmailService *services.EmailService
}

func NewAuthHandlerWithEmail(db *gorm.DB, emailService *services.EmailService) *AuthHandler {
	return &AuthHandler{
		DB:           db,
		EmailService: emailService,
	}
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

type AuthInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RegisterInput struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

var jwtSecret []byte

const apiKeyLength = 32

// SetJWTSecret sets the signing key that must be configured at startup.
func SetJWTSecret(secret string) error {
	if len(secret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters, got %d", len(secret))
	}
	jwtSecret = []byte(secret)
	return nil
}

func generateToken(userID uint) (string, error) {
	return generateTokenWithTTL(userID, 24*time.Hour)
}

func generateTokenWithTTL(userID uint, ttl time.Duration) (string, error) {
	secret, err := getJWTSecret()
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(ttl).Unix(),
	})

	return token.SignedString(secret)
}

func generateSheetToken(userID uint, sheetID uint, role string, ttl time.Duration) (string, error) {
	secret, err := getJWTSecret()
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      userID,
		"exp":      time.Now().Add(ttl).Unix(),
		"sheet_id": sheetID,
		"role":     role,
	})

	return token.SignedString(secret)
}

func generateApiKey() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const alphabetLen = len(alphabet)

	// Generate cryptographically secure random bytes
	randomBytes := make([]byte, apiKeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Convert to alphanumeric without modulo bias
	// Use rejection sampling to avoid bias
	result := make([]byte, apiKeyLength)
	for i := 0; i < apiKeyLength; i++ {
		// Keep generating random bytes until we get one in valid range
		// This eliminates modulo bias
		for {
			if _, err := rand.Read(randomBytes[i : i+1]); err != nil {
				return "", err
			}
			// Only accept values that don't cause bias (0-247 for alphabet of 62 chars)
			// 248 = 62 * 4, so 0-247 divides evenly
			if randomBytes[i] < 248 {
				result[i] = alphabet[int(randomBytes[i])%alphabetLen]
				break
			}
		}
	}

	// Add prefix for versioning and easy identification
	return "sk_" + string(result), nil
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate password strength
	if err := utils.ValidatePassword(input.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Generate email verification token
	verifyToken, err := services.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}

	// Set verification token expiry to 48 hours
	verifyExpiry := time.Now().Add(48 * time.Hour)

	user := models.User{
		Name:                   input.Name,
		Email:                  input.Email,
		Password:               string(hashedPassword),
		EmailVerifyToken:       verifyToken,
		EmailVerifyTokenExpiry: &verifyExpiry,
		EmailVerified:          false,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		if isDuplicateEmailError(err) {
			logger.SecurityEvent("registration_duplicate_email", input.Email, c.ClientIP(), "Attempt to register with existing email")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
			return
		}

		logger.Error(fmt.Sprintf("Registration failed for %s: %v", input.Email, err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Log successful registration
	logger.SecurityEvent("registration_success", user.Email, c.ClientIP(), "New user registered successfully")

	// Send verification email
	if h.EmailService != nil {
		if err := h.EmailService.SendVerificationEmail(user.Email, verifyToken); err != nil {
			// Log error but don't fail registration
			fmt.Printf("Failed to send verification email: %v\n", err)
		}
	}

	tokenString, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"user": gin.H{
			"id":             user.ID,
			"name":           user.Name,
			"email":          user.Email,
			"email_verified": user.EmailVerified,
		},
		"message": "Registration successful. Please check your email to verify your account.",
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input AuthInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	userFound := true
	if err := h.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		userFound = false
		// Create a dummy user with a bcrypt hash to prevent timing attacks
		// Even if user doesn't exist, we still perform bcrypt comparison
		user.Password = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy" // dummy hash
	}

	// Always perform bcrypt comparison to prevent timing attacks
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))

	// Check both conditions together
	if !userFound || err != nil {
		// Increment failed login attempts for existing users (no lockout).
		if userFound {
			user.FailedLoginAttempts++
			logger.SecurityEvent("login_failed", input.Email, c.ClientIP(), fmt.Sprintf("Failed login attempt (%d)", user.FailedLoginAttempts))
			h.DB.Save(&user)
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Successful login - reset failed attempts
	if userFound {
		user.FailedLoginAttempts = 0
		user.LockedUntil = nil
		h.DB.Save(&user)
		logger.SecurityEvent("login_success", input.Email, c.ClientIP(), "Successful login")
	}

	tokenString, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"user": gin.H{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

// AuthMiddleware accepts either Bearer JWT or API key (Authorization: ApiKey <key> or X-API-Key header).
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		apiKeyHeader := c.GetHeader("X-API-Key")

		// API key flow
		apiKey := ""
		if strings.HasPrefix(authHeader, "ApiKey ") {
			apiKey = strings.TrimPrefix(authHeader, "ApiKey ")
		} else if apiKeyHeader != "" {
			apiKey = apiKeyHeader
		}
		if apiKey != "" {
			var user models.User
			if err := h.DB.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
				c.Abort()
				return
			}
			c.Set("user_id", user.ID)
			c.Next()
			return
		}

		// JWT flow
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		secret, err := getJWTSecret()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server misconfiguration"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return secret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		sub, ok := claims["sub"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token subject"})
			c.Abort()
			return
		}

		c.Set("user_id", uint(sub))
		c.Next()
	}
}

// GenerateAPIKey creates a new 32-character API key with sk_ prefix for the authenticated user.
func (h *AuthHandler) GenerateAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid := userID.(uint)

	var key string
	for i := 0; i < 5; i++ {
		k, err := generateApiKey()
		if err != nil {
			continue
		}
		var count int64
		h.DB.Model(&models.User{}).Where("api_key = ?", k).Count(&count)
		if count == 0 {
			key = k
			break
		}
	}

	if key == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
		return
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", uid).Update("api_key", key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save API key"})
		return
	}

	// Get user email for logging
	var user models.User
	if err := h.DB.First(&user, uid).Error; err == nil {
		logger.SecurityEvent("api_key_generated", user.Email, c.ClientIP(), "New API key generated")
	}

	c.JSON(http.StatusOK, gin.H{"api_key": key})
}

// GenerateRealtimeToken returns a short-lived JWT suitable for Phoenix realtime join.
// Works with both Bearer JWT and API key auth via AuthMiddleware.
func (h *AuthHandler) GenerateRealtimeToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid := userID.(uint)
	ttl := 15 * time.Minute

	tokenString, err := generateTokenWithTTL(uid, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      tokenString,
		"expires_in": int(ttl.Seconds()),
	})
}

// VerifyEmail verifies user's email with token
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	var user models.User
	if err := h.DB.Where("email_verify_token = ?", token).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Check if token is expired
	if user.EmailVerifyTokenExpiry == nil || time.Now().After(*user.EmailVerifyTokenExpiry) {
		logger.SecurityEvent("email_verification_expired", user.Email, c.ClientIP(), "Attempted to use expired verification token")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token has expired. Please request a new one."})
		return
	}

	// Update user as verified
	user.EmailVerified = true
	user.EmailVerifyToken = ""
	user.EmailVerifyTokenExpiry = nil
	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	logger.SecurityEvent("email_verified", user.Email, c.ClientIP(), "Email successfully verified")
	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
}

// ForgotPassword initiates password reset process
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		// Don't reveal if email exists or not for security
		logger.SecurityEvent("password_reset_unknown_email", input.Email, c.ClientIP(), "Password reset requested for non-existent email")
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent"})
		return
	}

	logger.SecurityEvent("password_reset_requested", input.Email, c.ClientIP(), "Password reset token generated")

	// Generate reset token
	token, err := services.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	// Set token and expiry (1 hour)
	expiry := time.Now().Add(1 * time.Hour)
	user.ResetToken = token
	user.ResetTokenExpiry = &expiry

	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reset token"})
		return
	}

	// Send email
	if h.EmailService != nil {
		if err := h.EmailService.SendPasswordResetEmail(user.Email, token); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to send password reset email: %v\n", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent"})
}

// ResetPassword resets user password with token
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var input struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate password strength
	if err := utils.ValidatePassword(input.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.DB.Where("reset_token = ?", input.Token).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Check if token is expired
	if user.ResetTokenExpiry == nil || time.Now().After(*user.ResetTokenExpiry) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token has expired"})
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Update password and clear reset token
	user.Password = string(hashedPassword)
	user.ResetToken = ""
	user.ResetTokenExpiry = nil

	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	logger.SecurityEvent("password_reset_completed", user.Email, c.ClientIP(), "Password successfully reset")
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func getJWTSecret() ([]byte, error) {
	if len(jwtSecret) == 0 {
		return nil, errors.New("jwt secret not configured")
	}
	return jwtSecret, nil
}

func isDuplicateEmailError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "unique constraint") {
		return true
	}

	return false
}
