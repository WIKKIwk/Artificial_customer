package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Admin eslatma sozlash jarayonini boshlash
func (h *BotHandler) beginReminderInput(userID, chatID int64) {
	h.reminderMu.Lock()
	h.reminderInput[userID] = &reminderInputState{stage: reminderStageNeedCount, chatID: chatID}
	current := len(h.reminderTemplates)
	h.reminderMu.Unlock()

	msg := fmt.Sprintf("Nechta eslatma kiritmoqchisiz? (1-%d)", reminderMaxCount)
	if current > 0 {
		msg += fmt.Sprintf("\nHozirgi eslatmalar: %d ta. Yangi ro'yxat eski ro'yxatni almashtiradi.", current)
	}
	h.sendMessage(chatID, msg)
}

func (h *BotHandler) getReminderInputState(userID int64) (*reminderInputState, bool) {
	h.reminderMu.RLock()
	state, ok := h.reminderInput[userID]
	h.reminderMu.RUnlock()
	return state, ok
}

func (h *BotHandler) setReminderInputState(userID int64, state *reminderInputState) {
	h.reminderMu.Lock()
	h.reminderInput[userID] = state
	h.reminderMu.Unlock()
}

func (h *BotHandler) clearReminderInputState(userID int64) {
	h.reminderMu.Lock()
	delete(h.reminderInput, userID)
	h.reminderMu.Unlock()
}

func (h *BotHandler) setReminderTemplates(msgs []string) {
	var cleaned []string
	for _, m := range msgs {
		if s := strings.TrimSpace(m); s != "" {
			cleaned = append(cleaned, s)
		}
	}
	h.reminderMu.Lock()
	h.reminderTemplates = cleaned
	h.reminderMu.Unlock()
}

func (h *BotHandler) getReminderSettings() (bool, time.Duration) {
	h.reminderMu.RLock()
	enabled := h.reminderEnabled
	interval := h.reminderInterval
	h.reminderMu.RUnlock()
	return enabled, interval
}

func clampReminderInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return defaultReminderInterval
	}
	if interval < minReminderInterval {
		return minReminderInterval
	}
	if interval > maxReminderInterval {
		return maxReminderInterval
	}
	return interval
}

func (h *BotHandler) setReminderInterval(interval time.Duration) time.Duration {
	interval = clampReminderInterval(interval)
	h.reminderMu.Lock()
	h.reminderInterval = interval
	h.reminderMu.Unlock()
	return interval
}

func (h *BotHandler) enableReminders() {
	h.reminderMu.Lock()
	h.reminderEnabled = true
	h.reminderMu.Unlock()
}

func (h *BotHandler) disableReminders() {
	h.reminderMu.Lock()
	h.reminderEnabled = false
	for userID, timer := range h.configReminder {
		timer.Stop()
		delete(h.configReminder, userID)
	}
	h.reminderMu.Unlock()
}

func (h *BotHandler) handleAdminReminderInput(ctx context.Context, message *tgbotapi.Message) bool {
	userID := message.From.ID
	state, ok := h.getReminderInputState(userID)
	if !ok {
		return false
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		h.sendMessage(message.Chat.ID, "Iltimos, bo'sh bo'lmagan matn kiriting.")
		return true
	}

	switch state.stage {
	case reminderStageNeedCount:
		n, err := strconv.Atoi(text)
		if err != nil || n <= 0 || n > reminderMaxCount {
			h.sendMessage(message.Chat.ID, fmt.Sprintf("Son kiritish kerak (1-%d).", reminderMaxCount))
			return true
		}
		state.expected = n
		state.messages = nil
		state.stage = reminderStageNeedMessages
		h.setReminderInputState(userID, state)
		h.sendMessage(message.Chat.ID, "1 - eslatmani kiriting.")
	case reminderStageNeedMessages:
		state.messages = append(state.messages, text)
		if len(state.messages) >= state.expected {
			h.setReminderTemplates(state.messages)
			h.clearReminderInputState(userID)
			h.sendMessage(message.Chat.ID, fmt.Sprintf("✅ %d ta eslatma saqlandi. Endi foydalanuvchi passiv bo'lsa, shu matnlar tasodifiy yuboriladi.", len(state.messages)))
			return true
		}
		h.setReminderInputState(userID, state)
		next := len(state.messages) + 1
		h.sendMessage(message.Chat.ID, fmt.Sprintf("%d - eslatmani kiriting.", next))
	}

	return true
}

// Reminder helpers
func (h *BotHandler) wasConfigReminded(chatID int64) bool {
	h.reminderMu.RLock()
	defer h.reminderMu.RUnlock()
	return h.configReminded[chatID]
}

func (h *BotHandler) markConfigReminded(chatID int64) {
	h.reminderMu.Lock()
	defer h.reminderMu.Unlock()
	h.configReminded[chatID] = true
}

var (
	defaultReminderUz = []string{
		"Bugun zakaz qilsangiz 5% chegirma beramiz.",
		"Bugun zakaz qilsangiz bonus sifatida sichqoncha qo'shamiz.",
		"Bugungi zakazlar uchun yetkazib berish bepul.",
	}
	defaultReminderRu = []string{
		"Если оформите сегодня — дадим скидку 5%.",
		"Если оформите сегодня — положим мышку в подарок.",
		"Сегодня оформление — доставка бесплатно.",
	}
)

func (h *BotHandler) pickReminderText(lang string) string {
	h.reminderMu.RLock()
	custom := append([]string(nil), h.reminderTemplates...)
	h.reminderMu.RUnlock()

	list := custom
	if len(list) == 0 {
		if lang == "ru" {
			list = defaultReminderRu
		} else {
			list = defaultReminderUz
		}
	}
	if len(list) == 0 {
		return ""
	}
	idx := int(time.Now().UnixNano()) % len(list)
	return list[idx]
}

// Config yakunlangandan keyin foydalanuvchini eslatib qo'yish (5 daqiqadan so'ng)
func (h *BotHandler) scheduleConfigReminder(userID, chatID int64, configText string) {
	if strings.TrimSpace(configText) == "" {
		return
	}
	text := configText
	h.reminderMu.Lock()
	enabled := h.reminderEnabled
	interval := clampReminderInterval(h.reminderInterval)
	if timer, ok := h.configReminder[userID]; ok {
		timer.Stop()
	}
	if !enabled {
		delete(h.configReminder, userID)
		h.reminderMu.Unlock()
		return
	}
	timer := time.AfterFunc(interval, func() {
		h.fireConfigReminder(userID, chatID, text)
	})
	h.configReminder[userID] = timer
	h.reminderMu.Unlock()
}

func (h *BotHandler) cancelConfigReminder(userID int64) {
	h.reminderMu.Lock()
	if timer, ok := h.configReminder[userID]; ok {
		timer.Stop()
		delete(h.configReminder, userID)
	}
	h.reminderMu.Unlock()
}

func (h *BotHandler) fireConfigReminder(userID, chatID int64, configText string) {
	h.reminderMu.Lock()
	delete(h.configReminder, userID)
	h.reminderMu.Unlock()

	h.reminderMu.RLock()
	enabled := h.reminderEnabled
	h.reminderMu.RUnlock()
	if !enabled {
		return
	}

	lang := h.getUserLang(userID)
	offer := h.pickReminderText(lang)
	if offer == "" {
		return
	}

	summary := formatOrderStatusSummary(configText)
	msg := t(lang, "Konfiguratsiyangiz tayyor! ", "Ваша сборка готова! ") + offer
	if summary != "" {
		label := t(lang, "Oxirgi taklif:", "Последняя подборка:")
		msg += "\n\n" + label + "\n" + summary
	}
	msg += "\n\n" + t(lang, "Buyurtma beramizmi? Tugmadan tanlang 👇", "Запустим оформление? Нажмите кнопку ниже 👇")

	h.setLastSuggestion(userID, configText)
	h.savePendingApproval(userID, pendingApproval{
		UserID:   userID,
		UserChat: chatID,
		Summary:  configText,
		Config:   configText,
		SentAt:   time.Now(),
	})

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "✅ Zakaz beraman", "✅ Оформить"), "order_yes"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "❌ Hozir emas", "❌ Не сейчас"), "order_no"),
		),
	)
	msgObj := tgbotapi.NewMessage(chatID, msg)
	msgObj.ReplyMarkup = kb
	if _, err := h.sendAndLog(msgObj); err != nil {
		log.Printf("Config reminder send failed: %v", err)
	}
}
