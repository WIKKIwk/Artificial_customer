package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

// ProductUseCase mahsulot bilan bog'liq business logic
type ProductUseCase interface {
	// Search mahsulot qidirish
	Search(ctx context.Context, query string) ([]entity.Product, error)

	// GetByCategory kategoriya bo'yicha mahsulotlarni olish
	GetByCategory(ctx context.Context, category string) ([]entity.Product, error)

	// GetAll barcha mahsulotlarni olish
	GetAll(ctx context.Context) ([]entity.Product, error)

	// GetProductsAsText mahsulotlarni text formatda olish (AI uchun)
	GetProductsAsText(ctx context.Context) (string, error)

	// HasProducts mahsulotlar borligini tekshirish
	HasProducts(ctx context.Context) (bool, error)
}

type productUseCase struct {
	productRepo repository.ProductRepository
}

// NewProductUseCase yangi ProductUseCase yaratish
func NewProductUseCase(productRepo repository.ProductRepository) ProductUseCase {
	return &productUseCase{
		productRepo: productRepo,
	}
}

// Search mahsulot qidirish
func (u *productUseCase) Search(ctx context.Context, query string) ([]entity.Product, error) {
	return u.productRepo.Search(ctx, query)
}

// GetByCategory kategoriya bo'yicha mahsulotlarni olish
func (u *productUseCase) GetByCategory(ctx context.Context, category string) ([]entity.Product, error) {
	return u.productRepo.GetByCategory(ctx, category)
}

// GetAll barcha mahsulotlarni olish
func (u *productUseCase) GetAll(ctx context.Context) ([]entity.Product, error) {
	return u.productRepo.GetAll(ctx)
}

// GetProductsAsText mahsulotlarni text formatda olish (AI uchun)
func (u *productUseCase) GetProductsAsText(ctx context.Context) (string, error) {
	products, err := u.productRepo.GetAll(ctx)
	if err != nil {
		return "", err
	}

	if len(products) == 0 {
		return "", fmt.Errorf("no products available")
	}

	var sb strings.Builder
	sb.WriteString("=== MAVJUD MAHSULOTLAR ===\n\n")

	// Kategoriyalar bo'yicha guruhlash
	categoryMap := make(map[string][]entity.Product)
	for _, product := range products {
		category := product.Category
		if category == "" {
			category = "Boshqa"
		}
		categoryMap[category] = append(categoryMap[category], product)
	}

	// Har bir kategoriya uchun mahsulotlarni yozish
	for category, prods := range categoryMap {
		sb.WriteString(fmt.Sprintf("📂 %s:\n", category))
		for i, p := range prods {
			sb.WriteString(fmt.Sprintf("%d. %s - $%.2f", i+1, p.Name, p.Price))
			if p.Stock > 0 {
				sb.WriteString(fmt.Sprintf(" (Omborda: %d)", p.Stock))
			}
			if p.Description != "" {
				sb.WriteString(fmt.Sprintf("\n   %s", p.Description))
			}
			if len(p.Specs) > 0 {
				sb.WriteString("\n   Texnik xususiyatlar:")
				for key, value := range p.Specs {
					sb.WriteString(fmt.Sprintf("\n   - %s: %s", key, value))
				}
			}
			sb.WriteString("\n\n")
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// HasProducts mahsulotlar borligini tekshirish
func (u *productUseCase) HasProducts(ctx context.Context) (bool, error) {
	products, err := u.productRepo.GetAll(ctx)
	if err != nil {
		return false, err
	}
	return len(products) > 0, nil
}
