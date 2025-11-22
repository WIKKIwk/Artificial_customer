package entity

import "time"

// AdminSession admin sessiya
type AdminSession struct {
	UserID       int64
	IsAdmin      bool
	LoginTime    time.Time
	LastActivity time.Time
}

// AdminAction admin harakatlari
type AdminAction struct {
	ID        string
	UserID    int64
	Action    string // "login", "upload_catalog", "update_product"
	Details   string
	Timestamp time.Time
}
