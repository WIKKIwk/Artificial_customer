package repository

import (
	"context"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// ExcelParser Excel fayllarni parse qilish uchun interface
type ExcelParser interface {
	// ParseProducts Excel fayldan mahsulotlarni o'qish
	ParseProducts(ctx context.Context, filePath string) ([]entity.Product, error)

	// ParseProductsFromBytes byte array dan parse qilish
	ParseProductsFromBytes(ctx context.Context, data []byte, filename string) ([]entity.Product, error)
}
