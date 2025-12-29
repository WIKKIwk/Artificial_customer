package models

import (
	"encoding/json"
	"time"
)

// SpreadsheetData represents the model stored in the database
type SpreadsheetData struct {
	ID        uint `gorm:"primaryKey"`
	Filename  string
	RowData   json.RawMessage `gorm:"type:jsonb"` // Store row as JSON for flexibility
	CreatedAt time.Time
}
