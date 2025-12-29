package telegram

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type sheetMasterCellEdit struct {
	Row   int
	Col   int
	Value string
}

type inventoryAdjustment struct {
	Name  string
	Delta int
}

var (
	inventoryNameHeaders = []string{"название", "товар", "product", "name", "mahsulot", "nomi"}
	inventoryQtyHeaders  = []string{"количество", "qty", "quantity", "soni", "qolgan", "остаток", "stock", "count"}
)

func (h *BotHandler) syncInventoryAfterOrder(summary string, skip bool) {
	if skip {
		return
	}
	items := extractOrderItemNames(summary)
	if len(items) == 0 {
		items = extractConfigItemNames(summary)
	}
	if len(items) == 0 {
		return
	}

	go func(items []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		h.sheetMasterSyncMu.Lock()
		defer h.sheetMasterSyncMu.Unlock()

		selectedID, _ := sheetMasterSelectedFileID()

		if cfg, cfgErr := h.resolveSheetMasterConfig(); cfgErr == nil {
			if selectedID != 0 {
				cfg.FileID = strconv.FormatUint(uint64(selectedID), 10)
			}
			if updated, edits, err := sheetMasterDecrementStockViaAPI(ctx, cfg, items); err == nil {
				if updated > 0 {
					if len(edits) > 0 {
						if fileID, parseErr := strconv.ParseUint(cfg.FileID, 10, 64); parseErr == nil && fileID > 0 {
							if err := notifyRealtimeBatchEdits(ctx, uint(fileID), edits); err != nil {
								log.Printf("[inventory] api realtime notify failed: %v", err)
							}
						}
					}
					if err := h.refreshCatalogFromSheetMasterLocked(ctx); err != nil {
						log.Printf("[inventory] catalog refresh failed: %v", err)
					} else {
						log.Printf("[inventory] stock updated via api and catalog refreshed (file_id=%s, items=%d)", cfg.FileID, updated)
					}
					return
				}
				log.Printf("[inventory] api update returned 0 changes (file_id=%s), falling back to db", cfg.FileID)
			} else {
				log.Printf("[inventory] api update failed: %v", err)
			}
		}

		updated, fileID, edits, err := sheetMasterDecrementStockInDB(ctx, selectedID, items)
		if err != nil {
			log.Printf("[inventory] db update failed: %v", err)
			return
		}

		if updated == 0 {
			log.Printf("[inventory] no stock updates applied (file_id=%d)", fileID)
			return
		}

		if err := notifyRealtimeBatchEdits(ctx, fileID, edits); err != nil {
			log.Printf("[inventory] realtime notify failed: %v", err)
		}

		if err := h.refreshCatalogFromSheetMasterLocked(ctx); err != nil {
			log.Printf("[inventory] catalog refresh failed: %v", err)
		} else {
			log.Printf("[inventory] stock updated and catalog refreshed (file_id=%d, items=%d)", fileID, updated)
		}
	}(items)
}

func (h *BotHandler) adjustInventoryByName(ctx context.Context, item string, delta int) (int, error) {
	item = strings.TrimSpace(item)
	if item == "" || delta == 0 {
		return 0, nil
	}

	adjustments := []inventoryAdjustment{{Name: item, Delta: delta}}

	h.sheetMasterSyncMu.Lock()
	defer h.sheetMasterSyncMu.Unlock()

	selectedID, _ := sheetMasterSelectedFileID()

	if cfg, cfgErr := h.resolveSheetMasterConfig(); cfgErr == nil {
		if selectedID != 0 {
			cfg.FileID = strconv.FormatUint(uint64(selectedID), 10)
		}
		if updated, edits, _, err := sheetMasterAdjustStockViaAPI(ctx, cfg, adjustments); err == nil {
			if updated > 0 {
				if len(edits) > 0 {
					if fileID, parseErr := strconv.ParseUint(cfg.FileID, 10, 64); parseErr == nil && fileID > 0 {
						if err := notifyRealtimeBatchEdits(ctx, uint(fileID), edits); err != nil {
							log.Printf("[inventory] api realtime notify failed: %v", err)
						}
					}
				}
				if err := h.refreshCatalogFromSheetMasterLocked(ctx); err != nil {
					log.Printf("[inventory] catalog refresh failed: %v", err)
				}
				return updated, nil
			}
			log.Printf("[inventory] api update returned 0 changes (file_id=%s), falling back to db", cfg.FileID)
		} else {
			log.Printf("[inventory] api update failed: %v", err)
		}
	}

	updated, fileID, edits, _, err := sheetMasterAdjustStockInDB(ctx, selectedID, adjustments)
	if err != nil {
		return 0, err
	}
	if updated == 0 {
		return 0, nil
	}
	if err := notifyRealtimeBatchEdits(ctx, fileID, edits); err != nil {
		log.Printf("[inventory] realtime notify failed: %v", err)
	}
	if err := h.refreshCatalogFromSheetMasterLocked(ctx); err != nil {
		log.Printf("[inventory] catalog refresh failed: %v", err)
	}
	return updated, nil
}

func (h *BotHandler) adjustInventoryItems(ctx context.Context, items []string, delta int) (int, []string, error) {
	adjustments := buildInventoryAdjustments(items, delta)
	if len(adjustments) == 0 {
		return 0, nil, nil
	}

	h.sheetMasterSyncMu.Lock()
	defer h.sheetMasterSyncMu.Unlock()

	selectedID, _ := sheetMasterSelectedFileID()

	if cfg, cfgErr := h.resolveSheetMasterConfig(); cfgErr == nil {
		if selectedID != 0 {
			cfg.FileID = strconv.FormatUint(uint64(selectedID), 10)
		}
		if updated, edits, updatedNames, err := sheetMasterAdjustStockViaAPI(ctx, cfg, adjustments); err == nil {
			if updated > 0 {
				if len(edits) > 0 {
					if fileID, parseErr := strconv.ParseUint(cfg.FileID, 10, 64); parseErr == nil && fileID > 0 {
						if err := notifyRealtimeBatchEdits(ctx, uint(fileID), edits); err != nil {
							log.Printf("[inventory] api realtime notify failed: %v", err)
						}
					}
				}
				if err := h.refreshCatalogFromSheetMasterLocked(ctx); err != nil {
					log.Printf("[inventory] catalog refresh failed: %v", err)
				}
				return updated, updatedNames, nil
			}
			log.Printf("[inventory] api update returned 0 changes (file_id=%s), falling back to db", cfg.FileID)
		} else {
			log.Printf("[inventory] api update failed: %v", err)
		}
	}

	updated, fileID, edits, updatedNames, err := sheetMasterAdjustStockInDB(ctx, selectedID, adjustments)
	if err != nil {
		return 0, nil, err
	}
	if updated == 0 {
		return 0, nil, nil
	}
	if err := notifyRealtimeBatchEdits(ctx, fileID, edits); err != nil {
		log.Printf("[inventory] realtime notify failed: %v", err)
	}
	if err := h.refreshCatalogFromSheetMasterLocked(ctx); err != nil {
		log.Printf("[inventory] catalog refresh failed: %v", err)
	}
	return updated, updatedNames, nil
}

func extractOrderItemNames(text string) []string {
	var items []string
	for _, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		if strings.Contains(lower, "jami") ||
			strings.Contains(lower, "итого") ||
			strings.Contains(lower, "total") ||
			strings.Contains(lower, "narxi") ||
			strings.Contains(lower, "price") {
			continue
		}
		if strings.Contains(lower, "kategoriya") ||
			strings.Contains(lower, "category") ||
			strings.Contains(lower, "soni") ||
			strings.Contains(lower, "qty") ||
			strings.Contains(lower, "quantity") ||
			strings.Contains(lower, "stock") ||
			strings.Contains(lower, "qolgan") ||
			strings.Contains(lower, "ostatok") {
			continue
		}
		if strings.Contains(lower, "tugma") ||
			strings.Contains(lower, "savatcha") ||
			strings.Contains(lower, "savat") {
			continue
		}
		name := normalizeCartTitleCandidate(t)
		if name == "" {
			continue
		}
		items = append(items, name)
	}
	if len(items) == 0 {
		if title := strings.TrimSpace(cartTitleFromText(text)); title != "" {
			items = append(items, title)
		}
	}
	return items
}

func extractConfigItemNames(text string) []string {
	block := normalizeSpecBlock(text)
	if strings.TrimSpace(block) == "" {
		block = formatOrderStatusSummary(text)
	}
	if strings.TrimSpace(block) == "" {
		block = text
	}

	seen := make(map[string]struct{})
	var items []string
	for _, ln := range strings.Split(block, "\n") {
		t := stripBulletPrefix(strings.TrimSpace(ln))
		if t == "" {
			continue
		}
		if isConfigSummaryLine(t) {
			continue
		}
		if !priceWithCurrencyRegex.MatchString(t) {
			continue
		}
		if configLineHasZeroPrice(t) {
			continue
		}
		name := extractConfigItemName(t)
		if name == "" {
			continue
		}
		if isConfigPlaceholderName(name) {
			continue
		}
		norm := normalizeInventoryName(name)
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		items = append(items, name)
	}
	return items
}

func isConfigSummaryLine(line string) bool {
	trim := strings.TrimSpace(line)
	if trim == "" {
		return true
	}
	if jamiRegex.MatchString(stripBulletPrefix(trim)) {
		return true
	}
	lower := strings.ToLower(trim)
	if strings.HasPrefix(lower, "overall price") || strings.HasPrefix(lower, "price") {
		return true
	}
	if strings.HasPrefix(lower, "стоимость") || strings.HasPrefix(lower, "цена") {
		return true
	}
	return false
}

func configLineHasZeroPrice(line string) bool {
	for _, match := range priceWithCurrencyRegex.FindAllString(line, -1) {
		if val, ok := parseNumberWithSeparators(match); ok && val == 0 {
			return true
		}
	}
	return false
}

func extractConfigItemName(line string) string {
	t := stripBulletPrefix(strings.TrimSpace(line))
	if t == "" {
		return ""
	}
	if idx := strings.Index(t, ":"); idx >= 0 {
		label := strings.TrimSpace(t[:idx])
		if priceLabelRegex.MatchString(strings.ToLower(label)) {
			return ""
		}
		t = strings.TrimSpace(t[idx+1:])
	}
	if t == "" {
		return ""
	}
	t = priceWithCurrencyRegex.ReplaceAllString(t, "")
	t = priceLabelRegex.ReplaceAllString(t, "")
	t = strings.TrimSpace(strings.Trim(t, "-–—:"))
	t = strings.TrimSpace(strings.TrimLeft(t, "-–— "))
	t = strings.TrimSpace(strings.Join(strings.Fields(t), " "))
	return t
}

func isConfigPlaceholderName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.NewReplacer("’", "'", "‘", "'", "`", "'").Replace(lower)
	if lower == "" {
		return true
	}
	clean := strings.NewReplacer("'", "", "’", "", "‘", "", "`", "").Replace(lower)
	switch {
	case strings.Contains(lower, "integrated") || strings.Contains(lower, "onboard") || strings.Contains(lower, "встро"):
		return true
	case strings.Contains(lower, "kerak emas") || strings.Contains(clean, "kerakemas"):
		return true
	case strings.Contains(clean, "yoq") || strings.Contains(lower, "yo'q") || strings.Contains(lower, "нет"):
		return true
	case strings.Contains(lower, "ko'rsatilmagan") || strings.Contains(lower, "ko‘rsatilmagan") || strings.Contains(clean, "korsatilmagan"):
		return true
	case strings.Contains(lower, "aniqlanmagan") || strings.Contains(lower, "n/a") || strings.Contains(lower, "none"):
		return true
	}
	return false
}

func buildInventoryAdjustments(items []string, delta int) []inventoryAdjustment {
	if delta == 0 || len(items) == 0 {
		return nil
	}
	adjustments := make([]inventoryAdjustment, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		adjustments = append(adjustments, inventoryAdjustment{Name: item, Delta: delta})
	}
	return adjustments
}

func sheetMasterDecrementStockInDB(ctx context.Context, fileID uint, items []string) (int, uint, []sheetMasterCellEdit, error) {
	adjustments := buildInventoryAdjustments(items, -1)
	updated, updatedFileID, edits, _, err := sheetMasterAdjustStockInDB(ctx, fileID, adjustments)
	return updated, updatedFileID, edits, err
}

func sheetMasterAdjustStockInDB(ctx context.Context, fileID uint, adjustments []inventoryAdjustment) (int, uint, []sheetMasterCellEdit, []string, error) {
	if len(adjustments) == 0 {
		return 0, 0, nil, nil, nil
	}
	dsn, err := sheetMasterDBDSNFromEnv()
	if err != nil {
		return 0, 0, nil, nil, err
	}
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return 0, 0, nil, nil, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, 0, nil, nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var file sheetMasterDBFile
	if fileID == 0 {
		row := tx.QueryRowContext(ctx, `
SELECT id, name, state, updated_at
FROM sheet_files
ORDER BY updated_at DESC, id DESC
LIMIT 1
FOR UPDATE`)
		if scanErr := row.Scan(&file.ID, &file.Name, &file.StateJSON, &file.UpdatedAt); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return 0, 0, nil, nil, fmt.Errorf("sheet_files table is empty")
			}
			return 0, 0, nil, nil, scanErr
		}
	} else {
		row := tx.QueryRowContext(ctx, `
SELECT id, name, state, updated_at
FROM sheet_files
WHERE id = $1
LIMIT 1
FOR UPDATE`, fileID)
		if scanErr := row.Scan(&file.ID, &file.Name, &file.StateJSON, &file.UpdatedAt); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return 0, fileID, nil, nil, fmt.Errorf("file not found")
			}
			return 0, fileID, nil, nil, scanErr
		}
	}

	state, dataAny, grid, headerRow, nameCol, qtyCol, err := sheetMasterPrepareInventoryState(file.StateJSON)
	if err != nil {
		return 0, file.ID, nil, nil, err
	}

	edits, updatedNames := sheetMasterBuildInventoryAdjustEditsWithNames(adjustments, grid, headerRow, nameCol, qtyCol)
	if len(edits) == 0 {
		if commitErr := tx.Commit(); commitErr != nil {
			return 0, file.ID, nil, nil, commitErr
		}
		committed = true
		return 0, file.ID, nil, nil, nil
	}

	applyInventoryEdits(dataAny, edits)

	state["data"] = dataAny
	nextState, err := json.Marshal(state)
	if err != nil {
		return 0, file.ID, nil, nil, fmt.Errorf("encode sheet state: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE sheet_files
SET state = $1, updated_at = NOW()
WHERE id = $2`, nextState, file.ID); err != nil {
		return 0, file.ID, nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return 0, file.ID, nil, nil, err
	}
	committed = true

	return len(edits), file.ID, edits, updatedNames, nil
}

func sheetMasterDecrementStockViaAPI(ctx context.Context, cfg sheetMasterConfig, items []string) (int, []sheetMasterCellEdit, error) {
	adjustments := buildInventoryAdjustments(items, -1)
	updated, edits, _, err := sheetMasterAdjustStockViaAPI(ctx, cfg, adjustments)
	return updated, edits, err
}

func sheetMasterAdjustStockViaAPI(ctx context.Context, cfg sheetMasterConfig, adjustments []inventoryAdjustment) (int, []sheetMasterCellEdit, []string, error) {
	if len(adjustments) == 0 {
		return 0, nil, nil, nil
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.FileID) == "" {
		return 0, nil, nil, fmt.Errorf("sheetmaster api not configured")
	}
	schema, err := sheetMasterGetSchema(ctx, cfg)
	if err != nil {
		return 0, nil, nil, err
	}
	if schema.UsedRange == nil || strings.TrimSpace(schema.UsedRange.A1) == "" {
		return 0, nil, nil, fmt.Errorf("used range not available")
	}
	startRow, startCol, endRow, _, ok := sheetMasterParseRangeA1(schema.UsedRange.A1)
	if !ok {
		return 0, nil, nil, fmt.Errorf("invalid used range: %s", schema.UsedRange.A1)
	}
	rangeA1 := fmt.Sprintf("A1:C%d", endRow+1)
	if startRow > 0 || startCol > 0 {
		rangeA1 = fmt.Sprintf("A%d:C%d", startRow+1, endRow+1)
	}

	cells, err := sheetMasterGetCells(ctx, cfg, rangeA1)
	if err != nil {
		return 0, nil, nil, err
	}

	grid := buildGridFromValues(cells.Values, cells.Start.Row, cells.Start.Col)
	headerRow, nameCol, qtyCol, _ := sheetMasterFindInventoryHeader(grid)

	edits, updatedNames := sheetMasterBuildInventoryAdjustEditsWithNames(adjustments, grid, headerRow, nameCol, qtyCol)
	if len(edits) == 0 {
		return 0, nil, nil, nil
	}

	updated, err := sheetMasterPatchCells(ctx, cfg, edits)
	if err != nil {
		return 0, nil, nil, err
	}
	return updated, edits, updatedNames, nil
}

func (h *BotHandler) refreshCatalogFromSheetMasterLocked(ctx context.Context) error {
	selectedID, _ := sheetMasterSelectedFileID()
	if cfg, err := h.resolveSheetMasterConfig(); err == nil {
		if selectedID != 0 {
			cfg.FileID = strconv.FormatUint(uint64(selectedID), 10)
		}
		if exp, apiErr := sheetMasterExportFromAPI(ctx, cfg); apiErr == nil {
			_, upErr := h.adminUseCase.UploadCatalogSystem(ctx, exp.XLSXBytes, exp.Filename)
			return upErr
		} else {
			log.Printf("[inventory] api export error: %v", apiErr)
		}
	}

	if exp, err := sheetMasterExportFromDB(ctx, selectedID); err == nil {
		_, upErr := h.adminUseCase.UploadCatalogSystem(ctx, exp.XLSXBytes, exp.Filename)
		return upErr
	} else {
		log.Printf("[inventory] db export error: %v", err)
	}

	return fmt.Errorf("sheetmaster export failed")
}

func sheetMasterPrepareInventoryState(stateJSON []byte) (map[string]any, map[string]any, map[int]map[int]string, int, int, int, error) {
	var state map[string]any
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return nil, nil, nil, 0, 0, 0, fmt.Errorf("decode sheet state: %w", err)
	}
	dataAny, ok := state["data"].(map[string]any)
	if !ok || dataAny == nil {
		dataAny = map[string]any{}
		state["data"] = dataAny
	}

	grid := buildGridFromData(dataAny)
	headerRow, nameCol, qtyCol, found := sheetMasterFindInventoryHeader(grid)
	if !found {
		headerRow = 0
		nameCol = 0
		qtyCol = 1
	}

	return state, dataAny, grid, headerRow, nameCol, qtyCol, nil
}

func sheetMasterBuildInventoryEdits(items []string, grid map[int]map[int]string, headerRow, nameCol, qtyCol int) []sheetMasterCellEdit {
	adjustments := buildInventoryAdjustments(items, -1)
	return sheetMasterBuildInventoryAdjustEdits(adjustments, grid, headerRow, nameCol, qtyCol)
}

func sheetMasterBuildInventoryAdjustEdits(adjustments []inventoryAdjustment, grid map[int]map[int]string, headerRow, nameCol, qtyCol int) []sheetMasterCellEdit {
	edits, _ := sheetMasterBuildInventoryAdjustEditsWithNames(adjustments, grid, headerRow, nameCol, qtyCol)
	return edits
}

func sheetMasterBuildInventoryAdjustEditsWithNames(adjustments []inventoryAdjustment, grid map[int]map[int]string, headerRow, nameCol, qtyCol int) ([]sheetMasterCellEdit, []string) {
	rowNames := make(map[int]string)
	rowRaw := make(map[int]string)
	for row, cols := range grid {
		if row <= headerRow {
			continue
		}
		name := strings.TrimSpace(cols[nameCol])
		if name == "" {
			continue
		}
		rowNames[row] = normalizeInventoryName(name)
		rowRaw[row] = name
	}

	var edits []sheetMasterCellEdit
	var updatedNames []string
	seen := make(map[string]struct{})
	for _, adj := range adjustments {
		if adj.Delta == 0 {
			continue
		}
		itemNorm := normalizeInventoryName(adj.Name)
		if itemNorm == "" {
			continue
		}
		row := findInventoryRow(itemNorm, rowNames)
		if row < 0 {
			continue
		}
		qtyRaw := strings.TrimSpace(grid[row][qtyCol])
		qty, ok := parseInventoryQuantity(qtyRaw)
		if !ok {
			continue
		}
		newQty := qty + adj.Delta
		if adj.Delta < 0 && qty <= 0 {
			continue
		}
		if newQty < 0 {
			newQty = 0
		}
		if newQty == qty {
			continue
		}
		gridRow := grid[row]
		if gridRow == nil {
			gridRow = map[int]string{}
			grid[row] = gridRow
		}
		gridRow[qtyCol] = strconv.Itoa(newQty)
		edits = append(edits, sheetMasterCellEdit{
			Row:   row,
			Col:   qtyCol,
			Value: strconv.Itoa(newQty),
		})
		if raw, ok := rowRaw[row]; ok {
			name := strings.TrimSpace(raw)
			if name != "" {
				norm := normalizeInventoryName(name)
				if _, exists := seen[norm]; !exists {
					updatedNames = append(updatedNames, name)
					seen[norm] = struct{}{}
				}
			}
		}
	}
	return edits, updatedNames
}

func parseInventoryQuantity(raw string) (int, bool) {
	// Keep only digits and separators.
	var b strings.Builder
	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == ' ' {
			b.WriteRune(r)
		}
	}
	clean := strings.TrimSpace(b.String())
	if clean == "" {
		return 0, false
	}
	clean = strings.ReplaceAll(clean, " ", "")

	dot := strings.LastIndex(clean, ".")
	comma := strings.LastIndex(clean, ",")

	switch {
	case dot >= 0 && comma >= 0:
		if dot > comma {
			clean = strings.ReplaceAll(clean, ",", "")
		} else {
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.ReplaceAll(clean, ",", ".")
		}
	case dot >= 0:
		if strings.Count(clean, ".") > 1 {
			clean = strings.ReplaceAll(clean, ".", "")
		} else if after := clean[dot+1:]; len(after) == 3 {
			clean = strings.ReplaceAll(clean, ".", "")
		}
	case comma >= 0:
		if strings.Count(clean, ",") > 1 {
			clean = strings.ReplaceAll(clean, ",", "")
		} else if after := clean[comma+1:]; len(after) == 3 {
			clean = strings.ReplaceAll(clean, ",", "")
		} else {
			clean = strings.ReplaceAll(clean, ",", ".")
		}
	}

	val, err := strconv.ParseFloat(clean, 64)
	if err != nil || val < 0 {
		return 0, false
	}
	return int(val), true
}

func applyInventoryEdits(dataAny map[string]any, edits []sheetMasterCellEdit) {
	for _, edit := range edits {
		cellID := fmt.Sprintf("%d,%d", edit.Row, edit.Col)
		cellAny, _ := dataAny[cellID].(map[string]any)
		if cellAny == nil {
			cellAny = map[string]any{}
		}
		cellAny["value"] = edit.Value
		delete(cellAny, "computed")
		dataAny[cellID] = cellAny
	}
}

func buildGridFromData(dataAny map[string]any) map[int]map[int]string {
	grid := make(map[int]map[int]string)
	for key, cell := range dataAny {
		row, col, ok := sheetMasterParseCellKey(key)
		if !ok {
			continue
		}
		cellAny, ok := cell.(map[string]any)
		if !ok {
			continue
		}
		val := strings.TrimSpace(sheetMasterStateCellRawValue(cellAny))
		if val == "" {
			continue
		}
		rowMap := grid[row]
		if rowMap == nil {
			rowMap = map[int]string{}
			grid[row] = rowMap
		}
		rowMap[col] = val
	}
	return grid
}

func buildGridFromValues(values [][]string, startRow, startCol int) map[int]map[int]string {
	grid := make(map[int]map[int]string)
	for r, row := range values {
		for c, val := range row {
			v := strings.TrimSpace(val)
			if v == "" {
				continue
			}
			rowIdx := startRow + r
			colIdx := startCol + c
			rowMap := grid[rowIdx]
			if rowMap == nil {
				rowMap = map[int]string{}
				grid[rowIdx] = rowMap
			}
			rowMap[colIdx] = v
		}
	}
	return grid
}

func sheetMasterFindInventoryHeader(grid map[int]map[int]string) (headerRow, nameCol, qtyCol int, ok bool) {
	rows := sortedKeys(grid)
	for _, row := range rows {
		cols := grid[row]
		if cols == nil {
			continue
		}
		nameCol = -1
		qtyCol = -1
		for col, raw := range cols {
			norm := normalizeHeaderValue(raw)
			if nameCol == -1 && containsAny(norm, inventoryNameHeaders) {
				nameCol = col
			}
			if qtyCol == -1 && containsAny(norm, inventoryQtyHeaders) {
				qtyCol = col
			}
		}
		if nameCol >= 0 && qtyCol >= 0 {
			return row, nameCol, qtyCol, true
		}
	}
	return 0, 0, 1, false
}

func normalizeHeaderValue(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("_", " ", "-", " ", "—", " ", "–", " ", ".", " ").Replace(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func normalizeInventoryName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.Trim(s, "\"'`")
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.ReplaceAll(s, "—", "-")
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ToLower(s)
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	s = strings.Trim(s, " -–—:.,")
	return s
}

func containsAny(s string, kws []string) bool {
	for _, kw := range kws {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[int]map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func findInventoryRow(itemNorm string, rowNames map[int]string) int {
	if itemNorm == "" {
		return -1
	}
	for row, name := range rowNames {
		if itemNorm == name {
			return row
		}
	}
	for row, name := range rowNames {
		if name == "" {
			continue
		}
		if strings.Contains(name, itemNorm) || strings.Contains(itemNorm, name) {
			return row
		}
	}
	return -1
}

func sheetMasterParseRangeA1(rng string) (startRow, startCol, endRow, endCol int, ok bool) {
	rng = strings.TrimSpace(rng)
	if rng == "" {
		return 0, 0, 0, 0, false
	}
	parts := strings.Split(rng, ":")
	if len(parts) == 1 {
		row, col, ok := sheetMasterA1ToRowCol(parts[0])
		if !ok {
			return 0, 0, 0, 0, false
		}
		return row, col, row, col, true
	}
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}
	sr, sc, ok1 := sheetMasterA1ToRowCol(parts[0])
	er, ec, ok2 := sheetMasterA1ToRowCol(parts[1])
	if !ok1 || !ok2 {
		return 0, 0, 0, 0, false
	}
	return sr, sc, er, ec, true
}

type realtimeInternalCellEdit struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Value string `json:"value"`
}

type realtimeInternalBatchEditRequest struct {
	Edits []realtimeInternalCellEdit `json:"edits"`
}

func notifyRealtimeBatchEdits(ctx context.Context, sheetID uint, edits []sheetMasterCellEdit) error {
	if sheetID == 0 {
		return fmt.Errorf("realtime sheet_id is required")
	}

	payload := realtimeInternalBatchEditRequest{Edits: make([]realtimeInternalCellEdit, 0, len(edits))}
	for _, edit := range edits {
		if edit.Row < 0 || edit.Col < 0 {
			continue
		}
		payload.Edits = append(payload.Edits, realtimeInternalCellEdit{
			Row:   edit.Row,
			Col:   edit.Col,
			Value: edit.Value,
		})
	}
	if len(payload.Edits) == 0 {
		return nil
	}

	secret := strings.TrimSpace(os.Getenv("REALTIME_INTERNAL_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("INTERNAL_API_SECRET"))
	}
	if secret == "" {
		secret = "converter-internal-dev-secret"
	}

	addCandidate := func(list []string, seen map[string]struct{}, raw string) ([]string, map[string]struct{}) {
		base := normalizeRealtimeInternalBase(raw)
		if base == "" {
			return list, seen
		}
		if _, ok := seen[base]; ok {
			return list, seen
		}
		seen[base] = struct{}{}
		return append(list, base), seen
	}

	candidates := []string{}
	seen := make(map[string]struct{})

	baseURL := strings.TrimSpace(os.Getenv("REALTIME_INTERNAL_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("SHEETMASTER_REALTIME_INTERNAL_URL"))
	}
	if baseURL != "" {
		candidates, seen = addCandidate(candidates, seen, baseURL)
	} else {
		candidates, seen = addCandidate(candidates, seen, "http://backend-elixir:4000")
		candidates, seen = addCandidate(candidates, seen, "http://localhost:4000")
		candidates, seen = addCandidate(candidates, seen, "http://host.docker.internal:4000")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var lastErr error
	client := &http.Client{Timeout: 3 * time.Second}
	for _, base := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/spreadsheets/%d/batch_edit", base, sheetID), bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Secret", secret)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				lastErr = fmt.Errorf("realtime status=%d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
				return
			}
			lastErr = nil
		}()
		if lastErr == nil {
			return nil
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("realtime internal request failed")
	}
	return lastErr
}

func normalizeRealtimeInternalBase(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	trimmed := strings.TrimRight(raw, "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if strings.HasSuffix(trimmed, "/api/internal") || strings.HasSuffix(trimmed, "/internal") {
			return trimmed
		}
		return trimmed + "/api/internal"
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case strings.HasSuffix(path, "/api/internal") || strings.HasSuffix(path, "/internal"):
		// already normalized
	case path == "":
		path = "/api/internal"
	case strings.HasSuffix(path, "/api"):
		path = path + "/internal"
	default:
		path = path + "/api/internal"
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}
