package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/yourusername/telegram-ai-bot/internal/domain/constants"
)

// Config ilovaning konfiguratsiyasi
type Config struct {
	TelegramToken  string
	GeminiAPIKey   string
	AdminPassword  string
	AllowEmptySecrets bool
	MaxContextSize int
	Group1ChatID   int64
	Group1ThreadID int
	Group2ChatID   int64
	Group2ThreadID int
	Group3ChatID   int64
	Group3ThreadID int
	Group4ChatID   int64
	Group4ThreadID int
	Group5ChatID   int64
	Group5ThreadID int
}

func parseChatTarget(raw string) (int64, int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, nil
	}
	// Inline kommentariyalarni qo'llab-quvvatlash: "-100.../4  # izoh"
	if idx := strings.Index(raw, "#"); idx >= 0 {
		raw = strings.TrimSpace(raw[:idx])
	}
	parts := strings.Split(raw, "/")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("noto'g'ri format, misol: -1001234567890 yoki -1001234567890/2")
	}

	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if chatID > 0 {
		// Supergroup/kanallarda manfiy bo'lishi kerak, shuning uchun avtomatik tuzatamiz
		chatID = -chatID
	}

	threadID := 0
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		tid, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("topic ID noto'g'ri: %v", err)
		}
		if tid < 0 {
			tid = -tid
		}
		threadID = tid
	}

	return chatID, threadID, nil
}

// Load konfiguratsiyani yuklash
func Load() (*Config, error) {
	// .env faylini yuklash (mavjud bo'lsa)
	_ = godotenv.Load()

	config := &Config{
		TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		GeminiAPIKey:   os.Getenv("GEMINI_API_KEY"),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
		AllowEmptySecrets: getEnvBool("ALLOW_EMPTY_SECRETS", false),
		MaxContextSize: constants.DefaultMaxContextSize,
	}

	if rawGroupID := os.Getenv("GROUP_1_CHAT_ID"); rawGroupID != "" {
		chatID, threadID, err := parseChatTarget(rawGroupID)
		if err != nil {
			return nil, fmt.Errorf("GROUP_1_CHAT_ID noto'g'ri formatda: %v", err)
		}
		config.Group1ChatID = chatID
		config.Group1ThreadID = threadID
	}

	if rawGroupID := os.Getenv("GROUP_2_CHAT_ID"); rawGroupID != "" {
		chatID, threadID, err := parseChatTarget(rawGroupID)
		if err != nil {
			return nil, fmt.Errorf("GROUP_2_CHAT_ID noto'g'ri formatda: %v", err)
		}
		config.Group2ChatID = chatID
		config.Group2ThreadID = threadID
	}

	if rawGroupID := os.Getenv("GROUP_3_CHAT_ID"); rawGroupID != "" {
		chatID, threadID, err := parseChatTarget(rawGroupID)
		if err != nil {
			return nil, fmt.Errorf("GROUP_3_CHAT_ID noto'g'ri formatda: %v", err)
		}
		config.Group3ChatID = chatID
		config.Group3ThreadID = threadID
	}

	if rawGroupID := os.Getenv("GROUP_4_CHAT_ID"); rawGroupID != "" {
		chatID, threadID, err := parseChatTarget(rawGroupID)
		if err != nil {
			return nil, fmt.Errorf("GROUP_4_CHAT_ID noto'g'ri formatda: %v", err)
		}
		config.Group4ChatID = chatID
		config.Group4ThreadID = threadID
	}

	if rawGroupID := os.Getenv("GROUP_5_CHAT_ID"); rawGroupID != "" {
		chatID, threadID, err := parseChatTarget(rawGroupID)
		if err != nil {
			return nil, fmt.Errorf("GROUP_5_CHAT_ID noto'g'ri formatda: %v", err)
		}
		config.Group5ChatID = chatID
		config.Group5ThreadID = threadID
	}

	// Validatsiya
	if !config.AllowEmptySecrets {
		if config.TelegramToken == "" {
			return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable bo'sh")
		}
		if config.GeminiAPIKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable bo'sh")
		}
		if config.AdminPassword == "" {
			return nil, fmt.Errorf("ADMIN_PASSWORD environment variable bo'sh")
		}
	}

	return config, nil
}

func getEnvBool(key string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}
