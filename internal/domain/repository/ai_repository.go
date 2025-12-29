package repository

import (
	"context"

	"github.com/google/generative-ai-go/genai"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// AIRepository AI bilan ishlash uchun interface
type AIRepository interface {
	// GenerateResponse foydalanuvchi xabariga javob yaratish
	GenerateResponse(ctx context.Context, message entity.Message, context []entity.Message) (string, error)

	// GenerateResponseWithHistory kontekst bilan javob yaratish
	GenerateResponseWithHistory(ctx context.Context, userID int64, message string, history []entity.Message) (string, error)

	// GenerateConfigResponse MAXSUS konfiguratsiya uchun javob yaratish
	// Bu funksiya faqat /configuratsiya komandasi uchun ishlatiladi va PC yig'ishga ruxsat beradi
	GenerateConfigResponse(ctx context.Context, userID int64, message string, history []entity.Message) (string, error)

	// GetRawClient raw Gemini client ni qaytaradi (SmartRouter uchun)
	GetRawClient() *genai.Client
}
