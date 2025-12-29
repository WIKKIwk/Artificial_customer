package handlers

import (
	"converter-backend/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type createShareInput struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role"` // viewer|editor (default viewer)
}

type shareUserRow struct {
	UserID    uint      `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func normalizeShareRole(role string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", "viewer":
		return "viewer", true
	case "editor":
		return "editor", true
	default:
		return "", false
	}
}

func (h *FileHandler) CreateShare(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	fileID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	fileID := uint(fileID64)

	// Only owner can share (GetFile enforces ownership).
	file, err := h.Service.GetFile(userID, fileID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load file"})
		return
	}

	var input createShareInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role, ok := normalizeShareRole(input.Role)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role (use viewer|editor)"})
		return
	}

	var target models.User
	if err := h.Service.DB.Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(input.Email))).First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lookup user"})
		return
	}

	if target.ID == file.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot share with owner"})
		return
	}

	share := models.SheetFileShare{
		FileID: file.ID,
		UserID: target.ID,
		Role:   role,
	}
	if err := h.Service.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "file_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role", "updated_at"}),
	}).Create(&share).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create share"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id":  file.ID,
		"user_id":  target.ID,
		"email":    target.Email,
		"name":     target.Name,
		"role":     role,
		"message":  "shared",
		"editable": role == "editor",
	})
}

func (h *FileHandler) ListShares(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	fileID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	fileID := uint(fileID64)

	// Only owner can list shares.
	file, err := h.Service.GetFile(userID, fileID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load file"})
		return
	}

	var rows []shareUserRow
	if err := h.Service.DB.Table("sheet_file_shares s").
		Select("s.user_id, u.name, u.email, s.role, s.created_at, s.updated_at").
		Joins("JOIN users u ON u.id = s.user_id").
		Where("s.file_id = ?", file.ID).
		Order("s.created_at asc").
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list shares"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id": file.ID,
		"shares":  rows,
	})
}

func (h *FileHandler) DeleteShare(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	fileID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	fileID := uint(fileID64)

	targetID64, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	targetID := uint(targetID64)

	// Only owner can delete shares.
	file, err := h.Service.GetFile(userID, fileID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load file"})
		return
	}

	res := h.Service.DB.Where("file_id = ? AND user_id = ?", file.ID, targetID).Delete(&models.SheetFileShare{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete share"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "unshared"})
}
