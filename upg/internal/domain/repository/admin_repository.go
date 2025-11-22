package repository

import (
	"context"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// AdminRepository admin bilan ishlash uchun interface
type AdminRepository interface {
	// CreateSession admin sessiyasini yaratish
	CreateSession(ctx context.Context, session entity.AdminSession) error

	// GetSession sessiyani olish
	GetSession(ctx context.Context, userID int64) (*entity.AdminSession, error)

	// DeleteSession sessiyani o'chirish (logout)
	DeleteSession(ctx context.Context, userID int64) error

	// IsAdmin foydalanuvchi admin ekanligini tekshirish
	IsAdmin(ctx context.Context, userID int64) (bool, error)

	// LogAction admin harakatini loglash
	LogAction(ctx context.Context, action entity.AdminAction) error
}
