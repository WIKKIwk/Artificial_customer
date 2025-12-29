package telegram

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// formatAgo - vaqt farqini foydalanuvchiga qulay ko'rinishda qaytarish
func formatAgo(d time.Duration, lang string) string {
	if d < time.Minute {
		return t(lang, "hozir", "только что")
	}
	if d < time.Hour {
		min := int(d.Minutes())
		return fmt.Sprintf("%d %s", min, t(lang, "daq.", "мин"))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		return fmt.Sprintf("%d %s", h, t(lang, "soat", "ч"))
	}
	day := int(d.Hours() / 24)
	return fmt.Sprintf("%d %s", day, t(lang, "kun", "д"))
}

// formatClock - oxirgi aktiv vaqtni "15:04" yoki sana bilan qaytaradi
func formatClock(ts time.Time) string {
	tm := ts.In(time.Local)
	if time.Since(tm) > 24*time.Hour {
		return tm.Format("2006-01-02 15:04")
	}
	return tm.Format("15:04")
}

// deleteMessage - xabarni xatolarsiz o'chirish
func (h *BotHandler) deleteMessage(chatID int64, msgID int) {
	del := tgbotapi.NewDeleteMessage(chatID, msgID)
	if _, err := h.bot.Request(del); err != nil {
		log.Printf("delete message failed: %v", err)
	}
}

// deleteCommandMessage - komanda xabarini o'chirish, qayta urinish bilan
func (h *BotHandler) deleteCommandMessage(msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	chatID := msg.Chat.ID
	msgID := msg.MessageID
	h.deleteMessage(chatID, msgID)
	go func() {
		time.Sleep(500 * time.Millisecond)
		h.deleteMessage(chatID, msgID)
	}()
}

// deleteUserMessage - foydalanuvchi xabarini xavfsiz o'chirish
func (h *BotHandler) deleteUserMessage(chatID int64, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	h.deleteMessage(chatID, msg.MessageID)
}

func (h *BotHandler) trackWelcomeMessage(chatID int64, msgID int) {
	if chatID == 0 || msgID == 0 {
		return
	}
	h.welcomeMu.Lock()
	list := h.welcomeMsgs[chatID]
	for _, id := range list {
		if id == msgID {
			h.welcomeMu.Unlock()
			return
		}
	}
	h.welcomeMsgs[chatID] = append(list, msgID)
	h.welcomeMu.Unlock()
}

func (h *BotHandler) cleanupWelcomeMessages(chatID int64) {
	if chatID == 0 {
		return
	}
	h.welcomeMu.Lock()
	list := h.welcomeMsgs[chatID]
	delete(h.welcomeMsgs, chatID)
	h.welcomeMu.Unlock()
	for _, msgID := range list {
		h.deleteMessage(chatID, msgID)
	}
}
