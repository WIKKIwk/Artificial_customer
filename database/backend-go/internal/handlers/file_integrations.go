package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

const (
	maxCellsReadGrid = 20000
	maxSchemaCols    = 500
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func a1RangeToBounds(rangeStr string) (minRow, maxRow, minCol, maxCol int, ok bool) {
	trimmed := strings.TrimSpace(rangeStr)
	if trimmed == "" {
		return 0, 0, 0, 0, false
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) == 1 {
		row, col, ok := a1ToRowCol(parts[0])
		if !ok {
			return 0, 0, 0, 0, false
		}
		return row, row, col, col, true
	}
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}

	r1, c1, ok1 := a1ToRowCol(parts[0])
	r2, c2, ok2 := a1ToRowCol(parts[1])
	if !ok1 || !ok2 {
		return 0, 0, 0, 0, false
	}

	minRow = minInt(r1, r2)
	maxRow = maxInt(r1, r2)
	minCol = minInt(c1, c2)
	maxCol = maxInt(c1, c2)
	return minRow, maxRow, minCol, maxCol, true
}

func colToLabel(col int) string {
	if col < 0 {
		return ""
	}
	label := ""
	for col >= 0 {
		rem := col % 26
		label = string(rune('A'+rem)) + label
		col = col/26 - 1
	}
	return label
}

func stateCellRawValue(cellAny map[string]any) string {
	if cellAny == nil {
		return ""
	}
	v, ok := cellAny["value"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(v)
	}
}

func stateCellComputedValue(cellAny map[string]any) string {
	if cellAny == nil {
		return ""
	}
	v, ok := cellAny["computed"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(v)
	}
}

func containsLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func isNumericLike(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	_, err := strconv.ParseFloat(trimmed, 64)
	return err == nil
}

// GetCells returns sheet values in a specific A1 range.
// Example: GET /api/v1/files/:id/cells?range=A1:D20&format=grid
func (h *FileHandler) GetCells(c *gin.Context) {
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
	fileID := uint(id64)

	rangeStr := c.Query("range")
	if strings.TrimSpace(rangeStr) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range is required (e.g. A1:D20)"})
		return
	}
	minRow, maxRow, minCol, maxCol, ok := a1RangeToBounds(rangeStr)
	if !ok || minRow < 0 || minCol < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range"})
		return
	}

	rows := maxRow - minRow + 1
	cols := maxCol - minCol + 1
	if rows <= 0 || cols <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range"})
		return
	}
	if rows*cols > maxCellsReadGrid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("range too large (max %d cells)", maxCellsReadGrid)})
		return
	}

	file, role, err := h.Service.GetFileAccess(userID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	var state map[string]any
	if err := json.Unmarshal(file.State, &state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode file state"})
		return
	}
	dataAny, _ := state["data"].(map[string]any)
	if dataAny == nil {
		dataAny = map[string]any{}
	}

	valueMode := strings.ToLower(strings.TrimSpace(c.Query("value")))
	if valueMode == "" {
		valueMode = "raw"
	}
	getValue := func(cellAny map[string]any) string {
		if valueMode == "computed" {
			if v := stateCellComputedValue(cellAny); strings.TrimSpace(v) != "" {
				return v
			}
		}
		return stateCellRawValue(cellAny)
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if format == "" {
		format = "grid"
	}

	if format == "sparse" {
		type sparseCell struct {
			Row   int    `json:"row"`
			Col   int    `json:"col"`
			Value string `json:"value"`
		}

		out := make([]sparseCell, 0, 64)
		for r := minRow; r <= maxRow; r++ {
			for col := minCol; col <= maxCol; col++ {
				cellID := fmt.Sprintf("%d,%d", r, col)
				cellAny, _ := dataAny[cellID].(map[string]any)
				val := strings.TrimSpace(getValue(cellAny))
				if val == "" {
					continue
				}
				out = append(out, sparseCell{Row: r, Col: col, Value: val})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"file_id":     file.ID,
			"range":       strings.TrimSpace(rangeStr),
			"start":       gin.H{"row": minRow, "col": minCol},
			"end":         gin.H{"row": maxRow, "col": maxCol},
			"format":      "sparse",
			"value_mode":  valueMode,
			"cells":       out,
			"access_role": role,
		})
		return
	}

	values := make([][]string, rows)
	for r := 0; r < rows; r++ {
		rowIdx := minRow + r
		rowVals := make([]string, cols)
		for col := 0; col < cols; col++ {
			colIdx := minCol + col
			cellID := fmt.Sprintf("%d,%d", rowIdx, colIdx)
			cellAny, _ := dataAny[cellID].(map[string]any)
			rowVals[col] = getValue(cellAny)
		}
		values[r] = rowVals
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id":     file.ID,
		"range":       strings.TrimSpace(rangeStr),
		"start":       gin.H{"row": minRow, "col": minCol},
		"end":         gin.H{"row": maxRow, "col": maxCol},
		"format":      "grid",
		"value_mode":  valueMode,
		"values":      values,
		"access_role": role,
	})
}

// GetSchema returns a lightweight schema description: used range, guessed header row and column headers.
func (h *FileHandler) GetSchema(c *gin.Context) {
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
	fileID := uint(id64)

	file, role, err := h.Service.GetFileAccess(userID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	var state map[string]any
	if err := json.Unmarshal(file.State, &state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode file state"})
		return
	}
	dataAny, _ := state["data"].(map[string]any)
	if dataAny == nil {
		dataAny = map[string]any{}
	}

	minRow := int(^uint(0) >> 1)
	minCol := int(^uint(0) >> 1)
	maxRow := -1
	maxCol := -1

	// Compute used range based on non-empty raw values.
	for key, cell := range dataAny {
		cellAny, ok := cell.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stateCellRawValue(cellAny)) == "" {
			continue
		}

		parts := strings.Split(key, ",")
		if len(parts) != 2 {
			continue
		}
		r, errR := strconv.Atoi(parts[0])
		col, errC := strconv.Atoi(parts[1])
		if errR != nil || errC != nil || r < 0 || col < 0 {
			continue
		}

		if r < minRow {
			minRow = r
		}
		if col < minCol {
			minCol = col
		}
		if r > maxRow {
			maxRow = r
		}
		if col > maxCol {
			maxCol = col
		}
	}

	if maxRow < 0 || maxCol < 0 || minRow == int(^uint(0)>>1) || minCol == int(^uint(0)>>1) {
		c.JSON(http.StatusOK, gin.H{
			"file_id":     file.ID,
			"name":        file.Name,
			"used_range":  nil,
			"header_row":  nil,
			"columns":     []any{},
			"access_role": role,
		})
		return
	}

	if maxCol-minCol+1 > maxSchemaCols {
		maxCol = minCol + maxSchemaCols - 1
	}

	// Guess header row by scanning first few rows from minRow.
	headerRow := minRow
	bestScore := -1.0
	for r := minRow; r <= minRow+4 && r <= maxRow; r++ {
		nonEmpty := 0
		textLike := 0
		for col := minCol; col <= maxCol; col++ {
			cellID := fmt.Sprintf("%d,%d", r, col)
			cellAny, _ := dataAny[cellID].(map[string]any)
			v := strings.TrimSpace(stateCellRawValue(cellAny))
			if v == "" {
				continue
			}
			nonEmpty++
			if containsLetter(v) && !isNumericLike(v) {
				textLike++
			}
		}
		score := float64(nonEmpty) + float64(textLike)*0.25
		if score > bestScore {
			bestScore = score
			headerRow = r
		}
	}

	columns := make([]gin.H, 0, maxCol-minCol+1)
	categoryCandidates := make([]gin.H, 0, 4)
	keywords := []string{"category", "kategoriya", "categoriya", "катег", "категория", "tur", "type", "group", "bo'lim", "bo‘lim", "section"}

	for col := minCol; col <= maxCol; col++ {
		cellID := fmt.Sprintf("%d,%d", headerRow, col)
		cellAny, _ := dataAny[cellID].(map[string]any)
		header := strings.TrimSpace(stateCellRawValue(cellAny))
		label := colToLabel(col)
		columns = append(columns, gin.H{
			"col":    col,
			"label":  label,
			"header": header,
		})

		lower := strings.ToLower(strings.ReplaceAll(header, " ", ""))
		if lower != "" {
			for _, kw := range keywords {
				if strings.Contains(lower, strings.ToLower(strings.ReplaceAll(kw, " ", ""))) {
					categoryCandidates = append(categoryCandidates, gin.H{
						"col":   col,
						"label": label,
						"match": kw,
						"text":  header,
					})
					break
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id": file.ID,
		"name":    file.Name,
		"used_range": gin.H{
			"min_row": minRow,
			"max_row": maxRow,
			"min_col": minCol,
			"max_col": maxCol,
			"a1":      fmt.Sprintf("%s%d:%s%d", colToLabel(minCol), minRow+1, colToLabel(maxCol), maxRow+1),
		},
		"header_row": headerRow,
		"columns":    columns,
		"category_candidates": gin.H{
			"by_header": categoryCandidates,
		},
		"access_role": role,
	})
}
