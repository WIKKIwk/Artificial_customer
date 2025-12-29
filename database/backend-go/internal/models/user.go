package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name                   string  `gorm:"not null"`
	Email                  string  `gorm:"uniqueIndex;not null"`
	Password               string  `gorm:"not null"`
	ApiKey                 *string `gorm:"uniqueIndex"` // nil => NULL, avoids duplicate '' collisions
	GeminiAPIKey           *string `gorm:"type:text"`   // user-supplied Gemini API key (optional, not unique)
	EmailVerified          bool    `gorm:"default:false"`
	EmailVerifyToken       string  `gorm:"index"`
	EmailVerifyTokenExpiry *time.Time
	ResetToken             string `gorm:"index"`
	ResetTokenExpiry       *time.Time
	FailedLoginAttempts    int `gorm:"default:0"`
	LockedUntil            *time.Time
}
