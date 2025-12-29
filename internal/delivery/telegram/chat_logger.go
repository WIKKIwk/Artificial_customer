package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandler) logIncomingChatMessage(msg *tgbotapi.Message) {
	if h == nil || h.chatStore == nil || msg == nil || msg.From == nil || msg.Chat == nil || !msg.Chat.IsPrivate() {
		return
	}
	text := extractIncomingChatText(msg)
	if strings.TrimSpace(text) == "" {
		return
	}
	username := msg.From.UserName
	if username == "" {
		username = msg.From.FirstName
	}
	h.logChatMessage(chatLogMessage{
		UserID:    msg.From.ID,
		ChatID:    msg.Chat.ID,
		Username:  username,
		Direction: "user",
		MessageID: msg.MessageID,
		Text:      text,
		CreatedAt: time.Now(),
	})
}

func (h *BotHandler) logOutgoingChatMessage(chatID int64, messageID int, text string) {
	if h == nil || h.chatStore == nil {
		return
	}
	if chatID <= 0 {
		return
	}
	trim := strings.TrimSpace(text)
	if trim == "" {
		return
	}
	username := h.getCachedUsername(chatID)
	h.logChatMessage(chatLogMessage{
		UserID:    chatID,
		ChatID:    chatID,
		Username:  username,
		Direction: "bot",
		MessageID: messageID,
		Text:      trim,
		CreatedAt: time.Now(),
	})
}

func (h *BotHandler) logOutgoingFromChattable(msg tgbotapi.Chattable, sent tgbotapi.Message) {
	chatID, text, ok := outgoingTextFromChattable(msg)
	if !ok {
		return
	}
	if sent.Chat != nil && sent.Chat.ID != 0 {
		chatID = sent.Chat.ID
	}
	if chatID == 0 {
		return
	}
	h.logOutgoingChatMessage(chatID, sent.MessageID, text)
}

func (h *BotHandler) logChatMessage(msg chatLogMessage) {
	if h == nil || h.chatStore == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.chatStore.Save(ctx, msg); err != nil {
			log.Printf("[chatlog] save failed: %v", err)
		}
	}()
}

func (h *BotHandler) getCachedUsername(userID int64) string {
	h.nameMu.RLock()
	defer h.nameMu.RUnlock()
	return strings.TrimSpace(h.lastName[userID])
}

func extractIncomingChatText(msg *tgbotapi.Message) string {
	if msg == nil {
		return ""
	}
	if strings.TrimSpace(msg.Text) != "" {
		return msg.Text
	}
	if msg.Contact != nil {
		phone := strings.TrimSpace(msg.Contact.PhoneNumber)
		if phone != "" {
			return fmt.Sprintf("[contact] %s", phone)
		}
		return "[contact]"
	}
	if msg.Location != nil {
		return fmt.Sprintf("[location] %.6f, %.6f", msg.Location.Latitude, msg.Location.Longitude)
	}
	if msg.Document != nil {
		name := strings.TrimSpace(msg.Document.FileName)
		if name != "" {
			return fmt.Sprintf("[document] %s", name)
		}
		return "[document]"
	}
	if msg.Photo != nil {
		return "[photo]"
	}
	if msg.Video != nil {
		return "[video]"
	}
	if msg.Audio != nil {
		return "[audio]"
	}
	if msg.Voice != nil {
		return "[voice]"
	}
	return ""
}

func outgoingTextFromChattable(msg tgbotapi.Chattable) (int64, string, bool) {
	switch m := msg.(type) {
	case tgbotapi.MessageConfig:
		return m.ChatID, m.Text, true
	case tgbotapi.PhotoConfig:
		return m.ChatID, strings.TrimSpace(m.Caption), true
	case tgbotapi.DocumentConfig:
		if strings.TrimSpace(m.Caption) != "" {
			return m.ChatID, m.Caption, true
		}
		return m.ChatID, "[document]", true
	case tgbotapi.AudioConfig:
		return m.ChatID, strings.TrimSpace(m.Caption), true
	case tgbotapi.VideoConfig:
		return m.ChatID, strings.TrimSpace(m.Caption), true
	case tgbotapi.VoiceConfig:
		return m.ChatID, "[voice]", true
	case tgbotapi.StickerConfig:
		return m.ChatID, "[sticker]", true
	default:
		return 0, "", false
	}
}
