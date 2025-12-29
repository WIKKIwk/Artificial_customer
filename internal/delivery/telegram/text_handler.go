package telegram

import (
	"context"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleDocumentMessage fayl yuborilganda

func (h *BotHandler) handleTextMessage(ctx context.Context, userID int64, username, text string, chatID int64, msg *tgbotapi.Message) {
	lang := h.getUserLang(userID)
	// BIRINCHI: Admin parol kutish rejimini tekshirish
	if h.isAwaitingPassword(userID) {
		h.handlePasswordInput(ctx, msg)
		return
	}
	if h.isAdminActive(userID) && msg != nil {
		h.addAdminMessage(userID, chatID, msg.MessageID)
	}

	// IKKINCHI: Konfiguratsiya sessiyasi (profil to'ldirishdan OLDIN!)
	hasConfig := h.hasConfigSession(userID)
	log.Printf("[text_handler] userID=%d, hasConfigSession=%v, text=%q", userID, hasConfig, text)
	if hasConfig {
		h.handleConfigFlow(ctx, userID, username, text, chatID, msg)
		return
	}

	// Profil to'ldirish bosqichi (start/langdan keyin)
	if stage := h.getProfileStage(userID); stage != "" {
		switch stage {
		case "need_name":
			name := strings.TrimSpace(text)
			lang := h.getUserLang(userID)
			if name == "" || !validateName(name) {
				h.sendMessage(chatID, t(lang, "Iltimos, to'liq ismingizni faqat harflar bilan yozing (kamida 2 ta harf).", "Пожалуйста, укажите имя только буквами (минимум 2 буквы)."))
				return
			}
			h.setProfile(userID, userProfile{Name: name})
			h.setProfileStage(userID, "need_phone")
			if pid := h.getProfilePrompt(userID); pid != 0 {
				h.deleteMessage(chatID, pid)
				h.clearProfilePrompt(userID)
			}
			npid := h.sendPhonePrompt(chatID, lang)
			if npid != 0 {
				h.setProfilePrompt(userID, npid)
			}
			h.deleteUserMessage(chatID, msg)
			return
		case "need_phone":
			lang := h.getUserLang(userID)
			phone := ""
			if msg != nil && msg.Contact != nil && msg.Contact.PhoneNumber != "" {
				phone = msg.Contact.PhoneNumber
			} else {
				phone = strings.TrimSpace(text)
			}
			if !validatePhoneNumber(phone) {
				h.sendMessage(chatID, t(lang, "Noto'g'ri telefon raqami! Kamida 7 ta raqam bo'lishi kerak. Masalan: +998901234567", "Неверный номер телефона! Минимум 7 цифр. Например: +998901234567"))
				h.sendPhoneRequest(chatID)
				return
			}
			h.setProfile(userID, userProfile{Phone: phone})
			h.setProfileStage(userID, "")
			if pid := h.getProfilePrompt(userID); pid != 0 {
				h.deleteMessage(chatID, pid)
				h.clearProfilePrompt(userID)
			}
			h.deleteUserMessage(chatID, msg)
			// Shaxsiy salomlashuv
			if prof, ok := h.getProfile(userID); ok {
				h.sendGreeting(chatID, lang, prof.Name)
			} else {
				h.sendGreeting(chatID, lang, "")
			}
			return
		}
	}

	// Agar buyurtma rasmiylashtirish jarayoni bo'lsa
	if h.hasOrderSession(userID) {
		h.handleOrderFlow(ctx, userID, username, text, chatID, msg)
		return
	}

	// Agar komponent almashtirish jarayoni bo'lsa
	if h.hasPendingChange(userID) {
		h.handleChangeRequest(ctx, userID, username, text, chatID)
		return
	}

	// Admin valyuta kursi kiritish jarayoni
	if h.handleAdminCurrencyInput(ctx, msg) {
		return
	}

	// Admin eslatma sozlash jarayoni
	if h.handleAdminReminderInput(ctx, msg) {
		return
	}

	// Admin add product son kiritish jarayoni
	if h.handleAddProductQuantityInput(ctx, msg) {
		return
	}

	// Qidiruv jarayoni
	if h.handleSearchInput(ctx, msg) {
		return
	}

	// Auto import interval kiritish (admin)
	if h.handleImportAutoInput(ctx, msg) {
		return
	}

	// Database (SheetMaster) setup jarayoni (admin panel)
	if h.handleSheetMasterSetupInput(ctx, msg) {
		return
	}

	// Admin sessiyasi borida (va aktiv jarayon yo'q) AI bilan yozishmalarni bloklaymiz
	if isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID); isAdmin {
		h.sendMessage(chatID, t(lang, "Admin paneldasiz. AI bilan suhbat uchun /logout qiling.", "Вы в админ-панели. Для общения с ИИ нажмите /logout."))
		return
	}

	// ✨ SMART ROUTER YONIQ - PC yig'ish so'rovlarini /configuratsiya ga yo'naltiradi
	h.handleSmartTextMessage(ctx, userID, username, text, chatID, msg, lang)
}
