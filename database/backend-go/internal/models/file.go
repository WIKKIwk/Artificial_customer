package models

import (
	"encoding/json"
	"time"
)

// SheetFile stores a user's spreadsheet state (JSON from frontend).
type SheetFile struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	UserID    uint            `gorm:"index;not null" json:"user_id"`
	Name      string          `gorm:"not null" json:"name"`
	State     json.RawMessage `gorm:"type:jsonb;not null" json:"state"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
