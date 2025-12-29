package services

import (
	"converter-backend/internal/models"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SpreadsheetService struct {
	DB *gorm.DB
}

type CellEdit struct {
	Row   int
	Col   int
	Value string
}

type AccessibleFileMeta struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	UpdatedAt  time.Time `json:"updated_at"`
	OwnerID    uint      `json:"owner_id"`
	AccessRole string    `json:"access_role"` // owner|editor|viewer
}

func NewSpreadsheetService(dsn string) *SpreadsheetService {
	var db *gorm.DB
	var err error
	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, pingErr := db.DB()
			if pingErr == nil {
				pingErr = sqlDB.Ping()
			}
			if pingErr == nil {
				break
			}
			err = pingErr
		}
		log.Printf("SpreadsheetService DB connection attempt %d/%d failed: %v", attempt, maxAttempts, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("SpreadsheetService failed to connect to database after %d attempts: %v", maxAttempts, err)
	}

	db.AutoMigrate(&models.SpreadsheetData{}, &models.SheetFile{}, &models.SheetFileShare{})
	return &SpreadsheetService{DB: db}
}

func (s *SpreadsheetService) ProcessAndSave(file multipart.File, filename string) ([][]string, error) {
	// Save temp file for excelize (it prefers file paths)
	// In a real prod env, we might stream or use a better temp handling
	tempFile, err := os.CreateTemp("", "upload-*.xlsx")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempFile.Name())

	// Copy uploaded file to temp
	// Reset file pointer first just in case
	file.Seek(0, 0)
	_, err = tempFile.ReadFrom(file)
	if err != nil {
		return nil, err
	}
	tempFile.Close() // Close so excelize can open it

	var rows [][]string
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".csv" {
		rows, err = readCSVRows(tempFile.Name())
		if err != nil {
			return nil, err
		}
	} else {
		f, err := excelize.OpenFile(tempFile.Name())
		if err != nil {
			return nil, err
		}
		defer f.Close()

		sheetName := f.GetSheetName(0)
		rows, err = f.GetRows(sheetName)
		if err != nil {
			return nil, err
		}
	}

	records := make([]models.SpreadsheetData, 0, len(rows))
	for _, row := range rows {
		jsonData, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal row: %w", err)
		}
		records = append(records, models.SpreadsheetData{
			Filename: filename,
			RowData:  jsonData,
		})
	}

	if len(records) > 0 {
		if err := s.DB.CreateInBatches(records, 500).Error; err != nil {
			return nil, fmt.Errorf("failed to save rows: %w", err)
		}
	}

	return rows, nil
}

// SaveFile persists a sheet state for a user.
func (s *SpreadsheetService) SaveFile(userID uint, name string, state json.RawMessage) (*models.SheetFile, error) {
	file := &models.SheetFile{
		UserID: userID,
		Name:   name,
		State:  state,
	}
	if err := s.DB.Save(file).Error; err != nil {
		return nil, err
	}
	return file, nil
}

// ListFiles returns user's files sorted by updated_at desc.
func (s *SpreadsheetService) ListFiles(userID uint) ([]models.SheetFile, error) {
	var files []models.SheetFile
	err := s.DB.Where("user_id = ?", userID).Order("updated_at desc").Find(&files).Error
	return files, err
}

// GetFile fetches a file by id/user.
func (s *SpreadsheetService) GetFile(userID, fileID uint) (*models.SheetFile, error) {
	var file models.SheetFile
	err := s.DB.Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// GetFileAccess returns a file if the user is the owner or has an explicit share.
// Role is one of: owner|editor|viewer.
func (s *SpreadsheetService) GetFileAccess(userID, fileID uint) (*models.SheetFile, string, error) {
	if file, err := s.GetFile(userID, fileID); err == nil {
		return file, "owner", nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}

	var share models.SheetFileShare
	if err := s.DB.Where("file_id = ? AND user_id = ?", fileID, userID).First(&share).Error; err != nil {
		return nil, "", err
	}

	var file models.SheetFile
	if err := s.DB.Where("id = ?", fileID).First(&file).Error; err != nil {
		return nil, "", err
	}

	role := strings.ToLower(strings.TrimSpace(share.Role))
	switch role {
	case "editor", "viewer":
	default:
		role = "viewer"
	}

	return &file, role, nil
}

// ListAccessibleFiles returns both owned and shared file metas for a user.
func (s *SpreadsheetService) ListAccessibleFiles(userID uint) ([]AccessibleFileMeta, error) {
	var ownedFiles []models.SheetFile
	if err := s.DB.Select("id", "name", "updated_at", "user_id").Where("user_id = ?", userID).Find(&ownedFiles).Error; err != nil {
		return nil, err
	}

	metas := make([]AccessibleFileMeta, 0, len(ownedFiles))
	seen := make(map[uint]struct{}, len(ownedFiles))

	for _, file := range ownedFiles {
		metas = append(metas, AccessibleFileMeta{
			ID:         file.ID,
			Name:       file.Name,
			UpdatedAt:  file.UpdatedAt,
			OwnerID:    file.UserID,
			AccessRole: "owner",
		})
		seen[file.ID] = struct{}{}
	}

	var shares []models.SheetFileShare
	if err := s.DB.Where("user_id = ?", userID).Find(&shares).Error; err != nil {
		return nil, err
	}
	if len(shares) == 0 {
		return metas, nil
	}

	fileIDs := make([]uint, 0, len(shares))
	roleByFile := make(map[uint]string, len(shares))
	for _, share := range shares {
		fileIDs = append(fileIDs, share.FileID)
		role := strings.ToLower(strings.TrimSpace(share.Role))
		if role != "editor" && role != "viewer" {
			role = "viewer"
		}
		roleByFile[share.FileID] = role
	}

	var sharedFiles []models.SheetFile
	if err := s.DB.Select("id", "name", "updated_at", "user_id").Where("id IN ?", fileIDs).Find(&sharedFiles).Error; err != nil {
		return nil, err
	}

	for _, file := range sharedFiles {
		if _, ok := seen[file.ID]; ok {
			continue
		}
		role := roleByFile[file.ID]
		if role == "" {
			role = "viewer"
		}
		metas = append(metas, AccessibleFileMeta{
			ID:         file.ID,
			Name:       file.Name,
			UpdatedAt:  file.UpdatedAt,
			OwnerID:    file.UserID,
			AccessRole: role,
		})
		seen[file.ID] = struct{}{}
	}

	sort.Slice(metas, func(i, j int) bool {
		if metas[i].UpdatedAt.Equal(metas[j].UpdatedAt) {
			return metas[i].ID > metas[j].ID
		}
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})

	return metas, nil
}

// UpdateFileName updates only name and updated_at for an existing file.
func (s *SpreadsheetService) UpdateFileName(userID, fileID uint, name string) error {
	return s.DB.Model(&models.SheetFile{}).
		Where("id = ? AND user_id = ?", fileID, userID).
		Updates(map[string]interface{}{"name": name, "updated_at": time.Now()}).Error
}

// DeleteFile deletes a file by ID for a specific user
func (s *SpreadsheetService) DeleteFile(userID, fileID uint) error {
	result := s.DB.Where("id = ? AND user_id = ?", fileID, userID).Delete(&models.SheetFile{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// PatchFileCells applies a set of cell edits to an existing file state.
// It updates only state.data[*].value (and clears computed) while preserving other state fields.
func (s *SpreadsheetService) PatchFileCells(fileID uint, edits []CellEdit) (*models.SheetFile, int, error) {
	if len(edits) == 0 {
		return nil, 0, fmt.Errorf("no edits provided")
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var file models.SheetFile
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", fileID).
		First(&file).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	var state map[string]any
	if err := json.Unmarshal(file.State, &state); err != nil {
		tx.Rollback()
		return nil, 0, fmt.Errorf("failed to decode file state: %w", err)
	}

	dataAny, ok := state["data"].(map[string]any)
	if !ok || dataAny == nil {
		dataAny = map[string]any{}
	}

	maxRow := -1
	for _, edit := range edits {
		if edit.Row < 0 || edit.Col < 0 {
			continue
		}

		cellID := fmt.Sprintf("%d,%d", edit.Row, edit.Col)

		cellAny, _ := dataAny[cellID].(map[string]any)
		if cellAny == nil {
			cellAny = map[string]any{}
		}

		cellAny["value"] = edit.Value
		delete(cellAny, "computed")
		dataAny[cellID] = cellAny

		if edit.Row > maxRow {
			maxRow = edit.Row
		}
	}

	state["data"] = dataAny

	if maxRow >= 0 {
		switch current := state["rowCount"].(type) {
		case float64:
			if maxRow+1 > int(current) {
				state["rowCount"] = maxRow + 1
			}
		case int:
			if maxRow+1 > current {
				state["rowCount"] = maxRow + 1
			}
		}
	}

	nextState, err := json.Marshal(state)
	if err != nil {
		tx.Rollback()
		return nil, 0, fmt.Errorf("failed to encode file state: %w", err)
	}

	file.State = nextState
	if err := tx.Save(&file).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return &file, len(edits), nil
}

// ListFilesWithPagination returns user's files with pagination
func (s *SpreadsheetService) ListFilesWithPagination(userID uint, offset, limit int) ([]models.SheetFile, int64, error) {
	var files []models.SheetFile
	var total int64

	// Get total count
	if err := s.DB.Model(&models.SheetFile{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := s.DB.Where("user_id = ?", userID).
		Order("updated_at desc").
		Offset(offset).
		Limit(limit).
		Find(&files).Error

	return files, total, err
}

// readCSVRows loads a CSV file from disk into [][]string while preserving empty fields.
func readCSVRows(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // allow variable columns per row

	var rows [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, record)
	}
	return rows, nil
}
