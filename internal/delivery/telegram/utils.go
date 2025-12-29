package telegram

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// helper to generate UUID string if needed elsewhere
func newUUID() string {
	return uuid.New().String()
}

func t(lang, uz, ru string) string {
	if lang == "ru" {
		return ru
	}
	return uz
}

func nonEmpty(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

// replyID reply qilingan xabar ID sini qaytaradi (yoki 0)
func replyID(msg *tgbotapi.Message) int {
	if msg == nil || msg.ReplyToMessage == nil {
		return 0
	}
	return msg.ReplyToMessage.MessageID
}
