package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Pending change helpers
func (h *BotHandler) setPendingChange(userID int64, cr changeRequest) {
	h.changeMu.Lock()
	defer h.changeMu.Unlock()
	h.pendingChange[userID] = cr
}

func (h *BotHandler) popPendingChange(userID int64) (changeRequest, bool) {
	h.changeMu.Lock()
	defer h.changeMu.Unlock()
	cr, ok := h.pendingChange[userID]
	if ok {
		delete(h.pendingChange, userID)
	}
	return cr, ok
}

func (h *BotHandler) hasPendingChange(userID int64) bool {
	h.changeMu.RLock()
	defer h.changeMu.RUnlock()
	_, ok := h.pendingChange[userID]
	return ok
}

// Komponentni almashtirish uchun yuborilgan matnni qayta ishlash
func (h *BotHandler) handleChangeRequest(ctx context.Context, userID int64, username, text string, chatID int64) {
	lang := h.getUserLang(userID)
	cr, ok := h.popPendingChange(userID)
	if !ok {
		return
	}

	if !h.startProcessing(userID) {
		h.sendMessage(chatID, t(lang, "⏳ Oldingi so'rov yakunlanmoqda, iltimos kuting.", "⏳ Предыдущий запрос обрабатывается, подождите."))
		return
	}
	defer h.clearWaitingMessage(userID)
	defer h.endProcessing(userID)

	newValue := strings.TrimSpace(text)
	if newValue == "" {
		h.sendMessage(chatID, t(lang, "Iltimos, o'zgartirmoqchi bo'lgan komponent nomini yozing.", "Пожалуйста, укажите компонент для замены."))
		return
	}

	spec := cr.Spec
	switch cr.Component {
	case "cpu":
		spec.CPU = newValue
	case "gpu":
		spec.GPU = newValue
	case "ram":
		spec.RAM = newValue
	case "ssd":
		spec.Storage = newValue
	case "other":
		if spec.PCType != "" {
			spec.PCType = spec.PCType + " + " + newValue
		} else {
			spec.PCType = newValue
		}
	case "delete_other":
		// Deletion logic handled via prompt adjustment below
	}

	var prompt string
	if cr.Component == "delete_other" {
		prompt = fmt.Sprintf("Mijoz konfiguratsiyadan quyidagini o'chirib tashlamoqchi: %s. \nTalablar: maqsad=%s, budjet=%s, CPU=%s, RAM=%s, xotira=%s, GPU=%s.\nShu asosida yangi konfiguratsiya tuzib, narxlarni ko'rsating.\n\nFAQAT natijaviy konfiguratsiyani yozing, savol bermang va qo'shimcha izoh yozmang.\n\nNarxlarni quyidagi ko'rinishda alohida ko'rsating:\n- Konfiguratsiya narxi (faqat keys)\n- Monitor narxi (agar bo'lsa)\n- Periferiya narxi (agar bo'lsa)\n- Jami narx",
			newValue,
			nonEmpty(spec.PCType, "aniqlanmagan"),
			nonEmpty(spec.Budget, "aniqlanmagan"),
			nonEmpty(spec.CPU, "aniqlanmagan"),
			nonEmpty(spec.RAM, "aniqlanmagan"),
			nonEmpty(spec.Storage, "aniqlanmagan"),
			nonEmpty(spec.GPU, "aniqlanmagan"),
		)
	} else {
		prompt = fmt.Sprintf("Mijoz konfiguratsiyani o'zgartirmoqchi. Talablar: maqsad=%s, budjet=%s, CPU=%s, RAM=%s, xotira=%s, GPU=%s. Mijozning qo'shimcha talabi: %s. \n\nTALABLAR:\n1. Shu asosida faqat BITTA eng optimal konfiguratsiya tuzib bering.\n2. Ikkita variant taklif QILMANG. Faqat bitta aniq yechim bo'lsin.\n3. Savol bermang, hech qanday qo'shimcha izoh yozmang, faqat yakuniy konfiguratsiya va narxlarni qaytaring.\n4. Narxlarni quyidagi ko'rinishda alohida ko'rsating:\n- Konfiguratsiya narxi (faqat keys)\n- Monitor narxi (agar bo'lsa)\n- Periferiya narxi (agar bo'lsa)\n- Jami narx\n\n5. PSU da +200W zaxira qoldiring.",
			nonEmpty(spec.PCType, "aniqlanmagan"),
			nonEmpty(spec.Budget, "aniqlanmagan"),
			nonEmpty(spec.CPU, "aniqlanmagan"),
			nonEmpty(spec.RAM, "aniqlanmagan"),
			nonEmpty(spec.Storage, "aniqlanmagan"),
			nonEmpty(spec.GPU, "aniqlanmagan"),
			newValue,
		)
	}
	if lang == "ru" {
		prompt = "Отвечай только на русском языке.\n" + prompt
	}

	waitMsg, err := h.sendMessageWithResp(chatID, t(lang, "⏳ Iltimos, javobni kuting.", "⏳ Пожалуйста, подождите."))
	if err == nil {
		h.setWaitingMessage(userID, chatID, waitMsg.MessageID)
	}

	response, err := h.chatUseCase.ProcessConfigMessage(ctx, userID, username, prompt)
	if err != nil {
		log.Printf("Komponent almashtirish javobi xatosi: %v", err)
		h.sendMessage(chatID, "❌ Yangi konfiguratsiyani hisoblashda xatolik yuz berdi. Qayta urinib ko'ring.")
		return
	}

	response = h.applyCurrencyPreference(response)
	h.sendMessage(chatID, response)

	offerID := h.saveFeedback(userID, feedbackInfo{
		Summary:    fmt.Sprintf("Maqsad: %s, Budjet: %s, CPU: %s, RAM: %s, Xotira: %s, GPU: %s", spec.PCType, spec.Budget, spec.CPU, spec.RAM, spec.Storage, spec.GPU),
		ConfigText: response,
		Username:   username,
		ChatID:     chatID,
		Spec:       spec,
	})
	h.sendConfigFeedbackPrompt(chatID, userID, offerID)
	h.scheduleConfigReminder(userID, chatID, response)
}

// Komponent o'chirish tugmalari
func (h *BotHandler) sendDeleteComponentPrompt(chatID int64) {
	lang := h.getUserLang(chatID)
	msg := tgbotapi.NewMessage(chatID, t(lang, "Qaysi komponentni o'chirmoqchisiz?", "Какой компонент удалить?"))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🖥️ Monitor", "cfg_del_monitor"),
			tgbotapi.NewInlineKeyboardButtonData("🖱️ Peripherals", "cfg_del_peripherals"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Boshqa", "Другое"), "cfg_del_other"),
		),
	)
	_, _ = h.sendAndLog(msg)
}

// Komponent o'chirish logikasi
func (h *BotHandler) handleDeleteAction(ctx context.Context, userID int64, chatID int64, component string) {
	lang := h.getUserLang(userID)

	// Feedback ma'lumotlarini olish
	info, ok := h.getLatestFeedback(userID)
	if !ok {
		h.sendMessage(chatID, t(lang, "❌ Konfiguratsiya topilmadi.", "❌ Конфигурация не найдена."))
		return
	}

	if !h.startProcessing(userID) {
		h.sendMessage(chatID, t(lang, "⏳ Oldingi so'rov yakunlanmoqda, iltimos kuting.", "⏳ Предыдущий запрос обрабатывается, подождите."))
		return
	}
	defer h.clearWaitingMessage(userID)
	defer h.endProcessing(userID)

	spec := info.Spec
	var changeDesc string

	switch component {
	case "monitor":
		changeDesc = "Monitorni olib tashlash"
		// Monitor ma'lumotlarini tozalash kerak bo'lsa, spec da monitor maydoni yo'q, lekin promptda aytamiz
	case "peripherals":
		changeDesc = "Periferiyani olib tashlash"
	case "other":
		h.setPendingChange(userID, changeRequest{Component: "delete_other", Spec: spec})
		h.sendMessage(chatID, t(lang, "Nimani o'chirmoqchisiz? Yozib yuboring.", "Что удалить? Напишите."))
		return
	}

	prompt := fmt.Sprintf("Mijoz konfiguratsiyadan quyidagini o'chirib tashlamoqchi: %s. \nTalablar: maqsad=%s, budjet=%s, CPU=%s, RAM=%s, xotira=%s, GPU=%s.\n\nTALABLAR:\n1. Shu asosida yangi konfiguratsiya tuzib bering.\n2. Faqat BITTA variant bo'lsin.\n3. Narxlarni quyidagi ko'rinishda alohida ko'rsating:\n- Konfiguratsiya narxi (faqat keys)\n- Monitor narxi (agar bo'lsa)\n- Periferiya narxi (agar bo'lsa)\n- Jami narx\n\n4. PSU da +200W zaxira qoldiring.",
		changeDesc,
		nonEmpty(spec.PCType, "aniqlanmagan"),
		nonEmpty(spec.Budget, "aniqlanmagan"),
		nonEmpty(spec.CPU, "aniqlanmagan"),
		nonEmpty(spec.RAM, "aniqlanmagan"),
		nonEmpty(spec.Storage, "aniqlanmagan"),
		nonEmpty(spec.GPU, "aniqlanmagan"),
	)
	if lang == "ru" {
		prompt = "Отвечай только на русском языке.\n" + prompt
	}

	waitMsg, err := h.sendMessageWithResp(chatID, t(lang, "⏳ Iltimos, javobni kuting.", "⏳ Пожалуйста, подождите."))
	if err == nil {
		h.setWaitingMessage(userID, chatID, waitMsg.MessageID)
	}

	response, err := h.chatUseCase.ProcessConfigMessage(ctx, userID, info.Username, prompt)
	if err != nil {
		log.Printf("Komponent o'chirish javobi xatosi: %v", err)
		h.sendMessage(chatID, "❌ Yangi konfiguratsiyani hisoblashda xatolik yuz berdi. Qayta urinib ko'ring.")
		return
	}

	response = h.applyCurrencyPreference(response)
	h.sendMessage(chatID, response)

	offerID := h.saveFeedback(userID, feedbackInfo{
		Summary:    fmt.Sprintf("O'zgartirildi: %s. Maqsad: %s, Budjet: %s", changeDesc, spec.PCType, spec.Budget),
		ConfigText: response,
		Username:   info.Username,
		ChatID:     chatID,
		Spec:       spec,
	})
	h.sendConfigFeedbackPrompt(chatID, userID, offerID)
	h.scheduleConfigReminder(userID, chatID, response)
}
