package telegram

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Inline feedback tugmalari
func (h *BotHandler) sendConfigFeedbackPrompt(chatID int64, userID int64, offerID string) {
	if offerID == "" {
		offerID = newUUID()
	}
	lang := h.getUserLang(userID)
	msg := tgbotapi.NewMessage(chatID, t(lang,
		"✅ Konfiguratsiya tayyor!\n\nKonfiguratsiya yoqdimi? Savollaringiz bo'lsa menga yozishingiz mumkin va men darhol javob beraman. Agar juda zarur bo'lsa, @Ingame_support ga murojaat qiling 😊",
		"✅ Конфигурация готова!\n\nПонравилась конфигурация? Если есть вопросы, можете написать мне и я сразу отвечу. Если очень нужно, обращайтесь к @Ingame_support 😊",
	))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "✅ Zakaz beraman", "✅ Заказать"), "cfg_fb_yes|"+offerID),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "👎 Yo'q", "👎 Нет"), "cfg_fb_no|"+offerID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "📊 Analyze PC", "📊 Анализ ПК"), "cfg_analyze_pc|"+offerID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🔄 Komponentni almashtirish", "🔄 Заменить компонент"), "cfg_fb_change|"+offerID),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🗑️ Komponentni o'chirish", "🗑️ Удалить компонент"), "cfg_fb_delete|"+offerID),
		),
	)
	if _, err := h.sendAndLog(msg); err != nil {
		log.Printf("Feedback tugmalarini yuborishda xatolik: %v", err)
	}
}

// Feedback ma'lumotini saqlash va offerID qaytarish
func (h *BotHandler) saveFeedback(userID int64, info feedbackInfo) string {
	id := info.OfferID
	if id == "" {
		id = newUUID()
	}
	info.OfferID = id
	if strings.TrimSpace(info.ConfigText) != "" {
		h.setLastSuggestion(userID, info.ConfigText)
	}
	h.feedbackMu.Lock()
	defer h.feedbackMu.Unlock()
	h.feedbacks[userID] = info    // legacy latest
	h.feedbackByID[id] = info     // new scoped
	h.feedbackLatest[userID] = id // track latest id per user
	return id
}

// Feedback ma'lumotini olish va o'chirish
func (h *BotHandler) popFeedback(userID int64) (feedbackInfo, bool) {
	h.feedbackMu.Lock()
	defer h.feedbackMu.Unlock()
	// Legacy: latest by user
	if id, ok := h.feedbackLatest[userID]; ok {
		if info, ok2 := h.feedbackByID[id]; ok2 {
			delete(h.feedbackByID, id)
			delete(h.feedbackLatest, userID)
			delete(h.feedbacks, userID)
			return info, true
		}
	}
	info, ok := h.feedbacks[userID]
	if ok {
		delete(h.feedbacks, userID)
	}
	return info, ok
}

// getFeedback mavjud feedbackni o'chirmasdan olish
func (h *BotHandler) getFeedback(userID int64) (feedbackInfo, bool) {
	h.feedbackMu.RLock()
	defer h.feedbackMu.RUnlock()
	if id, ok := h.feedbackLatest[userID]; ok {
		if info, ok2 := h.feedbackByID[id]; ok2 {
			return info, true
		}
	}
	info, ok := h.feedbacks[userID]
	return info, ok
}

func (h *BotHandler) getFeedbackByID(id string) (feedbackInfo, bool) {
	h.feedbackMu.RLock()
	defer h.feedbackMu.RUnlock()
	info, ok := h.feedbackByID[id]
	return info, ok
}

func (h *BotHandler) getLatestFeedback(userID int64) (feedbackInfo, bool) {
	h.feedbackMu.RLock()
	defer h.feedbackMu.RUnlock()
	if id, ok := h.feedbackLatest[userID]; ok {
		if info, ok2 := h.feedbackByID[id]; ok2 {
			return info, true
		}
	}
	return feedbackInfo{}, false
}

func (h *BotHandler) popFeedbackByID(id string) (feedbackInfo, bool) {
	h.feedbackMu.Lock()
	defer h.feedbackMu.Unlock()
	info, ok := h.feedbackByID[id]
	if ok {
		delete(h.feedbackByID, id)
		// remove from latest if matches
		for uid, lid := range h.feedbackLatest {
			if lid == id {
				delete(h.feedbackLatest, uid)
				delete(h.feedbacks, uid)
				break
			}
		}
	}
	return info, ok
}

// Last suggestion helpers
func (h *BotHandler) setLastSuggestion(userID int64, text string) {
	h.suggestionMu.Lock()
	defer h.suggestionMu.Unlock()
	if h.lastSuggestion == nil {
		h.lastSuggestion = make(map[int64]string)
	}
	h.lastSuggestion[userID] = text
}

func (h *BotHandler) popLastSuggestion(userID int64) (string, bool) {
	h.suggestionMu.Lock()
	defer h.suggestionMu.Unlock()
	val, ok := h.lastSuggestion[userID]
	if ok {
		delete(h.lastSuggestion, userID)
	}
	return val, ok
}

func (h *BotHandler) getLastSuggestion(userID int64) (string, bool) {
	h.suggestionMu.RLock()
	defer h.suggestionMu.RUnlock()
	val, ok := h.lastSuggestion[userID]
	return val, ok
}

// clearUserCart - foydalanuvchi savatchasini tozalash (order rasmiylashtirilib tugagandan keyin)
func (h *BotHandler) clearUserCart(userID int64) {
	h.feedbackMu.Lock()
	defer h.feedbackMu.Unlock()

	// feedbackLatest dan ID ni topamiz
	if id, ok := h.feedbackLatest[userID]; ok {
		delete(h.feedbackByID, id)
		delete(h.feedbackLatest, userID)
	}
	delete(h.feedbacks, userID)

	// lastSuggestion ham tozalanadi
	h.suggestionMu.Lock()
	delete(h.lastSuggestion, userID)
	h.suggestionMu.Unlock()

	log.Printf("🧹 Savatcha tozalandi: userID=%d", userID)
}
