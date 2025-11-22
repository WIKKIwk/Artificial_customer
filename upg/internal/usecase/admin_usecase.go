package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

const (
	AdminPassword = "@#12" // Admin paroli
)

// AdminUseCase admin bilan bog'liq business logic
type AdminUseCase interface {
	// Login admin login qilish
	Login(ctx context.Context, userID int64, password string) (bool, error)

	// Logout admin logout qilish
	Logout(ctx context.Context, userID int64) error

	// IsAdmin admin ekanligini tekshirish
	IsAdmin(ctx context.Context, userID int64) (bool, error)

	// UploadCatalog Excel fayldan katalogni yuklash
	UploadCatalog(ctx context.Context, userID int64, fileData []byte, filename string) (int, error)

	// GetCatalogInfo katalog haqida ma'lumot
	GetCatalogInfo(ctx context.Context) (string, error)

	// CleanAll barcha mahsulotlar va chat tarixlarini tozalash
	CleanAll(ctx context.Context, userID int64) error
}

type adminUseCase struct {
	adminRepo   repository.AdminRepository
	productRepo repository.ProductRepository
	excelParser repository.ExcelParser
	chatRepo    repository.ChatRepository
}

// NewAdminUseCase yangi AdminUseCase yaratish
func NewAdminUseCase(
	adminRepo repository.AdminRepository,
	productRepo repository.ProductRepository,
	excelParser repository.ExcelParser,
	chatRepo repository.ChatRepository,
) AdminUseCase {
	return &adminUseCase{
		adminRepo:   adminRepo,
		productRepo: productRepo,
		excelParser: excelParser,
		chatRepo:    chatRepo,
	}
}

// Login admin login qilish
func (u *adminUseCase) Login(ctx context.Context, userID int64, password string) (bool, error) {
	// Parolni tekshirish
	if password != AdminPassword {
		return false, nil
	}

	// Admin sessiyasini yaratish
	session := entity.AdminSession{
		UserID:       userID,
		IsAdmin:      true,
		LoginTime:    time.Now(),
		LastActivity: time.Now(),
	}

	if err := u.adminRepo.CreateSession(ctx, session); err != nil {
		return false, fmt.Errorf("failed to create session: %w", err)
	}

	// Login harakatini loglash
	action := entity.AdminAction{
		ID:        uuid.New().String(),
		UserID:    userID,
		Action:    "login",
		Details:   "Admin successfully logged in",
		Timestamp: time.Now(),
	}
	_ = u.adminRepo.LogAction(ctx, action)

	return true, nil
}

// Logout admin logout qilish
func (u *adminUseCase) Logout(ctx context.Context, userID int64) error {
	return u.adminRepo.DeleteSession(ctx, userID)
}

// IsAdmin admin ekanligini tekshirish
func (u *adminUseCase) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	return u.adminRepo.IsAdmin(ctx, userID)
}

// UploadCatalog Excel fayldan katalogni yuklash
func (u *adminUseCase) UploadCatalog(ctx context.Context, userID int64, fileData []byte, filename string) (int, error) {
	// Admin tekshirish
	isAdmin, err := u.adminRepo.IsAdmin(ctx, userID)
	if err != nil {
		return 0, err
	}
	if !isAdmin {
		return 0, fmt.Errorf("user is not admin")
	}

	// Excel faylni parse qilish
	products, err := u.excelParser.ParseProductsFromBytes(ctx, fileData, filename)
	if err != nil {
		return 0, fmt.Errorf("failed to parse excel: %w", err)
	}

	if len(products) == 0 {
		return 0, fmt.Errorf("no products found in excel file")
	}

	// Katalogni yangilash
	catalog := entity.ProductCatalog{
		Products:  products,
		UpdatedAt: time.Now(),
		Source:    filename,
	}

	if err := u.productRepo.UpdateCatalog(ctx, catalog); err != nil {
		return 0, fmt.Errorf("failed to update catalog: %w", err)
	}

	// Upload harakatini loglash
	action := entity.AdminAction{
		ID:        uuid.New().String(),
		UserID:    userID,
		Action:    "upload_catalog",
		Details:   fmt.Sprintf("Uploaded %d products from %s", len(products), filename),
		Timestamp: time.Now(),
	}
	_ = u.adminRepo.LogAction(ctx, action)

	return len(products), nil
}

// GetCatalogInfo katalog haqida ma'lumot
func (u *adminUseCase) GetCatalogInfo(ctx context.Context) (string, error) {
	catalog, err := u.productRepo.GetCatalog(ctx)
	if err != nil {
		return "", err
	}

	// Kategoriyalarni sanash
	categories := make(map[string]int)
	for _, product := range catalog.Products {
		categories[product.Category]++
	}

	info := fmt.Sprintf("📦 Katalog: %s\n", catalog.Source)
	info += fmt.Sprintf("📅 Yangilangan: %s\n", catalog.UpdatedAt.Format("2006-01-02 15:04"))
	info += fmt.Sprintf("📊 Jami mahsulotlar: %d\n\n", len(catalog.Products))
	info += "📂 Kategoriyalar:\n"
	for cat, count := range categories {
		info += fmt.Sprintf("  • %s: %d ta\n", cat, count)
	}

	return info, nil
}

// CleanAll barcha mahsulotlar va chat tarixlarini tozalash
func (u *adminUseCase) CleanAll(ctx context.Context, userID int64) error {
	isAdmin, err := u.adminRepo.IsAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return fmt.Errorf("user is not admin")
	}

	if err := u.productRepo.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear products: %w", err)
	}
	if err := u.chatRepo.ClearAll(ctx); err != nil {
		return fmt.Errorf("failed to clear chats: %w", err)
	}

	action := entity.AdminAction{
		ID:        uuid.New().String(),
		UserID:    userID,
		Action:    "clean_all",
		Details:   "Cleared products and chat histories",
		Timestamp: time.Now(),
	}
	_ = u.adminRepo.LogAction(ctx, action)

	return nil
}
