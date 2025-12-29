package telegram

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type sheetMasterDBFile struct {
	ID        uint
	Name      string
	StateJSON json.RawMessage
	UpdatedAt time.Time
}

type sheetMasterDBFileMeta struct {
	ID        uint
	Name      string
	UpdatedAt time.Time
}

type sheetMasterDBExport struct {
	File        sheetMasterDBFile
	UsedRangeA1 string
	XLSXBytes   []byte
	Filename    string
}

var errSheetMasterDBNotConfigured = errors.New("sheetmaster db not configured")

const sheetMasterDBSelectionFile = "data/sheetmaster_db_selection.json"

type sheetMasterDBSelection struct {
	FileID    uint      `json:"file_id"`
	Name      string    `json:"name,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func loadSheetMasterDBSelection() (sheetMasterDBSelection, bool, error) {
	b, err := os.ReadFile(sheetMasterDBSelectionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return sheetMasterDBSelection{}, false, nil
		}
		return sheetMasterDBSelection{}, false, err
	}
	var sel sheetMasterDBSelection
	if err := json.Unmarshal(b, &sel); err != nil {
		return sheetMasterDBSelection{}, false, err
	}
	if sel.FileID == 0 {
		return sel, false, nil
	}
	return sel, true, nil
}

func saveSheetMasterDBSelection(sel sheetMasterDBSelection) error {
	if sel.FileID == 0 {
		return fmt.Errorf("file_id is required")
	}
	dir := filepath.Dir(sheetMasterDBSelectionFile)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(sel, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sheetMasterDBSelectionFile, b, 0o600)
}

func clearSheetMasterDBSelection() error {
	if err := os.Remove(sheetMasterDBSelectionFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sheetMasterSelectedFileID() (uint, bool) {
	sel, ok, err := loadSheetMasterDBSelection()
	if err != nil || !ok || sel.FileID == 0 {
		return 0, false
	}
	return sel.FileID, true
}

func sheetMasterDBDSNFromEnv() (string, error) {
	if dsn := strings.TrimSpace(getenvAny("SHEETMASTER_DB_DSN", "SHEETMASTER_DB_DSN_URL")); dsn != "" {
		return dsn, nil
	}
	// Default: docker-compose.database.yml
	return "host=converter_db user=user password=password dbname=converter_db port=5432 sslmode=disable", nil
}

func getenvAny(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func sheetMasterOpenDB(ctx context.Context, dsn string) (*sql.DB, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errSheetMasterDBNotConfigured
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(2 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func sheetMasterExportLatestFromDB(ctx context.Context) (sheetMasterDBExport, error) {
	return sheetMasterExportFromDB(ctx, 0)
}

func sheetMasterExportFromAPI(ctx context.Context, cfg sheetMasterConfig) (sheetMasterDBExport, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.FileID) == "" {
		return sheetMasterDBExport{}, fmt.Errorf("sheetmaster api not configured")
	}

	file, err := sheetMasterGetFileState(ctx, cfg)
	if err != nil {
		return sheetMasterDBExport{}, err
	}

	xlsxBytes, usedRangeA1, err := sheetMasterDBFileToXLSX(file)
	if err != nil {
		return sheetMasterDBExport{}, err
	}

	filename := sheetMasterSafeExcelFilename(file.Name, "sheetmaster.xlsx")

	return sheetMasterDBExport{
		File:        file,
		UsedRangeA1: usedRangeA1,
		XLSXBytes:   xlsxBytes,
		Filename:    filename,
	}, nil
}

func sheetMasterExportFromDB(ctx context.Context, fileID uint) (sheetMasterDBExport, error) {
	dsn, err := sheetMasterDBDSNFromEnv()
	if err != nil {
		return sheetMasterDBExport{}, err
	}

	var file sheetMasterDBFile
	if fileID == 0 {
		file, err = sheetMasterGetLatestFileFromDB(ctx, dsn)
	} else {
		file, err = sheetMasterGetFileFromDB(ctx, dsn, fileID)
	}
	if err != nil {
		return sheetMasterDBExport{}, err
	}

	xlsxBytes, usedRangeA1, err := sheetMasterDBFileToXLSX(file)
	if err != nil {
		return sheetMasterDBExport{}, err
	}

	filename := sheetMasterSafeExcelFilename(file.Name, "sheetmaster.xlsx")

	return sheetMasterDBExport{
		File:        file,
		UsedRangeA1: usedRangeA1,
		XLSXBytes:   xlsxBytes,
		Filename:    filename,
	}, nil
}

func sheetMasterDBFileUsedRangeA1(file sheetMasterDBFile) (string, bool, error) {
	var state map[string]any
	if err := json.Unmarshal(file.StateJSON, &state); err != nil {
		return "", false, fmt.Errorf("decode sheet state: %w", err)
	}
	dataAny, _ := state["data"].(map[string]any)
	if dataAny == nil {
		return "", false, nil
	}
	minRow, maxRow, minCol, maxCol, ok := sheetMasterComputeUsedBounds(dataAny)
	if !ok {
		return "", false, nil
	}
	return fmt.Sprintf("%s%d:%s%d",
		sheetMasterColToLabel(minCol), minRow+1,
		sheetMasterColToLabel(maxCol), maxRow+1,
	), true, nil
}

func sheetMasterSafeExcelFilename(name, fallback string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return fallback
	}
	n = strings.NewReplacer("/", "_", "\\", "_", ":", "_", "\n", "_", "\r", "_", "\t", "_").Replace(n)
	n = strings.TrimSpace(n)
	if n == "" {
		return fallback
	}
	// Avoid extremely long filenames.
	if len([]rune(n)) > 80 {
		n = string([]rune(n)[:80])
	}
	n = strings.TrimSuffix(n, ".xlsx")
	n = strings.TrimSuffix(n, ".xls")
	n = strings.TrimSpace(n)
	if n == "" {
		return fallback
	}
	return n + ".xlsx"
}

func sheetMasterGetLatestFileFromDB(ctx context.Context, dsn string) (sheetMasterDBFile, error) {
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return sheetMasterDBFile{}, err
	}
	defer db.Close()

	var f sheetMasterDBFile
	row := db.QueryRowContext(ctx, `
SELECT id, name, state, updated_at
FROM sheet_files
ORDER BY updated_at DESC, id DESC
LIMIT 1`)
	if err := row.Scan(&f.ID, &f.Name, &f.StateJSON, &f.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sheetMasterDBFile{}, fmt.Errorf("sheet_files table is empty")
		}
		return sheetMasterDBFile{}, err
	}
	return f, nil
}

func sheetMasterGetFileFromDB(ctx context.Context, dsn string, fileID uint) (sheetMasterDBFile, error) {
	if fileID == 0 {
		return sheetMasterDBFile{}, fmt.Errorf("file_id is required")
	}
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return sheetMasterDBFile{}, err
	}
	defer db.Close()

	var f sheetMasterDBFile
	row := db.QueryRowContext(ctx, `
SELECT id, name, state, updated_at
FROM sheet_files
WHERE id = $1
LIMIT 1`, fileID)
	if err := row.Scan(&f.ID, &f.Name, &f.StateJSON, &f.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sheetMasterDBFile{}, fmt.Errorf("file not found")
		}
		return sheetMasterDBFile{}, err
	}
	return f, nil
}

func sheetMasterListFilesFromDB(ctx context.Context, limit int) ([]sheetMasterDBFileMeta, error) {
	if limit <= 0 {
		limit = 20
	}
	dsn, err := sheetMasterDBDSNFromEnv()
	if err != nil {
		return nil, err
	}
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
SELECT id, name, updated_at
FROM sheet_files
ORDER BY updated_at DESC, id DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sheetMasterDBFileMeta
	for rows.Next() {
		var f sheetMasterDBFileMeta
		if err := rows.Scan(&f.ID, &f.Name, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func sheetMasterDBFileToXLSX(file sheetMasterDBFile) ([]byte, string, error) {
	var state map[string]any
	if err := json.Unmarshal(file.StateJSON, &state); err != nil {
		return nil, "", fmt.Errorf("decode sheet state: %w", err)
	}
	dataAny, _ := state["data"].(map[string]any)
	if dataAny == nil {
		return nil, "", fmt.Errorf("sheet state has no data")
	}

	minRow, maxRow, minCol, maxCol, ok := sheetMasterComputeUsedBounds(dataAny)
	if !ok {
		return nil, "", fmt.Errorf("sheet file is empty")
	}

	usedRangeA1 := fmt.Sprintf("%s%d:%s%d",
		sheetMasterColToLabel(minCol), minRow+1,
		sheetMasterColToLabel(maxCol), maxRow+1,
	)

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	for key, cell := range dataAny {
		row, col, ok := sheetMasterParseCellKey(key)
		if !ok {
			continue
		}
		cellAny, ok := cell.(map[string]any)
		if !ok {
			continue
		}
		val := strings.TrimSpace(sheetMasterStateCellExportValue(cellAny))
		if val == "" {
			continue
		}

		// Crop to used range so exported Excel starts at A1.
		row -= minRow
		col -= minCol
		if row < 0 || col < 0 {
			continue
		}

		cellName, err := excelize.CoordinatesToCellName(col+1, row+1)
		if err != nil {
			return nil, "", err
		}
		if err := f.SetCellValue(sheet, cellName, val); err != nil {
			return nil, "", err
		}
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), usedRangeA1, nil
}

func sheetMasterComputeUsedBounds(dataAny map[string]any) (minRow, maxRow, minCol, maxCol int, ok bool) {
	const maxInt = int(^uint(0) >> 1)
	minRow = maxInt
	minCol = maxInt
	maxRow = -1
	maxCol = -1

	for key, cell := range dataAny {
		cellAny, ok := cell.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(sheetMasterStateCellRawValue(cellAny)) == "" {
			continue
		}
		r, c, ok := sheetMasterParseCellKey(key)
		if !ok || r < 0 || c < 0 {
			continue
		}
		if r < minRow {
			minRow = r
		}
		if c < minCol {
			minCol = c
		}
		if r > maxRow {
			maxRow = r
		}
		if c > maxCol {
			maxCol = c
		}
	}

	if maxRow < 0 || maxCol < 0 || minRow == maxInt || minCol == maxInt {
		return 0, 0, 0, 0, false
	}
	return minRow, maxRow, minCol, maxCol, true
}

func sheetMasterStateCellRawValue(cellAny map[string]any) string {
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

func sheetMasterStateCellExportValue(cellAny map[string]any) string {
	raw := sheetMasterStateCellRawValue(cellAny)
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "=") {
		if v, ok := cellAny["computed"]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return t
				}
			default:
				return fmt.Sprint(t)
			}
		}
	}
	return raw
}

func sheetMasterParseCellKey(key string) (row, col int, ok bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return 0, 0, false
	}
	if strings.Contains(key, ",") {
		parts := strings.Split(key, ",")
		if len(parts) != 2 {
			return 0, 0, false
		}
		r, errR := strconv.Atoi(strings.TrimSpace(parts[0]))
		c, errC := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errR != nil || errC != nil {
			return 0, 0, false
		}
		return r, c, true
	}
	return sheetMasterA1ToRowCol(key)
}

func sheetMasterA1ToRowCol(cell string) (row int, col int, ok bool) {
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

func sheetMasterColToLabel(col int) string {
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
