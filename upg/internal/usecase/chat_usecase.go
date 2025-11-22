package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

// ChatUseCase chat bilan bog'liq business logic
type ChatUseCase interface {
	ProcessMessage(ctx context.Context, userID int64, username, text string) (string, error)
	ClearHistory(ctx context.Context, userID int64) error
	GetHistory(ctx context.Context, userID int64) ([]entity.Message, error)
	GetAllMessages(ctx context.Context, limit int) ([]entity.Message, error)
}

type chatUseCase struct {
	aiRepo      repository.AIRepository
	chatRepo    repository.ChatRepository
	productRepo repository.ProductRepository
}

// NewChatUseCase yangi ChatUseCase yaratish
func NewChatUseCase(
	aiRepo repository.AIRepository,
	chatRepo repository.ChatRepository,
	productRepo repository.ProductRepository,
) ChatUseCase {
	return &chatUseCase{
		aiRepo:      aiRepo,
		chatRepo:    chatRepo,
		productRepo: productRepo,
	}
}

// ProcessMessage foydalanuvchi xabarini qayta ishlash
func (u *chatUseCase) ProcessMessage(ctx context.Context, userID int64, username, text string) (string, error) {
	// AI so'rovlarini osilib qolmasligi uchun timeout
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Oldingi tarixni olish (oxirgi 10 ta xabar)
	history, err := u.chatRepo.GetHistory(ctx, userID, 10)
	if err != nil {
		return "", fmt.Errorf("failed to get history: %w", err)
	}

	// Mahsulotlar borligini tekshirish
	products, err := u.productRepo.GetAll(ctx)
	hasProducts := err == nil && len(products) > 0

	// Foydalanuvchi xabariga mahsulot katalogini qo'shish
	enrichedText := text
	if hasProducts {
		// HAR DOIM mahsulot ma'lumotini AI ga yuborish
		productsInfo := u.buildProductsContext(products)
		enrichedText = fmt.Sprintf(`Mijoz: %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📦 SIZNING DO'KONINGIZDA MAVJUD MAHSULOTLAR:
%s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ MAJBURIY QOIDALAR:
1. FAQAT yuqoridagi ro'yxatdan mahsulot taklif qiling
2. ALBATTA to'liq mahsulot nomini yozing (masalan: "Intel Core i5-12400F" yoki "RTX 3060 12GB")
3. HECH QACHON "270$ lik variant" demas - ANIQ nomini yoz!
4. Har bir mahsulot uchun ANIQ narxni ro'yxatdan ko'rsating
5. Agar mijoz budjet aytsa (masalan 1000$), imkon qadar shu budjetga yaqinlash — 0..100$ gacha oshishi mumkin
6. Jami summani hisoblashda xato qilma
7. Budjet yetmasa, arzonroq variantlar taklif qil

Mijozga javob ber:`, text, productsInfo)

		// DEBUG: AI ga ketayotgan ma'lumotni log qilish
		fmt.Printf("\n═══════════════════════════════════════════════════════\n")
		fmt.Printf("🤖 AI GA YUBORILAYOTGAN PROMPT:\n")
		fmt.Printf("═══════════════════════════════════════════════════════\n")
		fmt.Printf("%s\n", enrichedText)
		fmt.Printf("═══════════════════════════════════════════════════════\n\n")
	}

	// AI dan javob olish
	response, err := u.aiRepo.GenerateResponseWithHistory(ctx, userID, enrichedText, history)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	// Xabar va javobni saqlash (original text bilan, enriched emas!)
	message := entity.Message{
		ID:        uuid.New().String(),
		UserID:    userID,
		Username:  username,
		Text:      text, // Original text
		Response:  response,
		Timestamp: time.Now(),
	}

	if err := u.chatRepo.SaveMessage(ctx, message); err != nil {
		return "", fmt.Errorf("failed to save message: %w", err)
	}

	return response, nil
}

// buildProductsContext mahsulotlardan kontekst yaratish
func (u *chatUseCase) buildProductsContext(products []entity.Product) string {
	var sb strings.Builder

	// Kategoriyalar bo'yicha guruhlash
	categoryMap := make(map[string][]entity.Product)
	for _, p := range products {
		cat := p.Category
		if cat == "" {
			cat = "Boshqa"
		}
		categoryMap[cat] = append(categoryMap[cat], p)
	}

	// Mahsulotlarni yozish
	for category, prods := range categoryMap {
		sb.WriteString(fmt.Sprintf("\n📂 %s:\n", category))
		for i, p := range prods {
			// Narxni dollar formatida ko'rsatish (Stock 0 bo'lsa ham ko'rsatamiz - product mavjud)
			sb.WriteString(fmt.Sprintf("  %d. %s - $%.2f", i+1, p.Name, p.Price))

			if p.Stock > 0 {
				sb.WriteString(fmt.Sprintf(" (Omborda: %d ta)", p.Stock))
			}

			if p.Description != "" {
				sb.WriteString(fmt.Sprintf("\n     └─ %s", p.Description))
			}

			// Specs borligini tekshirish
			if len(p.Specs) > 0 {
				sb.WriteString("\n     └─ ")
				specs := []string{}
				for key, value := range p.Specs {
					specs = append(specs, fmt.Sprintf("%s: %s", key, value))
				}
				sb.WriteString(strings.Join(specs, ", "))
			}

			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ClearHistory foydalanuvchi tarixini tozalash
func (u *chatUseCase) ClearHistory(ctx context.Context, userID int64) error {
	return u.chatRepo.ClearHistory(ctx, userID)
}

// GetHistory foydalanuvchi tarixini olish
func (u *chatUseCase) GetHistory(ctx context.Context, userID int64) ([]entity.Message, error) {
	return u.chatRepo.GetHistory(ctx, userID, 0)
}

// GetAllMessages barcha foydalanuvchi xabarlarini olish (admin uchun)
func (u *chatUseCase) GetAllMessages(ctx context.Context, limit int) ([]entity.Message, error) {
	return u.chatRepo.GetAllMessages(ctx, limit)
}
