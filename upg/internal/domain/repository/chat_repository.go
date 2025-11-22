package repository

import (
	"context"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// ChatRepository chat history bilan ishlash uchun interface
type ChatRepository interface {
	// SaveMessage xabarni saqlash
	SaveMessage(ctx context.Context, message entity.Message) error

	// GetHistory foydalanuvchi chat tarixini olish
	GetHistory(ctx context.Context, userID int64, limit int) ([]entity.Message, error)

	// GetAllMessages barcha foydalanuvchi xabarlarini olish (so'nggi limit ta)
	GetAllMessages(ctx context.Context, limit int) ([]entity.Message, error)

	// ClearHistory foydalanuvchi tarixini tozalash
	ClearHistory(ctx context.Context, userID int64) error

	// ClearAll barcha foydalanuvchilarning tarixini o'chirish
	ClearAll(ctx context.Context) error

	// GetContext foydalanuvchi chat kontekstini olish
	GetContext(ctx context.Context, userID int64) (*entity.ChatContext, error)
}
