package repository

import (
	"context"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// ProductRepository mahsulotlar bilan ishlash uchun interface
type ProductRepository interface {
	// SaveProduct mahsulotni saqlash
	SaveProduct(ctx context.Context, product entity.Product) error

	// SaveMany ko'p mahsulotlarni saqlash
	SaveMany(ctx context.Context, products []entity.Product) error

	// GetByID ID bo'yicha mahsulotni olish
	GetByID(ctx context.Context, id string) (*entity.Product, error)

	// Search mahsulot qidirish
	Search(ctx context.Context, query string) ([]entity.Product, error)

	// GetByCategory kategoriya bo'yicha mahsulotlarni olish
	GetByCategory(ctx context.Context, category string) ([]entity.Product, error)

	// GetAll barcha mahsulotlarni olish
	GetAll(ctx context.Context) ([]entity.Product, error)

	// UpdateCatalog butun katalogni yangilash
	UpdateCatalog(ctx context.Context, catalog entity.ProductCatalog) error

	// GetCatalog katalogni olish
	GetCatalog(ctx context.Context) (*entity.ProductCatalog, error)

	// Clear barcha mahsulotlarni o'chirish
	Clear(ctx context.Context) error
}
