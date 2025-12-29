package telegram

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// threadIDForChat returns thread ID for known group chats (topics)
func (h *BotHandler) threadIDForChat(chatID int64) int {
	if chatID == h.group1ChatID {
		return h.group1ThreadID
	}
	if chatID == h.group2ChatID {
		return h.group2ThreadID
	}
	if chatID == h.group3ChatID {
		return h.group3ThreadID
	}
	if chatID == h.group4ChatID {
		return h.group4ThreadID
	}
	if chatID == h.group5ChatID {
		return h.group5ThreadID
	}
	return 0
}

// sendText sends a message with optional parseMode/replyMarkup and supports forum topics via threadOverride.
func (h *BotHandler) sendText(chatID int64, text string, parseMode string, replyMarkup interface{}, threadOverride int) (*tgbotapi.Message, error) {
	if h.bot == nil {
		return nil, fmt.Errorf("telegram bot is nil")
	}

	threadID := threadOverride
	if threadID == 0 {
		threadID = h.threadIDForChat(chatID)
	}

	if threadID > 0 {
		params := make(tgbotapi.Params)
		params.AddNonZero64("chat_id", chatID)
		params.AddNonZero("message_thread_id", threadID)
		params.AddNonEmpty("text", text)
		params.AddNonEmpty("parse_mode", parseMode)
		if replyMarkup != nil {
			if err := params.AddInterface("reply_markup", replyMarkup); err != nil {
				return nil, err
			}
		}
		resp, err := h.bot.MakeRequest("sendMessage", params)
		if err != nil {
			return nil, err
		}
		var msg tgbotapi.Message
		if err := json.Unmarshal(resp.Result, &msg); err != nil {
			return nil, err
		}
		h.logOutgoingChatMessage(chatID, msg.MessageID, text)
		return &msg, nil
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMode
	if replyMarkup != nil {
		msg.ReplyMarkup = replyMarkup
	}
	sent, err := h.sendAndLog(msg)
	if err != nil {
		return nil, err
	}
	return &sent, nil
}

func (h *BotHandler) sendAndLog(msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	if h.bot == nil {
		return tgbotapi.Message{}, fmt.Errorf("telegram bot is nil")
	}
	sent, err := h.bot.Send(msg)
	if err != nil {
		return sent, err
	}
	h.logOutgoingFromChattable(msg, sent)
	return sent, nil
}

// sendMessage oddiy xabar yuborish
func (h *BotHandler) sendMessage(chatID int64, text string) {
	if h.bot == nil {
		log.Printf("sendMessage skipped (bot is nil) chat=%d text=%q", chatID, truncateForLog(text, 120))
		return
	}

	// Bo'sh xabar tekshirish
	if strings.TrimSpace(text) == "" {
		log.Printf("⚠️ Bo'sh xabar yuborilmoqchi bo'ldi! ChatID: %d", chatID)
		text = `Kechirasiz, javob tayyorlanmadi. 😔

Quyidagilarni sinab ko'ring:
• Savolingizni boshqacha so'rab ko'ring
• /help - yordam va komandalar
• /clear - chat tarixini tozalash va qaytadan boshlash`
	}

	for _, chunk := range splitIntoChunks(text, 4096) {
		if sent, err := h.sendText(chatID, chunk, "", nil, 0); err != nil {
			log.Printf("Xabar yuborishda xatolik: %v", err)
			return
		} else if sent != nil {
			h.trackAdminMessage(chatID, sent.MessageID)
		}
	}
}

// sendMessageWithResp yuborilgan xabarni qaytarish
func (h *BotHandler) sendMessageWithResp(chatID int64, text string) (*tgbotapi.Message, error) {
	if h.bot == nil {
		return nil, fmt.Errorf("telegram bot is nil")
	}

	// Bo'sh xabar tekshirish
	if strings.TrimSpace(text) == "" {
		log.Printf("⚠️ Bo'sh xabar yuborilmoqchi bo'ldi! ChatID: %d", chatID)
		text = `Kechirasiz, javob tayyorlanmadi. 😔

Quyidagilarni sinab ko'ring:
• Savolingizni boshqacha so'rab ko'ring
• /help - yordam va komandalar
• /clear - chat tarixini tozalash va qaytadan boshlash`
	}

	sent, err := h.sendText(chatID, text, "", nil, 0)
	if err != nil {
		log.Printf("Xabar yuborishda xatolik: %v", err)
		return nil, err
	}
	h.trackAdminMessage(chatID, sent.MessageID)
	return sent, nil
}

// sendMessageMarkdown markdown formatda xabar yuborish
func (h *BotHandler) sendMessageMarkdown(chatID int64, text string) {
	// Bo'sh xabar tekshirish
	if strings.TrimSpace(text) == "" {
		log.Printf("⚠️ Bo'sh xabar yuborilmoqchi bo'ldi! ChatID: %d", chatID)
		text = `Kechirasiz, javob tayyorlanmadi. 😔

Quyidagilarni sinab ko'ring:
• Savolingizni boshqacha so'rab ko'ring
• /help - yordam va komandalar
• /clear - chat tarixini tozalash va qaytadan boshlash`
	}

	if sent, err := h.sendText(chatID, text, "Markdown", nil, 0); err != nil {
		log.Printf("Xabar yuborishda xatolik: %v", err)
	} else if sent != nil {
		h.trackAdminMessage(chatID, sent.MessageID)
	}
}

// splitIntoChunks matnni Telegram limitiga mos bo'lib yuborish uchun bo'ladi
func splitIntoChunks(s string, limit int) []string {
	if limit <= 0 {
		return []string{s}
	}
	var chunks []string
	var current strings.Builder

	for _, r := range s {
		current.WriteRune(r)
		if current.Len() >= limit {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}
