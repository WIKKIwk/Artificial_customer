package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Admin session message tracking
func (h *BotHandler) startAdminSession(userID, chatID int64) {
	h.adminMsgMu.Lock()
	if !h.adminActive[userID] {
		h.adminMessages[userID] = nil
	}
	h.adminActive[userID] = true
	if chatID > 0 {
		h.adminActive[chatID] = true
	}
	h.adminMsgMu.Unlock()
}

func (h *BotHandler) endAdminSession(userID, chatID int64) {
	h.adminMsgMu.Lock()
	delete(h.adminActive, userID)
	if chatID > 0 {
		delete(h.adminActive, chatID)
	}
	delete(h.adminMessages, userID)
	if chatID > 0 {
		delete(h.adminMessages, chatID)
	}
	h.adminMsgMu.Unlock()
}

func (h *BotHandler) trackAdminMessage(chatID int64, msgID int) {
	if chatID <= 0 {
		return
	}
	h.adminMsgMu.Lock()
	if h.adminActive[chatID] {
		h.adminMessages[chatID] = append(h.adminMessages[chatID], adminMessage{chatID: chatID, msgID: msgID})
	}
	h.adminMsgMu.Unlock()
}

// cleanupAdminMessagesAny userID yoki chatID bo'yicha yig'ilgan barcha admin xabarlarini o'chiradi
func (h *BotHandler) cleanupAdminMessagesAny(userID, chatID int64) {
	h.adminMsgMu.Lock()
	msgsUser := h.adminMessages[userID]
	msgsChat := h.adminMessages[chatID]
	delete(h.adminMessages, userID)
	delete(h.adminMessages, chatID)
	h.adminMsgMu.Unlock()

	seen := make(map[string]struct{})
	for _, m := range append(msgsUser, msgsChat...) {
		key := fmt.Sprintf("%d-%d", m.chatID, m.msgID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		h.deleteMessage(m.chatID, m.msgID)
	}
}

func (h *BotHandler) isAdminActive(userID int64) bool {
	h.adminMsgMu.RLock()
	active := h.adminActive[userID]
	h.adminMsgMu.RUnlock()
	return active
}

func (h *BotHandler) setAdminAuthorized(userID int64, authorized bool) {
	h.adminAuthMu.Lock()
	if authorized {
		h.adminAuthorized[userID] = true
	} else {
		delete(h.adminAuthorized, userID)
	}
	h.adminAuthMu.Unlock()
}

func (h *BotHandler) isAdminAuthorized(userID int64) bool {
	h.adminAuthMu.RLock()
	defer h.adminAuthMu.RUnlock()
	return h.adminAuthorized[userID]
}

func (h *BotHandler) setAwaitingSearch(userID int64, awaiting bool) {
	h.searchMu.Lock()
	if awaiting {
		h.searchAwait[userID] = true
	} else {
		delete(h.searchAwait, userID)
	}
	h.searchMu.Unlock()
}

func (h *BotHandler) isAwaitingSearch(userID int64) bool {
	h.searchMu.RLock()
	defer h.searchMu.RUnlock()
	return h.searchAwait[userID]
}

func (h *BotHandler) addAdminMessage(userID, chatID int64, msgID int) {
	if msgID == 0 {
		return
	}
	h.adminMsgMu.Lock()
	h.adminMessages[userID] = append(h.adminMessages[userID], adminMessage{chatID: chatID, msgID: msgID})
	if chatID > 0 {
		h.adminMessages[chatID] = append(h.adminMessages[chatID], adminMessage{chatID: chatID, msgID: msgID})
	}
	h.adminMsgMu.Unlock()
}

// Admin commands
func (h *BotHandler) handleLogoutCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "Siz admin emassiz.")
		return
	}

	if err := h.adminUseCase.Logout(ctx, userID); err != nil {
		h.sendMessage(message.Chat.ID, "Logout xatosi.")
		return
	}

	h.deleteCommandMessage(message)
	h.sendMessage(message.Chat.ID, "✅ Admin paneldan chiqdingiz.")
	h.clearReminderInputState(userID)
	h.setAdminAuthorized(userID, false)
	h.setAwaitingCurrencyRate(userID, false)
	h.setAwaitingSearch(userID, false)
	h.setAwaitingUserHistory(userID, false)
	h.clearAddProductState(userID)
	h.cleanupAdminMessagesAny(userID, message.Chat.ID)
	h.endAdminSession(userID, message.Chat.ID)
}

func (h *BotHandler) handleCleanCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	if err := h.adminUseCase.CleanAll(ctx, userID); err != nil {
		log.Printf("Clean error: %v", err)
		h.sendMessage(message.Chat.ID, "❌ Tozalashda xatolik yuz berdi.")
		return
	}

	h.sendMessage(message.Chat.ID, "🧹 Barcha mahsulotlar va chat tarixlari tozalandi. Bot yangilanib boshlandi.")
}

func (h *BotHandler) handleValCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if isAdmin {
		h.setAdminAuthorized(userID, true)
	}
	if !isAdmin && !h.isAdminAuthorized(userID) && !h.isAdminActive(userID) {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("so'm", "val_sum"),
			tgbotapi.NewInlineKeyboardButtonData("$", "val_usd"),
		),
	)
	msg := tgbotapi.NewMessage(message.Chat.ID, "Valyuta rejimini tanlang:")
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(message.Chat.ID, sent.MessageID)
	}
}

func (h *BotHandler) handleCatalogCommand(ctx context.Context, message *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, message.From.ID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	info, err := h.adminUseCase.GetCatalogInfo(ctx)
	if err != nil {
		h.sendMessage(message.Chat.ID, "❌ Katalog topilmadi. Excel fayl yuklang.")
		return
	}

	h.sendMessage(message.Chat.ID, info)
}

func (h *BotHandler) handleProductsCommand(ctx context.Context, message *tgbotapi.Message) {
	products, err := h.productUseCase.GetAll(ctx)
	if err != nil || len(products) == 0 {
		h.sendMessage(message.Chat.ID, "❌ Mahsulotlar topilmadi.")
		return
	}

	productsText, err := h.productUseCase.GetProductsAsText(ctx)
	if err != nil {
		h.sendMessage(message.Chat.ID, "❌ Mahsulotlarni yuklashda xatolik.")
		return
	}

	if len(productsText) > 4000 {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("📦 Jami %d ta mahsulot mavjud. Katalog juda katta, AI bilan savdo qilishingiz mumkin.", len(products)))
	} else {
		h.sendMessage(message.Chat.ID, productsText)
	}
}

func (h *BotHandler) handleOrdersAdminCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	// Parse arguments: /orders or /orders 20 or /orders all
	parts := strings.Fields(message.Text)
	limit := 10
	if len(parts) > 1 {
		if parts[1] == "all" {
			limit = 1000
		} else if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	orders := h.listRecentOrders(limit)
	if len(orders) == 0 {
		h.sendMessage(message.Chat.ID, "🧾 Hali buyurtmalar yo'q.")
		return
	}

	lang := h.getUserLang(userID)

	// Group orders by status for better overview
	statusGroups := make(map[string][]orderStatusInfo)
	for _, ord := range orders {
		status := ord.Status
		if status == "" {
			status = "processing"
		}
		statusGroups[status] = append(statusGroups[status], ord)
	}

	var sb strings.Builder
	sb.WriteString("🧾 *Buyurtmalar boshqaruvi*\n\n")
	sb.WriteString(fmt.Sprintf("📊 Jami: %d ta buyurtma\n", len(orders)))

	// Status summary
	if len(statusGroups["processing"]) > 0 {
		sb.WriteString(fmt.Sprintf("⏳ Jarayonda: %d ta\n", len(statusGroups["processing"])))
	}
	if len(statusGroups["ready_delivery"]) > 0 {
		sb.WriteString(fmt.Sprintf("📦 Tayyor (dostavka): %d ta\n", len(statusGroups["ready_delivery"])))
	}
	if len(statusGroups["ready_pickup"]) > 0 {
		sb.WriteString(fmt.Sprintf("🏪 Tayyor (pickup): %d ta\n", len(statusGroups["ready_pickup"])))
	}
	if len(statusGroups["onway"]) > 0 {
		sb.WriteString(fmt.Sprintf("🚚 Yo'lda: %d ta\n", len(statusGroups["onway"])))
	}
	if len(statusGroups["delivered"]) > 0 {
		sb.WriteString(fmt.Sprintf("✅ Yakunlangan: %d ta\n", len(statusGroups["delivered"])))
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("So'nggi %d ta buyurtma:\n\n", len(orders)))

	h.sendMessageMarkdown(message.Chat.ID, sb.String())

	// Send each order with interactive buttons
	for i, ord := range orders {
		if i >= 20 {
			// Limit to 20 orders with buttons to avoid spam
			remaining := len(orders) - 20
			h.sendMessage(message.Chat.ID, fmt.Sprintf("... va yana %d ta buyurtma.\nBarcha buyurtmalarni ko'rish uchun: /orders all", remaining))
			break
		}
		h.sendOrderDetailsWithButtons(message.Chat.ID, ord, i+1, lang)
	}
}

// sendOrderDetailsWithButtons sends order details with status change buttons
func (h *BotHandler) sendOrderDetailsWithButtons(chatID int64, ord orderStatusInfo, index int, lang string) {
	var sb strings.Builder
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("*#%d | OrderID: %s*\n", index, ord.OrderID))
	sb.WriteString(fmt.Sprintf("📌 Holat: *%s*\n", statusLabel(ord.Status, lang)))

	if ord.Username != "" {
		sb.WriteString(fmt.Sprintf("👤 User: @%s\n", ord.Username))
	}
	if ord.Phone != "" {
		sb.WriteString(fmt.Sprintf("📞 Tel: %s\n", ord.Phone))
	}
	if ord.Total != "" {
		sb.WriteString(fmt.Sprintf("💰 Jami: %s\n", ord.Total))
	}
	if ord.Delivery != "" {
		sb.WriteString(fmt.Sprintf("🚚 Yetkazish: %s\n", deliveryDisplay(ord.Delivery, lang)))
	}
	if !ord.CreatedAt.IsZero() {
		elapsed := time.Since(ord.CreatedAt)
		sb.WriteString(fmt.Sprintf("🕒 Vaqt: %s", ord.CreatedAt.Format("2006-01-02 15:04")))
		if elapsed < 24*time.Hour {
			sb.WriteString(fmt.Sprintf(" (%s oldin)", formatDuration(elapsed)))
		}
		sb.WriteString("\n")
	}

	detail := nonEmpty(ord.StatusSummary, formatOrderStatusSummary(ord.Summary))
	if detail != "" {
		if len(detail) > 200 {
			detail = detail[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n📝 Tafsilot:\n%s\n", detail))
	}

	// Create status change buttons based on current status
	var buttons [][]tgbotapi.InlineKeyboardButton
	currentStatus := ord.Status
	if currentStatus == "" {
		currentStatus = "processing"
	}

	switch currentStatus {
	case "processing":
		buttons = [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("📦 Tayyor (dostavka)", fmt.Sprintf("ordstat_ready_delivery:%s", ord.OrderID)),
				tgbotapi.NewInlineKeyboardButtonData("🏪 Tayyor (pickup)", fmt.Sprintf("ordstat_ready_pickup:%s", ord.OrderID)),
			},
		}
	case "ready_delivery":
		buttons = [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("🚚 Yo'lga chiqarish", fmt.Sprintf("ordstat_onway:%s", ord.OrderID)),
			},
			{
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Qayta ishlash", fmt.Sprintf("ordstat_processing:%s", ord.OrderID)),
			},
		}
	case "ready_pickup":
		buttons = [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("✅ Yakunlash", fmt.Sprintf("ordstat_delivered:%s", ord.OrderID)),
			},
			{
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Qayta ishlash", fmt.Sprintf("ordstat_processing:%s", ord.OrderID)),
			},
		}
	case "onway":
		buttons = [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("✅ Yetkazildi", fmt.Sprintf("ordstat_delivered:%s", ord.OrderID)),
			},
			{
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Tayyor holatga qaytarish", fmt.Sprintf("ordstat_ready_delivery:%s", ord.OrderID)),
			},
		}
	case "delivered":
		buttons = [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("🔄 Qayta ochish", fmt.Sprintf("ordstat_processing:%s", ord.OrderID)),
			},
		}
	}

	// Add view customer button
	if ord.UserChat != 0 {
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("👤 Mijozga xabar", fmt.Sprintf("ordmsg:%s:%d", ord.OrderID, ord.UserChat)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	if len(buttons) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	}
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(chatID, sent.MessageID)
	}
}

// handleOrderStatusChange changes order status and notifies user
func (h *BotHandler) handleOrderStatusChange(ctx context.Context, chatID, adminID int64, orderID, newStatus string) {
	// Check admin
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}

	// Find order
	order := h.findOrderByID(orderID)
	if order == nil {
		h.sendMessage(chatID, "❌ Buyurtma topilmadi.")
		return
	}

	oldStatus := order.Status
	order.Status = newStatus

	// Update order in storage
	h.updateOrderStatus(orderID, newStatus)

	// Send confirmation to admin
	h.sendMessage(chatID, fmt.Sprintf("✅ Buyurtma %s holati o'zgartirildi:\n%s → %s",
		orderID,
		statusLabel(oldStatus, "uz"),
		statusLabel(newStatus, "uz")))

	// Notify user if available
	if order.UserChat != 0 {
		userLang := h.getUserLang(order.UserID)
		var userMsg string
		switch newStatus {
		case "ready_delivery":
			userMsg = t(userLang,
				"📦 Buyurtmangiz tayyor! Tez orada yetkazib beriladi.\nOrderID: "+orderID,
				"📦 Ваш заказ готов! Скоро доставим.\nOrderID: "+orderID)
		case "ready_pickup":
			userMsg = t(userLang,
				"🏪 Buyurtmangiz tayyor! Olib ketishingiz mumkin.\nOrderID: "+orderID,
				"🏪 Ваш заказ готов! Можете забрать.\nOrderID: "+orderID)
		case "onway":
			userMsg = t(userLang,
				"🚚 Buyurtmangiz yo'lda! Tez orada yetkaziladi.\nOrderID: "+orderID,
				"🚚 Ваш заказ в пути! Скоро доставим.\nOrderID: "+orderID)
		case "delivered":
			userMsg = t(userLang,
				"✅ Buyurtmangiz yetkazildi! Xarid uchun rahmat!\nOrderID: "+orderID,
				"✅ Ваш заказ доставлен! Спасибо за покупку!\nOrderID: "+orderID)
		case "processing":
			userMsg = t(userLang,
				"⏳ Buyurtmangiz qayta ishlanmoqda.\nOrderID: "+orderID,
				"⏳ Ваш заказ обрабатывается.\nOrderID: "+orderID)
		default:
			userMsg = t(userLang,
				fmt.Sprintf("🔔 Buyurtma holati o'zgardi: %s\nOrderID: %s", statusLabel(newStatus, userLang), orderID),
				fmt.Sprintf("🔔 Статус заказа изменен: %s\nOrderID: %s", statusLabel(newStatus, userLang), orderID))
		}
		h.sendMessage(order.UserChat, userMsg)
	}
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "bir daqiqa"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		return fmt.Sprintf("%d daqiqa", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		return fmt.Sprintf("%d soat", hours)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%d kun", days)
}

func (h *BotHandler) handleOnlineCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.deleteCommandMessage(message)

	lang := h.getUserLang(userID)
	text := h.buildOnlineStats(lang)
	kb := h.onlineStatsKeyboard(lang)

	if info, ok := h.getAdminMenuMessage(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(info.chatID, info.messageID, text, kb)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = kb
	sent, err := h.sendAndLog(msg)
	if err != nil {
		log.Printf("online send error: %v", err)
		return
	}
	h.setAdminMenuMessage(userID, message.Chat.ID, sent.MessageID)
	h.trackAdminMessage(message.Chat.ID, sent.MessageID)
}

func (h *BotHandler) handleUsersCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.deleteCommandMessage(message)

	lang := h.getUserLang(userID)
	text := h.buildUsersStats(lang)
	kb := h.usersStatsKeyboard(lang)

	if info, ok := h.getAdminMenuMessage(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(info.chatID, info.messageID, text, kb)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = kb
	sent, err := h.sendAndLog(msg)
	if err != nil {
		log.Printf("users send error: %v", err)
		return
	}
	h.setAdminMenuMessage(userID, message.Chat.ID, sent.MessageID)
	h.trackAdminMessage(message.Chat.ID, sent.MessageID)
}

func (h *BotHandler) handleUserChatHistoryCommand(ctx context.Context, message *tgbotapi.Message) {
	if message == nil || message.From == nil {
		return
	}
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}
	if h.chatStore == nil {
		h.sendMessage(message.Chat.ID, "❌ Chat tarixini saqlash yoqilmagan.")
		return
	}

	args := strings.Fields(message.Text)
	if len(args) < 2 {
		h.setAwaitingUserHistory(userID, true)
		query := ""
		btn := tgbotapi.InlineKeyboardButton{
			Text:                         "🔎 Qidirish",
			SwitchInlineQueryCurrentChat: &query,
		}
		msg := tgbotapi.NewMessage(message.Chat.ID, "👤 Qaysi user chat tarixini ko'rmoqchisiz? Pastdagi qidiruv tugmasini bosing.")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		if sent, err := h.sendAndLog(msg); err == nil {
			h.trackAdminMessage(message.Chat.ID, sent.MessageID)
		}
		return
	}

	h.setAwaitingUserHistory(userID, false)
	target := strings.TrimSpace(args[1])
	target = strings.TrimPrefix(target, "@")
	limit := 20
	if len(args) >= 3 {
		if n, err := strconv.Atoi(args[2]); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	h.sendUserChatHistory(ctx, message.Chat.ID, target, limit)
}

func (h *BotHandler) sendUserChatHistory(ctx context.Context, chatID int64, target string, limit int) {
	if strings.TrimSpace(target) == "" {
		h.sendMessage(chatID, "❌ Foydalanuvchi topilmadi.")
		return
	}
	if h.chatStore == nil {
		h.sendMessage(chatID, "❌ Chat tarixini saqlash yoqilmagan.")
		return
	}
	if limit <= 0 {
		limit = userHistoryDefaultLimit
	}

	ctxFetch, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var logs []chatLogMessage
	var err error
	if id, ok := parseUserID(target); ok {
		logs, err = h.chatStore.ListByUser(ctxFetch, id, limit)
	} else {
		logs, err = h.chatStore.ListByUsername(ctxFetch, target, limit)
	}
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("❌ Chat tarixini olishda xatolik: %v", err))
		return
	}
	if len(logs) == 0 {
		h.sendMessage(chatID, "📭 Chat tarixi topilmadi.")
		return
	}

	startedAt := h.botStartedAt
	if startedAt.IsZero() {
		startedAt = time.Time{}
	}
	filtered := logs[:0]
	for _, item := range logs {
		if !startedAt.IsZero() && item.CreatedAt.Before(startedAt) {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		h.sendMessage(chatID, "📭 Chat tarixi topilmadi.")
		return
	}

	headerUser := filtered[0].Username
	if headerUser == "" {
		headerUser = target
	}
	entries := make([]string, 0, len(filtered))
	for _, item := range filtered {
		if shouldSkipHistoryMessage(item) {
			continue
		}
		role := "👤"
		if item.Direction == "bot" {
			role = "🤖"
		}
		ts := item.CreatedAt.In(time.Local).Format("2006-01-02 15:04")
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		entries = append(entries, fmt.Sprintf("%s [%s] %s", role, ts, text))
	}
	if len(entries) == 0 {
		h.sendMessage(chatID, "📭 Chat tarixi topilmadi.")
		return
	}

	chunks := buildUserHistoryChunks(headerUser, filtered[0].UserID, entries)
	if len(chunks) == 0 {
		h.sendMessage(chatID, "📭 Chat tarixi topilmadi.")
		return
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Yopish", "user_history_close"),
		),
	)

	sentMsgs := make([]adminMessage, 0, len(chunks))
	lastMsgID := 0
	for i, chunk := range chunks {
		msg := tgbotapi.NewMessage(chatID, chunk)
		if i == len(chunks)-1 {
			msg.ReplyMarkup = kb
		}
		sent, err := h.sendAndLog(msg)
		if err != nil {
			log.Printf("user chat history send error: %v", err)
			continue
		}
		sentMsgs = append(sentMsgs, adminMessage{chatID: chatID, msgID: sent.MessageID})
		h.trackAdminMessage(chatID, sent.MessageID)
		lastMsgID = sent.MessageID
	}
	if lastMsgID != 0 && len(sentMsgs) > 0 {
		h.storeUserHistoryMessages(chatID, lastMsgID, sentMsgs)
	}
}

func shouldSkipHistoryMessage(item chatLogMessage) bool {
	if item.Direction != "bot" {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(item.Text))
	if text == "" {
		return true
	}
	if strings.Contains(text, "iltimos, javobni kuting") || strings.Contains(text, "пожалуйста, подождите") {
		return true
	}
	return false
}

func buildUserHistoryChunks(headerUser string, userID int64, entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	const maxLen = 3900
	header := fmt.Sprintf("🗂️ Chat tarixi: %s (ID: %d)", headerUser, userID)
	chunks := make([]string, 0, 2)
	current := header

	for _, entry := range entries {
		if len(entry) > maxLen {
			if strings.TrimSpace(current) != "" {
				chunks = append(chunks, current)
				current = ""
			}
			parts := splitIntoChunks(entry, maxLen)
			chunks = append(chunks, parts...)
			continue
		}
		if current == "" {
			current = entry
			continue
		}
		sep := "\n\n"
		if len(current)+len(sep)+len(entry) > maxLen {
			chunks = append(chunks, current)
			current = entry
			continue
		}
		current += sep + entry
	}
	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func userHistoryKey(chatID int64, msgID int) string {
	return fmt.Sprintf("%d:%d", chatID, msgID)
}

func (h *BotHandler) storeUserHistoryMessages(chatID int64, msgID int, msgs []adminMessage) {
	if chatID == 0 || msgID == 0 || len(msgs) == 0 {
		return
	}
	key := userHistoryKey(chatID, msgID)
	h.userHistoryMsgMu.Lock()
	h.userHistoryMsgs[key] = msgs
	h.userHistoryMsgMu.Unlock()
}

func (h *BotHandler) popUserHistoryMessages(chatID int64, msgID int) []adminMessage {
	key := userHistoryKey(chatID, msgID)
	h.userHistoryMsgMu.Lock()
	msgs := h.userHistoryMsgs[key]
	delete(h.userHistoryMsgs, key)
	h.userHistoryMsgMu.Unlock()
	return msgs
}

func parseUserID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, id > 0
}

func (h *BotHandler) buildOnlineStats(lang string) string {
	now := time.Now()
	var last5m, last1h, last24h int

	type seenInfo struct {
		id   int64
		at   time.Time
		name string
	}
	var list []seenInfo

	h.lastSeenMu.RLock()
	h.nameMu.RLock()
	for id, ts := range h.lastSeen {
		diff := now.Sub(ts)
		if diff <= 5*time.Minute {
			last5m++
		}
		if diff <= time.Hour {
			last1h++
		}
		if diff <= 24*time.Hour {
			last24h++
		}
		list = append(list, seenInfo{
			id:   id,
			at:   ts,
			name: h.lastName[id],
		})
	}
	h.nameMu.RUnlock()
	total := len(list)
	h.lastSeenMu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].at.After(list[j].at)
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n", t(lang, "👥 Onlayn statistika", "👥 Онлайн статистика")))
	sb.WriteString(fmt.Sprintf("%s: %d\n", t(lang, "⏱️ Oxirgi 5 daqiqa", "⏱️ Последние 5 минут"), last5m))
	sb.WriteString(fmt.Sprintf("%s: %d\n", t(lang, "🕒 Oxirgi 1 soat", "🕒 Последний час"), last1h))
	sb.WriteString(fmt.Sprintf("%s: %d\n", t(lang, "📅 Oxirgi 24 soat", "📅 Последние 24 часа"), last24h))
	sb.WriteString(fmt.Sprintf("%s: %d\n", t(lang, "📊 Jami kuzatuv", "📊 Всего в базе"), total))
	sb.WriteString("\n")
	sb.WriteString(t(lang, "🟢 So'nggi faol foydalanuvchilar:\n", "🟢 Последние активные пользователи:\n"))
	maxShow := 20
	shown := 0
	for _, item := range list {
		if now.Sub(item.at) > 24*time.Hour {
			break
		}
		if shown >= maxShow {
			break
		}
		name := strings.TrimSpace(item.name)
		if name != "" && !strings.HasPrefix(name, "@") {
			name = "@" + name
		}
		if name == "" {
			name = fmt.Sprintf("ID:%d", item.id)
		} else {
			name = fmt.Sprintf("%s (ID:%d)", name, item.id)
		}
		sb.WriteString(fmt.Sprintf("• %s — %s, %s: %s\n", name, formatAgo(now.Sub(item.at), lang), t(lang, "oxirgi", "посл."), formatClock(item.at)))
		shown++
	}
	if shown == 0 {
		sb.WriteString(t(lang, "Hali faollik qayd etilmagan.", "Пока нет активности.") + "\n")
	}
	return sb.String()
}

func (h *BotHandler) buildUsersStats(lang string) string {
	now := time.Now()

	type seenInfo struct {
		id   int64
		at   time.Time
		name string
	}
	var list []seenInfo

	h.lastSeenMu.RLock()
	h.nameMu.RLock()
	for id, ts := range h.lastSeen {
		list = append(list, seenInfo{
			id:   id,
			at:   ts,
			name: h.lastName[id],
		})
	}
	h.nameMu.RUnlock()
	total := len(list)
	h.lastSeenMu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].at.After(list[j].at)
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n", t(lang, "👥 Foydalanuvchilar ro'yxati", "👥 Список пользователей")))
	sb.WriteString(fmt.Sprintf("%s: %d\n\n", t(lang, "Jami foydalanuvchi", "Всего пользователей"), total))

	sb.WriteString(t(lang, "🟢 Oxirgi faollar:\n", "🟢 Последние активные:\n"))
	maxShow := 30
	shown := 0
	for _, item := range list {
		if shown >= maxShow {
			break
		}
		name := strings.TrimSpace(item.name)
		if name != "" && !strings.HasPrefix(name, "@") {
			name = "@" + name
		}
		if name == "" {
			name = fmt.Sprintf("ID:%d", item.id)
		} else {
			name = fmt.Sprintf("%s (ID:%d)", name, item.id)
		}
		sb.WriteString(fmt.Sprintf("• %s — %s (%s)\n", name, formatAgo(now.Sub(item.at), lang), formatClock(item.at)))
		shown++
	}
	if shown == 0 {
		sb.WriteString(t(lang, "Hali foydalanuvchi yozmagan.", "Пока нет сообщений от пользователей.") + "\n")
	}

	return sb.String()
}

func (h *BotHandler) onlineStatsKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🔄 Yangilash", "🔄 Обновить"), "online_refresh"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "online_back"),
		),
	)
}

func (h *BotHandler) usersStatsKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🔄 Yangilash", "🔄 Обновить"), "users_refresh"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "online_back"),
		),
	)
}

func (h *BotHandler) handleOnlineRefreshCallback(ctx context.Context, chatID, userID int64, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		return
	}
	lang := h.getUserLang(userID)
	text := h.buildOnlineStats(lang)
	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, h.onlineStatsKeyboard(lang))
	if _, err := h.bot.Send(edit); err == nil {
		h.setAdminMenuMessage(userID, chatID, msg.MessageID)
	}
}

func (h *BotHandler) handleUsersRefreshCallback(ctx context.Context, chatID, userID int64, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		return
	}
	lang := h.getUserLang(userID)
	text := h.buildUsersStats(lang)
	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, h.usersStatsKeyboard(lang))
	if _, err := h.bot.Send(edit); err == nil {
		h.setAdminMenuMessage(userID, chatID, msg.MessageID)
	}
}

func (h *BotHandler) handleOnlineBackCallback(chatID, userID int64, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	isAdmin, _ := h.adminUseCase.IsAdmin(context.Background(), userID)
	if !isAdmin {
		return
	}
	text := h.getAdminMenuText()
	edit := tgbotapi.NewEditMessageText(chatID, msg.MessageID, text)
	edit.ParseMode = "Markdown"
	if _, err := h.bot.Send(edit); err == nil {
		h.setAdminMenuMessage(userID, chatID, msg.MessageID)
	}
}

func (h *BotHandler) sendAdminMenu(userID, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, h.getAdminMenuText())
	msg.ParseMode = "Markdown"
	sent, err := h.sendAndLog(msg)
	if err != nil {
		log.Printf("Admin menu send error: %v", err)
		return
	}
	h.setAdminMenuMessage(userID, chatID, sent.MessageID)
	h.trackAdminMessage(chatID, sent.MessageID)
}

func (h *BotHandler) getAdminMenuText() string {
	ctx := context.Background()

	// Statistika yig'ish
	products, _ := h.productUseCase.GetAll(ctx)
	productCount := len(products)

	// Foydalanuvchilar soni
	h.lastSeenMu.RLock()
	userCount := len(h.lastSeen)
	h.lastSeenMu.RUnlock()

	// Onlayn foydalanuvchilar (oxirgi 5 daqiqa)
	now := time.Now()
	var onlineCount int
	h.lastSeenMu.RLock()
	for _, ts := range h.lastSeen {
		if now.Sub(ts) <= 5*time.Minute {
			onlineCount++
		}
	}
	h.lastSeenMu.RUnlock()

	// Buyurtmalar (oxirgi 24 soat)
	orders := h.listRecentOrders(1000)
	var todayOrders int
	for _, ord := range orders {
		if !ord.CreatedAt.IsZero() && now.Sub(ord.CreatedAt) <= 24*time.Hour {
			todayOrders++
		}
	}

	return fmt.Sprintf(`🔧 *Admin Dashboard*

📊 *Bugungi statistika:*
━━━━━━━━━━━━━━━━━━━━
📦 Mahsulotlar: %d ta
👥 Jami foydalanuvchi: %d ta
🟢 Onlayn: %d kishi
🛒 Bugungi buyurtmalar: %d ta

📋 *Tezkor komandalar:*
━━━━━━━━━━━━━━━━━━━━
📤 *Katalog:*
• Excel yuklash
• /catalog - Katalog statistikasi
• /products - Mahsulotlar ro'yxati
• /search - Mahsulot qidirish (so'rov keyin yoziladi)
• /add\_product - Mahsulot sonini qo'shish
• /remove\_product - Mahsulot sonini kamaytirish
• /clean - Tozalash

🗄️ *Database (SheetMaster):*
• /import - Import komandalar menyusi
• /import\_now - Darhol katalogni yangilash (XLSX ham yuboradi)
• /import\_auto - Auto import intervalini sozlash
• /import\_auto\_status - Holat / oxirgi urinishlar
• /import\_auto\_off - Auto importni o'chirish
• /database\_select - Import faylini tanlash
• /db\_status - Ulanish holati + oxirgi fayl info
• /db\_sync - /import\_now bilan bir xil

👥 *Foydalanuvchilar:*
• /online - Faollik
• /user - Ro'yxat
• /userchathistory - Chat tarixi
• /about\_user - Foydalanuvchilar eksporti (XLSX)
• /broadcast - Hammaga xabar

🛒 *Buyurtmalar:*
• /orders - So'nggi buyurtmalar
• /top - TOP mahsulotlar

⚙️ *Sozlamalar:*
• /val - Valyuta rejimi
• /sticker - Sticker sozlash
• /stats - Buyurtma statistika
• /not - Eslatmalar

🚪 /logout - Chiqish`, productCount, userCount, onlineCount, todayOrders)
}

func (h *BotHandler) setAdminMenuMessage(userID, chatID int64, msgID int) {
	h.adminMenuMu.Lock()
	h.adminMenuMsgs[userID] = adminMenuMessage{chatID: chatID, messageID: msgID}
	h.adminMenuMu.Unlock()
}

func (h *BotHandler) getAdminMenuMessage(userID int64) (adminMenuMessage, bool) {
	h.adminMenuMu.RLock()
	val, ok := h.adminMenuMsgs[userID]
	h.adminMenuMu.RUnlock()
	return val, ok
}

// Fayl yuklash
func (h *BotHandler) downloadFile(fileID string) ([]byte, error) {
	file, err := h.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, err
	}

	fileURL := file.Link(h.bot.Token)
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (h *BotHandler) handleDocumentMessage(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Fayllarni faqat adminlar yuklashi mumkin. /admin komandasi bilan admin bo'ling.")
		return
	}

	doc := message.Document

	if doc.FileSize > 5*1024*1024 {
		h.sendMessage(message.Chat.ID, "❌ Fayl hajmi 5MB dan oshmasligi kerak!")
		return
	}

	if !strings.HasSuffix(doc.FileName, ".xlsx") && !strings.HasSuffix(doc.FileName, ".xls") {
		h.sendMessage(message.Chat.ID, "❌ Faqat Excel fayllari (.xlsx, .xls) qabul qilinadi!")
		return
	}

	h.sendMessage(message.Chat.ID, "⏳ Fayl yuklanmoqda va qayta ishlanmoqda... Bu bir necha daqiqa vaqt olishi mumkin.")

	fileBytes, err := h.downloadFile(doc.FileID)
	if err != nil {
		log.Printf("File download error: %v", err)
		h.sendMessage(message.Chat.ID, "❌ Faylni yuklashda xatolik yuz berdi.")
		return
	}

	// Run the catalog upload in a separate goroutine to avoid blocking the bot.
	go func() {
		// Create a new context for the goroutine to avoid cancellation.
		goroutineCtx := context.Background()

		count, err := h.adminUseCase.UploadCatalog(goroutineCtx, userID, fileBytes, doc.FileName)
		if err != nil {
			log.Printf("Upload catalog error: %v", err)
			h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Katalog yuklashda xatolik: %v", err))
			return
		}

		h.sendMessage(message.Chat.ID, fmt.Sprintf("✅ Katalog muvaffaqiyatli yangilandi! %d ta mahsulot qo'shildi.", count))
	}()
}

func (h *BotHandler) handleSearchCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	// Komandadan keyin qidiruv so'zini ajratib olish
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		h.setAwaitingSearch(userID, true)
		h.sendMessage(message.Chat.ID, "❓ Nima qidiryapsiz? Mahsulot nomini yozing.\n\nMisol: RTX 3060")
		return
	}

	query := strings.Join(parts[1:], " ")
	h.setAwaitingSearch(userID, false)
	h.sendAdminSearchResults(ctx, message.Chat.ID, query)
}

func (h *BotHandler) handleSearchInput(ctx context.Context, message *tgbotapi.Message) bool {
	userID := message.From.ID
	if !h.isAwaitingSearch(userID) {
		return false
	}

	query := strings.TrimSpace(message.Text)
	if query == "" {
		h.sendMessage(message.Chat.ID, "❓ Nima qidiryapsiz? Mahsulot nomini yozing.")
		return true
	}

	h.setAwaitingSearch(userID, false)
	h.sendAdminSearchResults(ctx, message.Chat.ID, query)
	return true
}

func (h *BotHandler) sendAdminSearchResults(ctx context.Context, chatID int64, query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		h.sendMessage(chatID, "❓ Qidiruv uchun mahsulot nomini kiriting.")
		return
	}

	if all, err := h.productUseCase.GetAll(ctx); err == nil && len(all) == 0 {
		h.sendMessage(chatID, "Katalog yuklanmagan. Admin Excel yuklashi yoki /import ishlatishi kerak.")
		return
	}

	products, err := h.productUseCase.Search(ctx, query)
	if err != nil || len(products) == 0 {
		h.sendMessage(chatID, fmt.Sprintf("🔍 '%s' uchun mahsulot topilmadi.", query))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 *Qidiruv natijasi: '%s'*\n\n", query))
	sb.WriteString(fmt.Sprintf("Topildi: *%d ta mahsulot*\n", len(products)))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")

	for i, p := range products {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n... va yana %d ta mahsulot", len(products)-20))
			break
		}
		sb.WriteString(fmt.Sprintf("\n📦 *%s*\n", p.Name))
		sb.WriteString(fmt.Sprintf("💰 Narx: $%.2f\n", p.Price))
		sb.WriteString(fmt.Sprintf("📁 Kategoriya: %s\n", p.Category))
		if p.Stock > 0 {
			sb.WriteString(fmt.Sprintf("📊 Soni: %d\n", p.Stock))
		}
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	}

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(chatID, sent.MessageID)
	}
}

// handleBroadcastCommand barcha foydalanuvchilarga xabar yuborish
func (h *BotHandler) handleBroadcastCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	// Komandadan keyin xabarni ajratib olish
	parts := strings.SplitN(message.Text, " ", 2)
	if len(parts) < 2 {
		h.sendMessage(message.Chat.ID, `📢 *Broadcast xabar yuborish*

Barcha foydalanuvchilarga xabar yuborish uchun:
/broadcast Sizning xabaringiz

⚠️ *Diqqat:* Bu xabar barcha foydalanuvchilarga yuboriladi!

💡 *Maslahat:*
• Qisqa va aniq yozing
• Emoji ishlating 😊
• Muhim ma'lumotlarni qalin qiling: *qalin matn*`)
		return
	}

	broadcastMsg := strings.TrimSpace(parts[1])
	if broadcastMsg == "" {
		h.sendMessage(message.Chat.ID, "❌ Xabar bo'sh bo'lishi mumkin emas!")
		return
	}

	// Tasdiqlash
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ha, yuborish", fmt.Sprintf("broadcast_confirm:%s", broadcastMsg)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Bekor qilish", "broadcast_cancel"),
		),
	)

	h.lastSeenMu.RLock()
	userCount := len(h.lastSeen)
	h.lastSeenMu.RUnlock()

	confirmText := fmt.Sprintf(`📢 *Broadcast xabar*

*Yuboriladi:*
%s

━━━━━━━━━━━━━━━━━━━━
👥 Qabul qiluvchilar: *%d ta foydalanuvchi*

Yuborishni tasdiqlaysizmi?`, broadcastMsg, userCount)

	msg := tgbotapi.NewMessage(message.Chat.ID, confirmText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(message.Chat.ID, sent.MessageID)
	}
}

// handleBroadcastConfirm broadcast xabarni yuborishni tasdiqlash
func (h *BotHandler) handleBroadcastConfirm(ctx context.Context, chatID, userID int64, broadcastMsg string) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		return
	}

	h.sendMessage(chatID, "📤 Xabar yuborilmoqda...")

	// Barcha foydalanuvchilarga yuborish
	var success, failed int
	h.lastSeenMu.RLock()
	users := make([]int64, 0, len(h.lastSeen))
	for id := range h.lastSeen {
		users = append(users, id)
	}
	h.lastSeenMu.RUnlock()

	for _, targetID := range users {
		msg := tgbotapi.NewMessage(targetID, broadcastMsg)
		msg.ParseMode = "Markdown"
		if _, err := h.sendAndLog(msg); err != nil {
			failed++
			log.Printf("Broadcast failed for user %d: %v", targetID, err)
		} else {
			success++
		}
		time.Sleep(50 * time.Millisecond) // Anti-flood
	}

	resultMsg := fmt.Sprintf(`✅ *Broadcast yakunlandi!*

📊 *Statistika:*
✅ Muvaffaqiyatli: %d ta
❌ Xato: %d ta
📊 Jami: %d ta`, success, failed, success+failed)

	h.sendMessage(chatID, resultMsg)
}

// handleTopCommand eng ko'p sotilgan mahsulotlar
func (h *BotHandler) handleTopCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	// Parse arguments: /top or /top 20
	parts := strings.Fields(message.Text)
	limit := 20
	if len(parts) > 1 {
		if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	// Collect all orders
	orders := h.listRecentOrders(1000)
	if len(orders) == 0 {
		h.sendMessage(message.Chat.ID, "📊 Hali buyurtmalar yo'q.")
		return
	}

	// Extract product mentions from orders
	productMentions := make(map[string]int)
	categoryMentions := make(map[string]int)

	// Common product patterns
	cpuPattern := regexp.MustCompile(`(?i)(Intel|AMD|Ryzen|Core)\s+[^\s,\n]+`)
	gpuPattern := regexp.MustCompile(`(?i)(RTX|GTX|RX|Arc)\s+\d+`)
	ramPattern := regexp.MustCompile(`(?i)(\d+GB)\s+(DDR\d)`)
	ssdPattern := regexp.MustCompile(`(?i)(\d+(?:GB|TB))\s+(?:SSD|NVMe|M\.2)`)

	for _, ord := range orders {
		text := ord.Summary + " " + ord.StatusSummary

		// Find CPUs
		if matches := cpuPattern.FindAllString(text, -1); len(matches) > 0 {
			for _, match := range matches {
				productMentions[strings.TrimSpace(match)]++
			}
			categoryMentions["CPU"]++
		}

		// Find GPUs
		if matches := gpuPattern.FindAllString(text, -1); len(matches) > 0 {
			for _, match := range matches {
				productMentions[strings.TrimSpace(match)]++
			}
			categoryMentions["GPU"]++
		}

		// Find RAM
		if matches := ramPattern.FindAllString(text, -1); len(matches) > 0 {
			for _, match := range matches {
				productMentions[strings.TrimSpace(match)]++
			}
			categoryMentions["RAM"]++
		}

		// Find SSD
		if matches := ssdPattern.FindAllString(text, -1); len(matches) > 0 {
			for _, match := range matches {
				productMentions[strings.TrimSpace(match)]++
			}
			categoryMentions["SSD"]++
		}
	}

	// Sort products by mentions
	type productStat struct {
		name  string
		count int
	}
	var stats []productStat
	for name, count := range productMentions {
		stats = append(stats, productStat{name, count})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].count > stats[j].count
	})

	// Build response
	var sb strings.Builder
	sb.WriteString("📊 *TOP Mahsulotlar Statistikasi*\n\n")
	sb.WriteString(fmt.Sprintf("📈 Tahlil qilingan buyurtmalar: %d ta\n", len(orders)))
	sb.WriteString(fmt.Sprintf("🔍 Topilgan mahsulotlar: %d xil\n\n", len(productMentions)))

	// Category breakdown
	if len(categoryMentions) > 0 {
		sb.WriteString("📦 *Kategoriyalar bo'yicha:*\n")
		if count, ok := categoryMentions["CPU"]; ok {
			sb.WriteString(fmt.Sprintf("   🖥 CPU: %d ta\n", count))
		}
		if count, ok := categoryMentions["GPU"]; ok {
			sb.WriteString(fmt.Sprintf("   🎮 GPU: %d ta\n", count))
		}
		if count, ok := categoryMentions["RAM"]; ok {
			sb.WriteString(fmt.Sprintf("   💾 RAM: %d ta\n", count))
		}
		if count, ok := categoryMentions["SSD"]; ok {
			sb.WriteString(fmt.Sprintf("   💿 SSD: %d ta\n", count))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("🏆 *TOP %d Eng Mashhur Mahsulotlar:*\n", limit))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━\n")

	if len(stats) == 0 {
		sb.WriteString("Hali statistika yo'q.\n")
	} else {
		displayLimit := limit
		if len(stats) < displayLimit {
			displayLimit = len(stats)
		}

		for i := 0; i < displayLimit; i++ {
			medal := ""
			switch i {
			case 0:
				medal = "🥇"
			case 1:
				medal = "🥈"
			case 2:
				medal = "🥉"
			default:
				medal = fmt.Sprintf("%d.", i+1)
			}

			percentage := float64(stats[i].count) * 100 / float64(len(orders))
			sb.WriteString(fmt.Sprintf("%s *%s*\n   └ %d marta (%.1f%%)\n",
				medal, stats[i].name, stats[i].count, percentage))
		}
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("💡 _Bu statistika oxirgi buyurtmalar asosida tuzilgan._")

	h.sendMessageMarkdown(message.Chat.ID, sb.String())
}

// handleAdminCurrencyInput admin USD->SUM kursini kiritayotganda ishlaydi
func (h *BotHandler) handleAdminCurrencyInput(ctx context.Context, message *tgbotapi.Message) bool {
	userID := message.From.ID
	if !h.isAwaitingCurrencyRate(userID) {
		return false
	}
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.setAwaitingCurrencyRate(userID, false)
		return false
	}

	txt := strings.TrimSpace(message.Text)
	if txt == "" {
		h.sendMessage(message.Chat.ID, "Iltimos, kursni kiriting. Masalan: 12500")
		return true
	}
	txt = strings.ReplaceAll(txt, " ", "")
	txt = strings.ReplaceAll(txt, ",", ".")
	val, err := strconv.ParseFloat(txt, 64)
	if err != nil || val <= 0 {
		h.sendMessage(message.Chat.ID, "❌ Noto'g'ri format. Masalan: 12500 yoki 12500.5")
		return true
	}

	h.setAwaitingCurrencyRate(userID, false)
	h.setCurrencyMode("sum", val)
	h.sendMessage(message.Chat.ID, fmt.Sprintf("✅ Kurs saqlandi: 1$ = %.2f so'm. AI javoblari so'mga o'giriladi.", val))
	return true
}

// ==================== SheetMaster (Database) Commands ====================

// handleDBStatusCommand SheetMaster (database) API holatini ko'rsatadi
func (h *BotHandler) handleDBStatusCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	// Primary: direct DB access (no API key needed)
	if dsn, dsnErr := sheetMasterDBDSNFromEnv(); dsnErr == nil {
		var f sheetMasterDBFile
		var dbErr error
		var selectedNote string
		if sel, ok, _ := loadSheetMasterDBSelection(); ok && sel.FileID != 0 {
			f, dbErr = sheetMasterGetFileFromDB(ctx, dsn, sel.FileID)
			if dbErr == nil {
				selectedNote = " (tanlangan)"
			}
		} else {
			f, dbErr = sheetMasterGetLatestFileFromDB(ctx, dsn)
		}
		if dbErr == nil {
			usedRange := "—"
			if ur, ok, err := sheetMasterDBFileUsedRangeA1(f); err == nil && ok && strings.TrimSpace(ur) != "" {
				usedRange = ur
			}

			name := strings.TrimSpace(f.Name)
			if name == "" {
				name = fmt.Sprintf("file_id=%d", f.ID)
			}
			h.sendMessage(message.Chat.ID, fmt.Sprintf("🗄️ Database (SheetMaster)\n\n✅ DB: connected\n📄 Fayl: %s (id=%d)%s\n📐 Used range: %s", name, f.ID, selectedNote, usedRange))
			return
		}
	}

	// Fallback: SheetMaster REST API (requires API key)
	cfg, err := h.resolveSheetMasterConfig()
	if err != nil {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Database ulanmagan va API ham sozlanmagan.\n\nXatolik: %v", err))
		return
	}

	health, err := sheetMasterHealth(ctx, cfg)
	if err != nil {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ SheetMaster health xatosi: %v", err))
		return
	}

	schema, err := sheetMasterGetSchema(ctx, cfg)
	if err != nil {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ SheetMaster schema xatosi: %v", err))
		return
	}

	usedRange := "—"
	if schema.UsedRange != nil && strings.TrimSpace(schema.UsedRange.A1) != "" {
		usedRange = schema.UsedRange.A1
	}

	name := strings.TrimSpace(schema.Name)
	if name == "" {
		name = fmt.Sprintf("file_id=%s", cfg.FileID)
	}

	h.sendMessage(message.Chat.ID, fmt.Sprintf("🗄️ Database (SheetMaster)\n\n✅ Health: %s\n📄 Fayl: %s\n📐 Used range: %s", health, name, usedRange))
}

func (h *BotHandler) handleDatabaseSelectCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	parts := strings.Fields(message.Text)
	if len(parts) > 1 {
		arg := strings.TrimSpace(parts[1])
		arg = strings.TrimPrefix(strings.ToLower(arg), "file_id=")
		arg = strings.Trim(arg, "`\"'")
		if arg == "latest" || arg == "clear" || arg == "reset" || arg == "0" {
			if err := clearSheetMasterDBSelection(); err != nil {
				h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Tanlovni tozalashda xatolik: %v", err))
				return
			}
			h.sendMessage(message.Chat.ID, "✅ Tanlov tozalandi. Endi /import_now oxirgi faylni oladi.")
			return
		}
		id, err := strconv.ParseUint(arg, 10, 64)
		if err != nil || id == 0 {
			h.sendMessage(message.Chat.ID, "❌ Fayl ID noto'g'ri. Misol: /database_select 12")
			return
		}
		h.applyDatabaseSelection(ctx, message.Chat.ID, uint(id))
		return
	}

	files, err := sheetMasterListFilesFromDB(ctx, 20)
	if err != nil {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Database fayllarini olishda xatolik: %v", err))
		return
	}

	var header strings.Builder
	if sel, ok, _ := loadSheetMasterDBSelection(); ok && sel.FileID != 0 {
		name := strings.TrimSpace(sel.Name)
		if name == "" {
			name = fmt.Sprintf("file_id=%d", sel.FileID)
		}
		header.WriteString(fmt.Sprintf("Hozirgi tanlangan fayl: %s (id=%d)\n\n", name, sel.FileID))
	} else {
		header.WriteString("Hozircha fayl tanlanmagan. Oxirgi fayl ishlatiladi.\n\n")
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, header.String()+formatSheetMasterDBFilesList(files))
	msg.ReplyMarkup = buildSheetMasterDBFilesKeyboard(files)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(message.Chat.ID, sent.MessageID)
	}
}

func (h *BotHandler) applyDatabaseSelection(ctx context.Context, chatID int64, fileID uint) {
	dsn, err := sheetMasterDBDSNFromEnv()
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("❌ Database DSN topilmadi: %v", err))
		return
	}
	file, err := sheetMasterGetFileFromDB(ctx, dsn, fileID)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("❌ Fayl topilmadi: %v", err))
		return
	}
	sel := sheetMasterDBSelection{
		FileID:    file.ID,
		Name:      file.Name,
		UpdatedAt: file.UpdatedAt,
	}
	if err := saveSheetMasterDBSelection(sel); err != nil {
		h.sendMessage(chatID, fmt.Sprintf("❌ Tanlovni saqlashda xatolik: %v", err))
		return
	}
	name := strings.TrimSpace(file.Name)
	if name == "" {
		name = fmt.Sprintf("file_id=%d", file.ID)
	}
	h.sendMessage(chatID, fmt.Sprintf("✅ Tanlandi: %s (id=%d). Endi /import_now shu fayldan import qiladi.", name, file.ID))
}

func formatSheetMasterDBFilesList(files []sheetMasterDBFileMeta) string {
	if len(files) == 0 {
		return "📂 Database fayllari topilmadi."
	}
	var sb strings.Builder
	sb.WriteString("📂 Database fayllari (oxirgi 20 ta):\n\n")
	for _, f := range files {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = "(no name)"
		}
		sb.WriteString(fmt.Sprintf("• %d — %s\n", f.ID, name))
	}
	sb.WriteString("\nFaylni tanlash uchun tugmani bosing yoki /database_select <id> yuboring.\nTanlovni bekor qilish: /database_select latest")
	return sb.String()
}

func buildSheetMasterDBFilesKeyboard(files []sheetMasterDBFileMeta) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(files)+1)
	for _, f := range files {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = "(no name)"
		}
		label := fmt.Sprintf("%d — %s", f.ID, truncateInlineLabel(name, 36))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("db_select|%d", f.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Oxirgi faylni ishlatish", "db_select|latest"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func truncateInlineLabel(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}

// handleDBSyncCommand SheetMaster (database) dan katalogni yuklab, bot katalogini yangilaydi
func (h *BotHandler) handleDBSyncCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.sendMessage(message.Chat.ID, "🔄 Database sync boshlandi...")

	selectedID, hasSelected := sheetMasterSelectedFileID()

	if cfg, cfgErr := h.resolveSheetMasterConfig(); cfgErr == nil {
		if hasSelected {
			cfg.FileID = strconv.FormatUint(uint64(selectedID), 10)
		}
		if exp, err := sheetMasterExportFromAPI(ctx, cfg); err == nil {
			count, upErr := h.adminUseCase.UploadCatalog(ctx, userID, exp.XLSXBytes, exp.Filename)
			if upErr != nil {
				h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Katalogni yangilashda xatolik: %v", upErr))
				return
			}

			const maxSend = 40 * 1024 * 1024
			if len(exp.XLSXBytes) <= maxSend {
				doc := tgbotapi.NewDocument(message.Chat.ID, tgbotapi.FileBytes{Name: exp.Filename, Bytes: exp.XLSXBytes})
				doc.Caption = fmt.Sprintf("📎 Database'dan olingan Excel (XLSX)\n\nfile_id=%d\nused_range=%s", exp.File.ID, exp.UsedRangeA1)
				if sent, sendErr := h.sendAndLog(doc); sendErr != nil {
					log.Printf("[db_sync] send xlsx error: %v", sendErr)
				} else {
					h.trackAdminMessage(message.Chat.ID, sent.MessageID)
				}
			} else {
				h.sendMessage(message.Chat.ID, fmt.Sprintf("⚠️ Excel fayl juda katta (%d MB), shuning uchun yuborilmadi.", len(exp.XLSXBytes)/(1024*1024)))
			}

			note := ""
			if hasSelected {
				note = " (tanlangan fayl)"
			}
			h.sendMessage(message.Chat.ID, fmt.Sprintf("✅ Database dan katalog yuklandi!%s\n\n📦 Yuklangan mahsulotlar: %d ta", note, count))
			return
		} else {
			log.Printf("[db_sync] api import error: %v", err)
		}
	}

	if exp, err := sheetMasterExportFromDB(ctx, selectedID); err == nil {
		count, upErr := h.adminUseCase.UploadCatalog(ctx, userID, exp.XLSXBytes, exp.Filename)
		if upErr != nil {
			h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Katalogni yangilashda xatolik: %v", upErr))
			return
		}

		// Adminga Excel faylni ham yuboramiz (backup/tekshiruv uchun)
		const maxSend = 40 * 1024 * 1024 // 40MB
		if len(exp.XLSXBytes) <= maxSend {
			doc := tgbotapi.NewDocument(message.Chat.ID, tgbotapi.FileBytes{Name: exp.Filename, Bytes: exp.XLSXBytes})
			doc.Caption = fmt.Sprintf("📎 Database'dan olingan Excel (XLSX)\n\nfile_id=%d\nused_range=%s", exp.File.ID, exp.UsedRangeA1)
			if sent, sendErr := h.sendAndLog(doc); sendErr != nil {
				log.Printf("[db_sync] send xlsx error: %v", sendErr)
			} else {
				h.trackAdminMessage(message.Chat.ID, sent.MessageID)
			}
		} else {
			h.sendMessage(message.Chat.ID, fmt.Sprintf("⚠️ Excel fayl juda katta (%d MB), shuning uchun yuborilmadi.", len(exp.XLSXBytes)/(1024*1024)))
		}

		note := ""
		if hasSelected {
			note = " (tanlangan fayl)"
		}
		h.sendMessage(message.Chat.ID, fmt.Sprintf("✅ Database dan katalog yuklandi!%s\n\n📦 Yuklangan mahsulotlar: %d ta", note, count))
		return
	} else {
		log.Printf("[db_sync] db import error: %v", err)
	}

	if _, cfgErr := h.resolveSheetMasterConfig(); cfgErr != nil {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Database'dan import bo'lmadi va API ham sozlanmagan.\n\nXatolik: %v", cfgErr))
		return
	}
	h.sendMessage(message.Chat.ID, "❌ Database'dan ham import bo'lmadi.")
}
