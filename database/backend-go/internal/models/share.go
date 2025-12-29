package models

import "time"

// SheetFileShare grants a user access to a SheetFile owned by another user.
// Role can be "viewer" or "editor".
type SheetFileShare struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	FileID uint   `gorm:"not null;uniqueIndex:idx_sheet_file_share" json:"file_id"`
	UserID uint   `gorm:"not null;uniqueIndex:idx_sheet_file_share" json:"user_id"`
	Role   string `gorm:"type:varchar(16);not null" json:"role"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
