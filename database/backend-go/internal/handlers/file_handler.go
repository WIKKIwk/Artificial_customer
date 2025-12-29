package handlers

import (
	"converter-backend/internal/models"
	"converter-backend/internal/services"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FileHandler struct {
	Service *services.SpreadsheetService
}

func NewFileHandler(service *services.SpreadsheetService) *FileHandler {
	return &FileHandler{Service: service}
}

type saveFileInput struct {
	ID    *uint           `json:"id,omitempty"` // optional: update existing
	Name  string          `json:"name"`
	State json.RawMessage `json:"state"`
}

func (h *FileHandler) Save(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	var input saveFileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if len(input.State) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state is required"})
		return
	}

	var file *models.SheetFile
	accessRole := "owner"
	if input.ID != nil {
		// Update existing file state and name (requires owner/editor)
		existing, role, err := h.Service.GetFileAccess(userID, *input.ID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		if role == "viewer" {
			c.JSON(http.StatusForbidden, gin.H{"error": "read-only access"})
			return
		}
		existing.Name = input.Name
		existing.State = input.State
		if err := h.Service.DB.Save(existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update file"})
			return
		}
		file = existing
		accessRole = role
	} else {
		saved, err := h.Service.SaveFile(userID, input.Name, input.State)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
			return
		}
		file = saved
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          file.ID,
		"name":        file.Name,
		"state":       file.State,
		"access_role": accessRole,
	})
}

func (h *FileHandler) List(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	// Check for pagination parameters
	page := 1
	limit := 50 // default

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit
	files, err := h.Service.ListAccessibleFiles(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files"})
		return
	}
	total := int64(len(files))

	start := offset
	if start < 0 {
		start = 0
	}
	if start > len(files) {
		start = len(files)
	}
	end := start + limit
	if end > len(files) {
		end = len(files)
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files[start:end],
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

func (h *FileHandler) Get(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	idParam := c.Param("id")
	id64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	file, role, err := h.Service.GetFileAccess(userID, uint(id64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          file.ID,
		"user_id":     file.UserID,
		"name":        file.Name,
		"state":       file.State,
		"created_at":  file.CreatedAt,
		"updated_at":  file.UpdatedAt,
		"access_role": role,
	})
}

// Delete handles file deletion
func (h *FileHandler) Delete(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	idParam := c.Param("id")
	id64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	_, role, err := h.Service.GetFileAccess(userID, uint(id64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if role != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can delete"})
		return
	}

	if err := h.Service.DeleteFile(userID, uint(id64)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file deleted successfully"})
}

type patchCellEditInput struct {
	Cell  string `json:"cell,omitempty"` // A1 notation, e.g. "B2"
	Row   *int   `json:"row,omitempty"`  // 0-based
	Col   *int   `json:"col,omitempty"`  // 0-based
	Value string `json:"value"`          // raw cell input
}

type patchCellsInput struct {
	Edits []patchCellEditInput `json:"edits"`
}

func (h *FileHandler) PatchCells(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(uint)

	idParam := c.Param("id")
	id64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var input patchCellsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(input.Edits) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "edits is required"})
		return
	}
	if len(input.Edits) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many edits (max 1000)"})
		return
	}

	edits := make([]services.CellEdit, 0, len(input.Edits))
	for _, edit := range input.Edits {
		var row, col int
		var ok bool
		if edit.Cell != "" {
			row, col, ok = a1ToRowCol(edit.Cell)
		} else if edit.Row != nil && edit.Col != nil {
			row, col = *edit.Row, *edit.Col
			ok = true
		}
		if !ok || row < 0 || col < 0 {
			continue
		}
		edits = append(edits, services.CellEdit{Row: row, Col: col, Value: edit.Value})
	}

	if len(edits) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid edits"})
		return
	}

	fileID := uint(id64)
	_, role, err := h.Service.GetFileAccess(userID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "read-only access"})
		return
	}

	file, updated, err := h.Service.PatchFileCells(fileID, edits)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to patch cells"})
		return
	}

	go notifyRealtimeBatchEdits(file.ID, edits)

	c.JSON(http.StatusOK, gin.H{
		"id":      file.ID,
		"updated": updated,
	})
}

func a1ToRowCol(cell string) (row int, col int, ok bool) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return 0, 0, false
	}

	i := 0
	for i < len(cell) {
		ch := cell[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			i++
			continue
		}
		break
	}

	if i == 0 || i == len(cell) {
		return 0, 0, false
	}

	colStr := cell[:i]
	rowStr := cell[i:]

	rowNum, err := strconv.Atoi(rowStr)
	if err != nil || rowNum <= 0 {
		return 0, 0, false
	}

	colNum := 0
	for j := 0; j < len(colStr); j++ {
		ch := colStr[j]
		if ch >= 'a' && ch <= 'z' {
			ch = ch - 'a' + 'A'
		}
		if ch < 'A' || ch > 'Z' {
			return 0, 0, false
		}
		colNum = colNum*26 + int(ch-'A'+1)
	}

	return rowNum - 1, colNum - 1, true
}
