package repository

import (
	"context"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// AIRepository AI bilan ishlash uchun interface
type AIRepository interface {
	// GenerateResponse foydalanuvchi xabariga javob yaratish
	GenerateResponse(ctx context.Context, message entity.Message, context []entity.Message) (string, error)

	// GenerateResponseWithHistory kontekst bilan javob yaratish
	GenerateResponseWithHistory(ctx context.Context, userID int64, message string, history []entity.Message) (string, error)
}
