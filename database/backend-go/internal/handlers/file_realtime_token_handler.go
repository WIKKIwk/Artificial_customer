package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// FileRealtimeToken returns a short-lived JWT scoped to a specific file (sheet).
// It includes claims: sub, exp, sheet_id, role.
func (h *FileHandler) FileRealtimeToken(c *gin.Context) {
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

	_, role, err := h.Service.GetFileAccess(userID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	ttl := 15 * time.Minute
	tokenString, err := generateSheetToken(userID, fileID, role, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      tokenString,
		"expires_in": int(ttl.Seconds()),
		"sheet_id":   fileID,
		"role":       role,
	})
}
