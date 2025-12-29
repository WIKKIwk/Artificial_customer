package services

import (
	"converter-backend/internal/models"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&models.SheetFile{}, &models.SpreadsheetData{})
	return db
}

func Test_SaveFile(t *testing.T) {
	db := setupTestDB()
	service := &SpreadsheetService{DB: db}

	state := json.RawMessage(`{"data": {"A1": {"value": "test"}}}`)

	file, err := service.SaveFile(1, "Test File", state)

	assert.Nil(t, err)
	assert.NotNil(t, file)
	assert.Equal(t, "Test File", file.Name)
	assert.Equal(t, uint(1), file.UserID)
}

func Test_ListFilesWithPagination(t *testing.T) {
	db := setupTestDB()
	service := &SpreadsheetService{DB: db}

	// Create some test files
	for i := 1; i <= 10; i++ {
		state := json.RawMessage(`{}`)
		service.SaveFile(1, "File "+string(rune(i+'0')), state)
	}

	// Test pagination
	files, total, err := service.ListFilesWithPagination(1, 0, 5)

	assert.Nil(t, err)
	assert.Equal(t, int64(10), total)
	assert.Equal(t, 5, len(files))
}

func Test_DeleteFile(t *testing.T) {
	db := setupTestDB()
	service := &SpreadsheetService{DB: db}

	state := json.RawMessage(`{}`)
	file, _ := service.SaveFile(1, "Test File", state)

	// Delete the file
	err := service.DeleteFile(1, file.ID)
	assert.Nil(t, err)

	// Try to get the deleted file
	_, err = service.GetFile(1, file.ID)
	assert.NotNil(t, err)
}

func Test_GetFile(t *testing.T) {
	db := setupTestDB()
	service := &SpreadsheetService{DB: db}

	state := json.RawMessage(`{"data": {"A1": {"value": "test"}}}`)
	savedFile, _ := service.SaveFile(1, "Test File", state)

	// Get the file
	file, err := service.GetFile(1, savedFile.ID)

	assert.Nil(t, err)
	assert.NotNil(t, file)
	assert.Equal(t, "Test File", file.Name)
	assert.Equal(t, savedFile.ID, file.ID)
}

func Test_GetFile_WrongUser(t *testing.T) {
	db := setupTestDB()
	service := &SpreadsheetService{DB: db}

	state := json.RawMessage(`{}`)
	savedFile, _ := service.SaveFile(1, "Test File", state)

	// Try to get file with wrong user ID
	_, err := service.GetFile(999, savedFile.ID)

	assert.NotNil(t, err)
}
