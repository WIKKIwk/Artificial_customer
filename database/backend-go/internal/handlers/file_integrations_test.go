package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"converter-backend/internal/models"
	"converter-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFileHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.SheetFile{}, &models.SheetFileShare{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func Test_a1RangeToBounds(t *testing.T) {
	minR, maxR, minC, maxC, ok := a1RangeToBounds("A1:D20")
	assert.True(t, ok)
	assert.Equal(t, 0, minR)
	assert.Equal(t, 19, maxR)
	assert.Equal(t, 0, minC)
	assert.Equal(t, 3, maxC)

	minR, maxR, minC, maxC, ok = a1RangeToBounds("B2")
	assert.True(t, ok)
	assert.Equal(t, 1, minR)
	assert.Equal(t, 1, maxR)
	assert.Equal(t, 1, minC)
	assert.Equal(t, 1, maxC)

	_, _, _, _, ok = a1RangeToBounds("not-a-range")
	assert.False(t, ok)
}

func Test_FileHandler_GetCells_Grid(t *testing.T) {
	db := setupFileHandlerTestDB(t)
	service := &services.SpreadsheetService{DB: db}
	handler := NewFileHandler(service)

	state := json.RawMessage(`{
		"data": {
			"0,0": {"value":"Product"},
			"0,1": {"value":"Category"},
			"1,0": {"value":"CPU"},
			"1,1": {"value":"Components"}
		},
		"rowCount": 100
	}`)
	file := models.SheetFile{UserID: 1, Name: "Test", State: state}
	assert.NoError(t, db.Create(&file).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})
	router.GET("/files/:id/cells", handler.GetCells)

	req, _ := http.NewRequest("GET", "/files/1/cells?range=A1:B2&format=grid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Values [][]string `json:"values"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, [][]string{{"Product", "Category"}, {"CPU", "Components"}}, resp.Values)
}

func Test_FileHandler_GetSchema(t *testing.T) {
	db := setupFileHandlerTestDB(t)
	service := &services.SpreadsheetService{DB: db}
	handler := NewFileHandler(service)

	state := json.RawMessage(`{
		"data": {
			"0,0": {"value":"Product"},
			"0,1": {"value":"Category"},
			"1,0": {"value":"CPU"},
			"1,1": {"value":"Components"}
		},
		"rowCount": 100
	}`)
	file := models.SheetFile{UserID: 1, Name: "Test", State: state}
	assert.NoError(t, db.Create(&file).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})
	router.GET("/files/:id/schema", handler.GetSchema)

	req, _ := http.NewRequest("GET", "/files/1/schema", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		UsedRange struct {
			A1 string `json:"a1"`
		} `json:"used_range"`
		HeaderRow int `json:"header_row"`
		Columns   []struct {
			Col    int    `json:"col"`
			Label  string `json:"label"`
			Header string `json:"header"`
		} `json:"columns"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "A1:B2", resp.UsedRange.A1)
	assert.Equal(t, 0, resp.HeaderRow)
	assert.Len(t, resp.Columns, 2)
	assert.Equal(t, "A", resp.Columns[0].Label)
	assert.Equal(t, "Product", resp.Columns[0].Header)
	assert.Equal(t, "B", resp.Columns[1].Label)
	assert.Equal(t, "Category", resp.Columns[1].Header)
}

