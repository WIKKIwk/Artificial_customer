package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandler) clearInlineButtons(cq *tgbotapi.CallbackQuery) bool {
	if cq == nil {
		return false
	}
	if cq.Message != nil && cq.Message.Chat != nil {
		return h.clearInlineButtonsByMessage(cq.Message.Chat.ID, cq.Message.MessageID, "")
	}
	if cq.InlineMessageID != "" {
		return h.clearInlineButtonsByMessage(0, 0, cq.InlineMessageID)
	}
	return false
}

func (h *BotHandler) clearInlineButtonsByMessage(chatID int64, messageID int, inlineID string) bool {
	empty := [][]tgbotapi.InlineKeyboardButton{}
	edit := tgbotapi.EditMessageReplyMarkupConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:          chatID,
			MessageID:       messageID,
			InlineMessageID: inlineID,
			ReplyMarkup:     &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: empty},
		},
	}
	if _, err := h.bot.Request(edit); err != nil {
		log.Printf("inline keyboard clear failed: %v", err)
		return false
	}
	return true
}

func (h *BotHandler) keepAnalyzePCButtonOnly(cq *tgbotapi.CallbackQuery, offerID, lang string) {
	if cq == nil || cq.Message == nil || cq.Message.Chat == nil {
		return
	}
	btn := tgbotapi.NewInlineKeyboardButtonData(t(lang, "📊 Analyze PC", "📊 Анализ ПК"), "cfg_analyze_pc|"+offerID)
	markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
	edit := tgbotapi.NewEditMessageReplyMarkup(cq.Message.Chat.ID, cq.Message.MessageID, markup)
	if _, err := h.bot.Request(edit); err != nil {
		log.Printf("inline keyboard keep analyze failed: %v", err)
	}
}

func (h *BotHandler) setPurchasePromptMessage(userID, chatID int64, messageID int) {
	if userID == 0 || chatID == 0 || messageID == 0 {
		return
	}
	h.purchaseMsgMu.Lock()
	h.purchaseMsg[userID] = purchasePromptMessage{chatID: chatID, messageID: messageID}
	h.purchaseMsgMu.Unlock()
}

func (h *BotHandler) popPurchasePromptMessage(userID int64) (purchasePromptMessage, bool) {
	h.purchaseMsgMu.Lock()
	defer h.purchaseMsgMu.Unlock()
	msg, ok := h.purchaseMsg[userID]
	if ok {
		delete(h.purchaseMsg, userID)
	}
	return msg, ok
}

func (h *BotHandler) clearPurchasePromptButtons(userID int64, cq *tgbotapi.CallbackQuery) {
	cleared := h.clearInlineButtons(cq)
	if msg, ok := h.popPurchasePromptMessage(userID); ok && !cleared {
		_ = h.clearInlineButtonsByMessage(msg.chatID, msg.messageID, "")
	}
}
