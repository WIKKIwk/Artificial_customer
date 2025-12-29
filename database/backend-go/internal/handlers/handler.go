package handlers

import (
	"converter-backend/internal/services"
	"converter-backend/internal/utils"
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Service *services.SpreadsheetService
}

func NewHandler(service *services.SpreadsheetService) *Handler {
	return &Handler{Service: service}
}

func (h *Handler) Convert(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer file.Close()

	// Validate File Magic Number
	if err := utils.ValidateFile(file); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process
	rows, err := h.Service.ProcessAndSave(file, fileHeader.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process file: " + err.Error()})
		return
	}

	// Response
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", fileHeader.Filename))
	c.Header("Content-Type", "text/csv")
	c.Header("X-Db-Saved", "true")

	writer := csv.NewWriter(c.Writer)
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return
		}
	}
	writer.Flush()
}
