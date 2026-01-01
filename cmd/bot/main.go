package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/telegram-ai-bot/config"
	"github.com/yourusername/telegram-ai-bot/internal/delivery/telegram"
	"github.com/yourusername/telegram-ai-bot/internal/infrastructure/gemini"
	"github.com/yourusername/telegram-ai-bot/internal/infrastructure/parser"
	"github.com/yourusername/telegram-ai-bot/internal/infrastructure/storage"
	"github.com/yourusername/telegram-ai-bot/internal/usecase"
	"github.com/yourusername/telegram-ai-bot/pkg/logger"
)

func main() {
	initDefaultTimezone()

	// Logger ni ishga tushirish
	logger.Init()
	logger.InfoLogger.Println("🚀 Ilova ishga tushmoqda...")

	// Konfiguratsiyani yuklash
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Konfiguratsiya yuklanmadi: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if cfg.AllowEmptySecrets {
		if strings.TrimSpace(cfg.AdminPassword) == "" {
			cfg.AdminPassword = generateTempSecret(16)
			logger.InfoLogger.Printf("ADMIN_PASSWORD bo'sh. Vaqtinchalik parol: %s", cfg.AdminPassword)
		}

		missing := []string{}
		if isEmptyOrDisabled(cfg.TelegramToken) {
			missing = append(missing, "TELEGRAM_BOT_TOKEN")
		}
		if isEmptyOrDisabled(cfg.GeminiAPIKey) {
			missing = append(missing, "GEMINI_API_KEY")
		}
		if len(missing) > 0 {
			logger.InfoLogger.Printf("Secretlar yetishmayapti (%s). Bot vaqtincha ishga tushmaydi.", strings.Join(missing, ", "))
			<-sigChan
			return
		}
	}

	// Dependencies ni yaratish (Dependency Injection)

	// 1. Gemini AI client
	aiRepo, err := gemini.NewGeminiClient(cfg.GeminiAPIKey)
	if err != nil {
		log.Fatalf("❌ Gemini client yaratilmadi: %v", err)
	}
	logger.InfoLogger.Println("✅ Gemini AI client tayyor (gemini-2.5-flash)")

	// 2. Repositories (in-memory)
	chatRepo := storage.NewMemoryChatRepository(cfg.MaxContextSize)
	productRepo := storage.NewMemoryProductRepository()
	adminRepo := storage.NewMemoryAdminRepository()
	logger.InfoLogger.Println("✅ Repositories tayyor (in-memory)")

	// 3. Excel parser
	excelParser := parser.NewExcelParser()
	logger.InfoLogger.Println("✅ Excel parser tayyor")

	// 5. Use cases
	chatUseCase := usecase.NewChatUseCase(aiRepo, chatRepo, productRepo)
	adminUseCase := usecase.NewAdminUseCase(adminRepo, productRepo, excelParser, chatRepo, cfg.AdminPassword)
	productUseCase := usecase.NewProductUseCase(productRepo)
	logger.InfoLogger.Println("✅ Use cases tayyor")

	// 5. Telegram bot handler
	botHandler, err := telegram.NewBotHandler(
		cfg.TelegramToken,
		cfg.Group1ChatID,
		cfg.Group1ThreadID,
		cfg.Group2ChatID,
		cfg.Group2ThreadID,
		cfg.Group3ChatID,
		cfg.Group3ThreadID,
		cfg.Group4ChatID,
		cfg.Group4ThreadID,
		cfg.Group5ChatID,
		cfg.Group5ThreadID,
		chatUseCase,
		adminUseCase,
		productUseCase,
		aiRepo.GetRawClient(), // Gemini client for SmartRouter
	)
	if err != nil {
		log.Fatalf("❌ Bot handler yaratilmadi: %v", err)
	}
	logger.InfoLogger.Printf("✅ Telegram bot tayyor: @%s", botHandler.GetBotUsername())

	// Context yaratish
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Botni alohida goroutine da ishga tushirish
	go func() {
		if err := botHandler.Start(ctx); err != nil {
			logger.ErrorLogger.Printf("❌ Bot xatosi: %v", err)
		}
	}()

	logger.InfoLogger.Println("🤖 Bot ishlayapti. To'xtatish uchun Ctrl+C ni bosing.")

	// Signal kutish
	<-sigChan
	logger.InfoLogger.Println("⏳ To'xtatish signali qabul qilindi...")

	// Graceful shutdown
	cancel()
	logger.InfoLogger.Println("✅ Bot to'xtatildi.")
}

func initDefaultTimezone() {
	const tzName = "Asia/Tashkent"
	if loc, err := time.LoadLocation(tzName); err == nil {
		time.Local = loc
		return
	}
	time.Local = time.FixedZone(tzName, 5*60*60)
}

func isEmptyOrDisabled(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	return strings.EqualFold(value, "disabled")
}

func generateTempSecret(byteLen int) string {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "change-me"
	}
	return hex.EncodeToString(buf)
}
