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

// handleCommand komandalarni qayta ishlash
func (h *BotHandler) handleCommand(ctx context.Context, message *tgbotapi.Message) {
	if message == nil || message.From == nil {
		return
	}
	userID := message.From.ID
	cmd := extractCommand(message)
	if cmd == "" {
		h.sendMessage(message.Chat.ID, "Noma'lum komanda. /help yordam uchun.")
		return
	}
	if h.isAdminActive(userID) {
		h.addAdminMessage(userID, message.Chat.ID, message.MessageID)
	}
	if strings.HasPrefix(cmd, "not_") {
		// Shortcut: /not_10, /not_on, /not_off, /not_text
		arg := strings.TrimPrefix(cmd, "not_")
		h.handleAdminReminderCommandWithArg(ctx, message, arg)
		return
	}

	switch cmd {
	case "start":
		// Har doim til tanlash menyusini yuborish
		h.trackWelcomeMessage(message.Chat.ID, message.MessageID)
		h.sendLanguageSelector(message.Chat.ID)
	case "help":
		h.sendMessage(message.Chat.ID, h.getHelpMessage())
	case "clear":
		h.handleClearCommand(ctx, message)
	case "history":
		h.handleHistoryCommand(ctx, message)
	case "admin":
		h.handleAdminCommand(ctx, message)
	case "logout":
		h.handleLogoutCommand(ctx, message)
	case "catalog":
		h.handleCatalogCommand(ctx, message)
	case "products":
		h.handleProductsCommand(ctx, message)
	case "configuratsiya":
		h.handleConfigCommand(ctx, message)
	case "chat":
		h.handleChatCommand(ctx, message)
	case "order":
		h.handleOrderCommand(ctx, message)
	case "ordersadmin":
		h.handleOrdersAdminCommand(ctx, message)
	case "online":
		h.handleOnlineCommand(ctx, message)
	case "user":
		h.handleUsersCommand(ctx, message)
	case "users":
		// Alias (admin menu oldin noto'g'ri yozilgan)
		h.handleUsersCommand(ctx, message)
	case "user_chat_history":
		h.handleUserChatHistoryCommand(ctx, message)
	case "userchathistory":
		h.handleUserChatHistoryCommand(ctx, message)
	case "about_user":
		h.handleAboutUserCommand(ctx, message)
	case "agout_user":
		h.handleAboutUserCommand(ctx, message)
	case "clean":
		h.handleCleanCommand(ctx, message)
	case "val":
		h.handleValCommand(ctx, message)
	case "sticker":
		h.handleStickerCommand(ctx, message)
	case "stats":
		h.handleStatsCommand(ctx, message)
	case "hisobot":
		h.handleHisobotCommand(ctx, message)
	case "savat":
		h.handleCartCommand(message.Chat.ID, message.From.ID)
	case "not":
		h.handleAdminReminderCommand(ctx, message)
	case "search":
		h.handleSearchCommand(ctx, message)
	case "add_product":
		h.handleAddProductCommand(ctx, message)
	case "remove_product":
		h.handleRemoveProductCommand(ctx, message)
	case "broadcast":
		h.handleBroadcastCommand(ctx, message)
	case "top":
		h.handleTopCommand(ctx, message)
	case "orders":
		h.handleOrdersAdminCommand(ctx, message)
	case "db_set":
		h.handleDBSetCommand(ctx, message)
	case "db_cancel":
		h.handleDBCancelCommand(ctx, message)
	case "db_clear":
		h.handleDBClearCommand(ctx, message)
	case "db_files":
		h.handleDBFilesCommand(ctx, message)
	case "db_config":
		h.handleDBConfigCommand(ctx, message)
	case "db_status":
		h.handleDBStatusCommand(ctx, message)
	case "db_sync":
		h.handleDBSyncCommand(ctx, message)
	case "database_select":
		h.handleDatabaseSelectCommand(ctx, message)
	case "import":
		// Import menu (admin)
		h.handleImportMenuCommand(ctx, message)
	case "imort":
		// Common typo alias: /imort -> /import_now
		h.handleDBSyncCommand(ctx, message)
	case "import_now":
		h.handleDBSyncCommand(ctx, message)
	case "import_auto":
		h.handleImportAutoCommand(ctx, message)
	case "import_auto_status":
		h.handleImportAutoStatusCommand(ctx, message)
	case "import_auto_off":
		h.handleImportAutoOffCommand(ctx, message)
	default:
		h.sendMessage(message.Chat.ID, "Noma'lum komanda. /help yordam uchun.")
	}
}

// extractCommand returns command name even if Telegram mark-up entity yo'q (fallback to raw slash text).
func extractCommand(msg *tgbotapi.Message) string {
	if msg == nil {
		return ""
	}
	if msg.IsCommand() {
		return msg.Command()
	}
	txt := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(txt, "/") {
		return ""
	}
	first := strings.Fields(txt)[0]
	first = strings.TrimPrefix(first, "/")
	if first == "" {
		return ""
	}
	parts := strings.SplitN(first, "@", 2)
	return parts[0]
}

// handleClearCommand tarixni tozalash
func (h *BotHandler) handleClearCommand(ctx context.Context, message *tgbotapi.Message) {
	err := h.chatUseCase.ClearHistory(ctx, message.From.ID)
	if err != nil {
		h.sendMessage(message.Chat.ID, "Tarixni tozalashda xatolik.")
		return
	}
	// ✅ Bug #6 fix: configReminded ni ham tozalash
	h.reminderMu.Lock()
	delete(h.configReminded, message.Chat.ID)
	h.reminderMu.Unlock()
	// Ehtiyot uchun eslatma taymerini tozalaymiz
	h.cancelConfigReminder(message.From.ID)

	h.sendMessage(message.Chat.ID, "✅ Chat tarixi tozalandi! Yangi suhbat boshlashingiz mumkin.")
}

// handleHistoryCommand tarixni ko'rsatish
func (h *BotHandler) handleHistoryCommand(ctx context.Context, message *tgbotapi.Message) {
	history, err := h.chatUseCase.GetHistory(ctx, message.From.ID)
	if err != nil {
		h.sendMessage(message.Chat.ID, "Tarixni olishda xatolik.")
		return
	}

	if len(history) == 0 {
		h.sendMessage(message.Chat.ID, "Sizning chat tarixingiz bo'sh.")
		return
	}

	var sb strings.Builder
	sb.WriteString("📜 *Chat tarixi:*\n\n")
	for i, msg := range history {
		sb.WriteString(fmt.Sprintf("*%d.* %s\n", i+1, msg.Text))
		if msg.Response != "" {
			sb.WriteString(fmt.Sprintf("↳ %s\n\n", msg.Response))
		}
	}

	h.sendMessageMarkdown(message.Chat.ID, sb.String())
}

// handleChatCommand - /chat: konfiguratsiya sessiyasidan chiqib, erkin chatga qaytish
func (h *BotHandler) handleChatCommand(ctx context.Context, message *tgbotapi.Message) {
	_ = ctx
	userID := message.From.ID
	lang := h.getUserLang(userID)

	if h.hasConfigSession(userID) {
		h.cancelConfigSession(userID)
		h.cancelConfigReminder(userID)
		h.sendMessage(message.Chat.ID, t(lang,
			"Konfiguratsiya jarayoni to'xtatildi. Endi erkin chatdamsiz. Savol yoki mahsulot bo'yicha yozishingiz mumkin.",
			"Сессия конфигурации остановлена. Теперь вы в свободном чате, можете задавать вопросы или искать товары."))
		return
	}

	// Agar konfiguratsiya yo'q bo'lsa, oddiy chat allaqachon aktiv
	h.sendMessage(message.Chat.ID, t(lang,
		"Allaqachon erkin chatdasiz. Mahsulot yoki savollarni yozishingiz mumkin. Yangi konfiguratsiya uchun /configuratsiya ni bosing.",
		"Вы уже в свободном чате. Можете задавать вопросы или искать товары. Для новой конфигурации нажмите /configuratsiya."))
}

// handleAdminCommand admin login boshlash yoki admin menyu ko'rsatish
func (h *BotHandler) handleAdminCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	// /admin xabarini sessiya tozalash uchun saqlab qo'yamiz
	if message != nil {
		h.addAdminMessage(userID, message.Chat.ID, message.MessageID)
		h.deleteCommandMessage(message)
	}
	// Parol so'rovini ham tozalash uchun chat bo'yicha sessiyani faollashtirib qo'yamiz
	h.startAdminSession(userID, message.Chat.ID)

	// Allaqachon admin bo'lsa, admin menyu ko'rsatish
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if isAdmin {
		h.setAdminAuthorized(userID, true)
		h.sendAdminMenu(userID, message.Chat.ID)
		return
	}

	// Admin emas bo'lsa, parol so'rash
	h.setAwaitingPassword(userID, true)
	h.sendMessage(message.Chat.ID, "🔐 Admin parolini kiriting:")
}

// handleAdminReminderCommand - eslatmalarni yoqish/o'chirish va intervalni sozlash (/not)
func (h *BotHandler) handleAdminReminderCommand(ctx context.Context, message *tgbotapi.Message) {
	h.handleAdminReminderCommandWithArg(ctx, message, "")
}

func (h *BotHandler) handleAdminReminderCommandWithArg(ctx context.Context, message *tgbotapi.Message, overrideArg string) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.startAdminSession(userID, message.Chat.ID)

	arg := strings.ToLower(strings.TrimSpace(overrideArg))
	if arg == "" {
		arg = strings.ToLower(strings.TrimSpace(message.CommandArguments()))
	}

	switch {
	case arg == "off" || arg == "0" || arg == "stop":
		h.disableReminders()
		h.sendMessage(message.Chat.ID, "🔕 Avtomatik eslatmalar o'chirildi. /not on bilan qayta yoqing.")
		return
	case arg == "on":
		h.enableReminders()
		_, interval := h.getReminderSettings()
		h.sendMessage(message.Chat.ID, fmt.Sprintf("🔔 Eslatmalar yoqildi. Joriy interval: %d daqiqa.", int(interval.Minutes())))
		return
	case arg == "text" || arg == "matn" || arg == "template" || arg == "templates":
		h.beginReminderInput(userID, message.Chat.ID)
		return
	case arg != "":
		if mins, err := strconv.Atoi(arg); err == nil && mins > 0 {
			interval := h.setReminderInterval(time.Duration(mins) * time.Minute)
			h.enableReminders()
			h.sendMessage(message.Chat.ID, fmt.Sprintf("⏱ Eslatma intervali %d daqiqaga o'rnatildi va yoqildi.", int(interval.Minutes())))
			return
		}
		h.sendMessage(message.Chat.ID, "❌ Noto'g'ri format. Masalan: /not_10, /not_on, /not_off, /not_text")
		return
	}

	enabled, interval := h.getReminderSettings()
	status := "🔔 Eslatmalar yoqilgan."
	if !enabled {
		status = "🔕 Eslatmalar o'chirilgan."
	}
	h.sendMessage(message.Chat.ID, fmt.Sprintf(`%s
Joriy interval: %d daqiqa.

Qo'llanma:
/not_10 — har 10 daqiqada
/not_on — yoqish
/not_off — o'chirish
/not_text — eslatma matnlarini almashtirish`, status, int(interval.Minutes())))
}

// handlePasswordInput parol kiritilganini qayta ishlash
func (h *BotHandler) handlePasswordInput(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	password := strings.TrimSpace(message.Text)

	// Parol kutish rejimini o'chirish
	h.setAwaitingPassword(userID, false)

	// Xabarni o'chirish (xavfsizlik uchun)
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	_, _ = h.sendAndLog(deleteMsg)
	// Trekda ham saqlaymiz (logout paytida o'chirish uchun)
	h.addAdminMessage(userID, message.Chat.ID, message.MessageID)

	// Login urinishi
	success, err := h.adminUseCase.Login(ctx, userID, password)
	if err != nil {
		log.Printf("Login error: %v", err)
		h.sendMessage(message.Chat.ID, "❌ Login xatosi yuz berdi.")
		return
	}

	if !success {
		h.setAwaitingPassword(userID, true)
		h.sendMessage(message.Chat.ID, "❌ Noto'g'ri parol. Qayta kiriting:")
		return
	}

	// Muvaffaqiyatli login - admin menyu ko'rsatish
	h.setAdminAuthorized(userID, true)

	// Admin menu bilan birgalikda muvaffaqiyat xabarini yuborish
	welcomeMsg := "✅ Muvaffaqiyatli tizimga kirdingiz!\n\n" + h.getAdminMenuText()
	msg := tgbotapi.NewMessage(message.Chat.ID, welcomeMsg)
	msg.ParseMode = "Markdown"
	sent, err := h.sendAndLog(msg)
	if err != nil {
		log.Printf("Admin menu send error: %v", err)
		h.sendMessage(message.Chat.ID, "✅ Tizimga kirdingiz! /admin - Dashboard")
		return
	}
	h.setAdminMenuMessage(userID, message.Chat.ID, sent.MessageID)
	h.trackAdminMessage(message.Chat.ID, sent.MessageID)
}
