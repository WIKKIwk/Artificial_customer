package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Order session helpers
func (h *BotHandler) startOrderSession(userID int64, info pendingApproval) {
	// Buyurtma jarayoni boshlansa, avvalgi eslatma taymerini bekor qilamiz
	h.cancelConfigReminder(userID)

	session := &orderSession{
		Stage:     orderStageNeedName,
		Summary:   info.Summary,
		ConfigTxt: info.Config,
		Username:  info.Username,
		ChatID:    info.UserChat,
		MessageID: 0,
	}
	// Profil bo'lsa, avvaldan to'ldiramiz
	if prof, ok := h.getProfile(userID); ok {
		if strings.TrimSpace(prof.Name) != "" {
			session.Name = prof.Name
			session.Stage = orderStageNeedPhone
		}
		if strings.TrimSpace(prof.Phone) != "" {
			session.Phone = prof.Phone
			session.Stage = orderStageNeedLocation
		}
	}
	h.orderMu.Lock()
	h.orderSessions[userID] = session
	h.orderMu.Unlock()

	configText := strings.TrimSpace(info.Config)
	if configText == "" {
		return
	}
	items := extractConfigItemNames(configText)
	if len(items) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	updated, reservedItems, err := h.adjustInventoryItems(ctx, items, -1)
	if err != nil {
		log.Printf("[inventory] config reserve failed user=%d err=%v", userID, err)
		return
	}
	if updated == 0 || len(reservedItems) == 0 {
		return
	}

	h.orderMu.Lock()
	if sess, ok := h.orderSessions[userID]; ok && sess != nil {
		sess.InventoryReserved = true
		sess.ReservedItems = reservedItems
		h.orderSessions[userID] = sess
	}
	h.orderMu.Unlock()
	log.Printf("[inventory] config reserved user=%d items=%d", userID, len(reservedItems))
}

func (h *BotHandler) clearOrderSession(userID int64) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	if ok {
		h.stashOrderCleanupLocked(userID, session)
		delete(h.orderSessions, userID)
	}
	h.orderMu.Unlock()

	if ok && session != nil && session.ChatID != 0 {
		// Reply keyboardni yashiramiz (telefon/location bosqichi tugagach)
		h.hideReplyKeyboard(session.ChatID)
	}
}

func (h *BotHandler) stashOrderCleanupLocked(userID int64, session *orderSession) {
	if session == nil {
		return
	}
	var ids []int
	if session.MessageID != 0 {
		ids = append(ids, session.MessageID)
	}
	for _, id := range session.FormMessageIDs {
		if id != 0 {
			ids = append(ids, id)
		}
	}
	ids = uniqueOrderMessageIDs(ids)
	if len(ids) == 0 {
		return
	}
	info := h.orderCleanup[userID]
	if info.ChatID == 0 && session.ChatID != 0 {
		info.ChatID = session.ChatID
	}
	info.MessageIDs = append(info.MessageIDs, ids...)
	info.MessageIDs = uniqueOrderMessageIDs(info.MessageIDs)
	h.orderCleanup[userID] = info
}

func (h *BotHandler) releaseReservedInventory(userID int64) {
	var items []string
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	if ok && session != nil && session.InventoryReserved && len(session.ReservedItems) > 0 {
		items = append(items, session.ReservedItems...)
		session.InventoryReserved = false
		session.ReservedItems = nil
		h.orderSessions[userID] = session
	}
	h.orderMu.Unlock()

	if len(items) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if updated, _, err := h.adjustInventoryItems(ctx, items, 1); err != nil {
		log.Printf("[inventory] config release failed user=%d err=%v", userID, err)
	} else if updated > 0 {
		log.Printf("[inventory] config release user=%d items=%d", userID, updated)
	}
}

func (h *BotHandler) hideReplyKeyboard(chatID int64) {
	if chatID == 0 || h.bot == nil {
		return
	}
	removeKb := tgbotapi.NewRemoveKeyboard(true)
	msg := tgbotapi.NewMessage(chatID, "✅")
	msg.ReplyMarkup = removeKb
	if sent, err := h.sendAndLog(msg); err != nil {
		log.Printf("remove keyboard failed chat=%d err=%v", chatID, err)
	} else {
		h.deleteMessage(chatID, sent.MessageID)
	}
}

func (h *BotHandler) showReplyKeyboard(chatID int64, kb tgbotapi.ReplyKeyboardMarkup) {
	if chatID == 0 || h.bot == nil {
		return
	}
	msg := tgbotapi.NewMessage(chatID, "✅")
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err != nil {
		log.Printf("show keyboard failed chat=%d err=%v", chatID, err)
		return
	} else {
		h.deleteMessage(chatID, sent.MessageID)
	}
}

func (h *BotHandler) hasOrderSession(userID int64) bool {
	h.orderMu.RLock()
	defer h.orderMu.RUnlock()
	_, ok := h.orderSessions[userID]
	return ok
}

// handleOrderCommand foydalanuvchi buyurtmalari statusini ko'rsatadi
func (h *BotHandler) handleOrderCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := h.getUserLang(userID)
	orders := h.listOrdersByUser(userID)
	if len(orders) == 0 {
		h.sendMessage(message.Chat.ID, t(lang, "Sizda hali buyurtmalar yo'q.", "У вас пока нет заказов."))
		return
	}
	// Har bir buyurtma nomini tugma ko'rinishida chiqaramiz
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, ord := range orders {
		title := orderTitle(ord)
		if len(title) > 50 {
			title = title[:50] + "…"
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🧾 %s", title), "order_view|"+ord.OrderID)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	// Barchasini tozalash tugmasi
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "🗑️ Barchasini tozalash", "🗑️ Очистить все"), "order_clear_all"),
	))
	msg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "🧾 Buyurtmalaringiz:", "🧾 Ваши заказы:"))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, _ = h.sendAndLog(msg)
}

// Order flow
func (h *BotHandler) handleOrderFlow(ctx context.Context, userID int64, username, text string, chatID int64, msg *tgbotapi.Message) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	h.orderMu.Unlock()
	if !ok {
		return
	}

	// Orqaga tugmasi (matn) bosilganda
	if isBackCommand(text) {
		h.handleOrderBack(userID, chatID, msg)
		return
	}

	// Agar foydalanuvchi 👍 yoki sotib olish niyatini bildirsa, bosqichni eslatamiz va davom ettiramiz
	if containsThumbsUp(text) || isPurchaseIntent(text) {
		switch session.Stage {
		case orderStageNeedName:
			h.sendMessage(chatID, "👍 Qabul qilindi! Buyurtmani rasmiylashtirish uchun to'liq ismingizni yozing.")
		case orderStageNeedPhone:
			h.sendMessage(chatID, "👍 Qabul qilindi! Iltimos, telefon raqamingizni yuboring.")
			h.sendPhoneRequest(chatID)
		case orderStageNeedLocation:
			h.sendMessage(chatID, "👍 Qabul qilindi! Yetkazib berish manzilini yuboring.")
			h.sendLocationRequest(chatID)
		case orderStageNeedDeliveryChoice:
			h.sendMessage(chatID, "👍 Qabul qilindi! Yetkazib berish usulini tanlang.")
			h.sendDeliveryChoice(chatID)
		case orderStageNeedDeliveryConfirm:
			h.sendMessage(chatID, "👍 Qabul qilindi! Dostavka narxiga rozimisiz?")
			h.sendDeliveryConfirm(chatID)
		}
		return
	}

	switch session.Stage {
	case orderStageNeedName:
		session.Name = strings.TrimSpace(text)
		if session.Name == "" {
			lang := h.getUserLang(userID)
			h.sendOrderForm(userID, t(lang, "Iltimos, to'liq ismingizni yozing.", "Пожалуйста, укажите полное имя."), nil)
			return
		}
		// ✅ Ism validatsiyasi qo'shildi
		if !validateName(session.Name) {
			lang := h.getUserLang(userID)
			h.sendOrderForm(userID, t(lang, "Noto'g'ri ism formati. Iltimos, ismingizni faqat harflar bilan kiriting (kamida 2 ta harf).", "Неверный формат имени. Укажите имя только буквами (минимум 2 буквы)."), nil)
			return
		}
		h.setProfile(userID, userProfile{Name: session.Name})
		oldMsg := session.MessageID
		session.Stage = orderStageNeedPhone
		session.MessageID = 0
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		if oldMsg != 0 {
			h.deleteMessage(chatID, oldMsg)
		}
		h.sendOrderForm(userID, "📞 Telefon raqamingizni yuboring.", nil)
		h.deleteUserMessage(chatID, msg)
		return
	case orderStageNeedPhone:
		phone := ""
		if msg != nil && msg.Contact != nil && msg.Contact.PhoneNumber != "" {
			phone = msg.Contact.PhoneNumber
		} else {
			phone = strings.TrimSpace(text)
		}
		if phone == "" {
			h.sendOrderForm(userID, "📞 Telefon raqamingizni yuboring.", nil)
			return
		}
		// ✅ Telefon validatsiyasi qo'shildi
		if !validatePhoneNumber(phone) {
			h.sendOrderForm(userID, t(h.getUserLang(userID), "Noto'g'ri telefon raqami! Kamida 7 ta raqam bo'lishi kerak. Masalan: +998901234567", "Неверный номер телефона! Минимум 7 цифр. Например: +998901234567"), nil)
			return
		}
		oldMsg := session.MessageID
		session.Phone = phone
		h.setProfile(userID, userProfile{Phone: session.Phone})
		session.Stage = orderStageNeedLocation
		session.MessageID = 0
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		if oldMsg != 0 {
			h.deleteMessage(chatID, oldMsg)
		}
		h.sendOrderForm(userID, "📍 Lokatsiyani yuboring yoki manzilni yozing.", nil)
		h.deleteUserMessage(chatID, msg)
		return
	case orderStageNeedLocation:
		locText := strings.TrimSpace(text)
		if msg != nil && msg.Location != nil {
			loc := msg.Location
			locText = fmt.Sprintf("https://www.google.com/maps?q=%.5f,%.5f", loc.Latitude, loc.Longitude)
		}
		if locText == "" {
			h.sendOrderForm(userID, "📍 Lokatsiyani yuboring yoki manzilni yozing.", nil)
			return
		}
		session.Location = locText
		session.Stage = orderStageNeedDeliveryChoice
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		h.hideReplyKeyboard(chatID)
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(t(h.getUserLang(userID), "🏬 Olib ketaman", "🏬 Самовывоз"), "delivery_pickup"),
				tgbotapi.NewInlineKeyboardButtonData(t(h.getUserLang(userID), "🚚 Dostavka", "🚚 Доставка"), "delivery_courier"),
			),
		)
		h.sendOrderForm(userID, t(h.getUserLang(userID), "Mahsulotni qanday olasiz?", "Как хотите получить заказ?"), &kb)
		h.deleteUserMessage(chatID, msg)
		return
	case orderStageNeedDeliveryChoice:
		// Kutamiz (callbacklar bilan)
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		return
	case orderStageNeedDeliveryConfirm:
		// Kutamiz (callbacklar bilan)
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		return
	}
}

// Order jarayonida "Orqaga" tugmasi/callbacklarini qayta ishlash
func (h *BotHandler) handleOrderBack(userID, chatID int64, msg *tgbotapi.Message) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	if !ok {
		h.orderMu.Unlock()
		return
	}
	oldMsg := session.MessageID
	switch session.Stage {
	case orderStageNeedName:
		h.orderMu.Unlock()
		return
	case orderStageNeedPhone:
		session.Stage = orderStageNeedName
	case orderStageNeedLocation:
		session.Stage = orderStageNeedPhone
	case orderStageNeedDeliveryChoice:
		session.Stage = orderStageNeedLocation
		session.Delivery = ""
	case orderStageNeedDeliveryConfirm:
		session.Stage = orderStageNeedDeliveryChoice
		session.Delivery = ""
	}
	// Force new form send
	session.MessageID = 0
	h.orderSessions[userID] = session
	h.orderMu.Unlock()

	lang := h.getUserLang(userID)
	switch session.Stage {
	case orderStageNeedName:
		h.sendOrderForm(userID, t(lang, "📝 Iltimos, to'liq ismingizni yozing.", "📝 Пожалуйста, укажите полное имя."), nil)
	case orderStageNeedPhone:
		h.sendOrderForm(userID, "📞 Telefon raqamingizni yuboring.", nil)
	case orderStageNeedLocation:
		h.sendOrderForm(userID, "📍 Lokatsiyani yuboring yoki manzilni yozing.", nil)
	case orderStageNeedDeliveryChoice:
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(t(lang, "🏬 Olib ketaman", "🏬 Самовывоз"), "delivery_pickup"),
				tgbotapi.NewInlineKeyboardButtonData(t(lang, "🚚 Dostavka", "🚚 Доставка"), "delivery_courier"),
			),
		)
		h.sendOrderForm(userID, t(lang, "Mahsulotni qanday olasiz?", "Как хотите получить заказ?"), &kb)
	}

	if msg != nil && oldMsg != 0 {
		h.deleteMessage(chatID, oldMsg)
	}
}

// /order uchun detallarini ko'rsatish (callback orqali)
func (h *BotHandler) handleOrderViewCallback(chatID, userID int64, orderID string) {
	lang := h.getUserLang(userID)
	info, ok := h.getOrderStatus(orderID)
	if !ok {
		h.sendMessage(chatID, t(lang, "❌ Buyurtma topilmadi.", "❌ Заказ не найден."))
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🧾 OrderID: %s\n", orderID))
	sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "📌 Holat", "📌 Статус"), statusLabel(info.Status, lang)))
	if info.Total != "" {
		sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "💰 Jami", "💰 Итого"), info.Total))
	}
	if info.Delivery != "" {
		sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "🚚 Yetkazish", "🚚 Доставка"), deliveryDisplay(info.Delivery, lang)))
	}
	if strings.TrimSpace(info.Summary) != "" {
		sb.WriteString(fmt.Sprintf("\n%s:\n%s", t(lang, "📝 Tafsilotlar", "📝 Детали"), info.Summary))
	} else if strings.TrimSpace(info.StatusSummary) != "" {
		sb.WriteString(fmt.Sprintf("\n%s:\n%s", t(lang, "📝 Tafsilotlar", "📝 Детали"), info.StatusSummary))
	}
	h.sendMessage(chatID, sb.String())
}

// /order -> barchasini tozalash
func (h *BotHandler) handleOrderClearAll(chatID, userID int64, msg *tgbotapi.Message) {
	lang := h.getUserLang(userID)
	if err := h.clearOrdersForUser(userID); err != nil {
		log.Printf("order clear all error user=%d err=%v", userID, err)
		h.sendMessage(chatID, t(lang, "❌ Buyurtmalarni tozalab bo'lmadi.", "❌ Не удалось очистить заказы."))
		return
	}
	text := t(lang, "🗑️ Buyurtmalar tozalandi.", "🗑️ Заказы очищены.")
	if msg != nil {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
		if _, err := h.bot.Send(edit); err != nil {
			h.sendMessage(chatID, text)
		}
	} else {
		h.sendMessage(chatID, text)
	}
}

// Delivery bosqichlari callbacklari
func (h *BotHandler) handleDeliveryChoice(ctx context.Context, userID int64, choice string, chatID int64) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	if ok {
		if choice == "pickup" {
			session.Delivery = "pickup"
			h.orderSessions[userID] = session
		} else if choice == "courier" {
			session.Delivery = "courier"
			session.Stage = orderStageNeedDeliveryConfirm
			h.orderSessions[userID] = session
		}
	}
	h.orderMu.Unlock()

	if !ok {
		h.sendMessage(chatID, "Buyurtma ma'lumotlari topilmadi. /configuratsiya ni qayta bosing.")
		return
	}

	if choice == "pickup" {
		lang := h.getUserLang(userID)
		h.sendOrderForm(userID, t(lang, "✅ Buyurtmangiz qabul qilindi. Tayyor bo'lganda admin sizga bog'lanadi.", "✅ Заказ принят. Как будет готов, с вами свяжется админ."), nil)
		h.sendOrderToGroup2(userID, session, "Olib ketish", "")
		h.clearOrderSession(userID)
	} else {
		lang := h.getUserLang(userID)
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(t(lang, "Ha ✅", "Да ✅"), "delivery_confirm_yes"),
				tgbotapi.NewInlineKeyboardButtonData(t(lang, "Yo'q ❌", "Нет ❌"), "delivery_confirm_no"),
			),
		)
		h.sendOrderForm(userID, t(lang, "🚚 Yetkazib berish Yandex Go orqali amalga oshiriladi. Operator sizga aniq narxni aytadi. Rozimisiz?", "🚚 Доставка будет осуществлена через Yandex Go. Оператор сообщит вам точную стоимость. Согласны?"), &kb)
	}
}

func (h *BotHandler) handleDeliveryConfirm(ctx context.Context, userID int64, agree bool, chatID int64) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	h.orderMu.Unlock()
	if !ok {
		h.sendMessage(chatID, "Buyurtma ma'lumotlari topilmadi. /configuratsiya ni qayta bosing.")
		return
	}

	if !agree {
		h.sendOrderForm(userID, "Unda buyurtmani olib ketish punktidan olib keting.", nil)
		session.Delivery = "pickup"
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		h.sendOrderToGroup2(userID, session, "Olib ketish", "Dostavka narxiga rozilik bermadi")
		h.clearOrderSession(userID)
		return
	}

	// Ha bo'lsa
	lang := h.getUserLang(userID)
	h.sendOrderForm(userID, t(lang, "✅ Buyurtmangiz qabul qilindi. Tayyor bo'lganda admin sizga bog'lanadi.", "✅ Заказ принят. Как будет готов, с вами свяжется админ."), nil)

	if h.activeOrdersChatID != 0 {
		h.sendOrderToGroup2(userID, session, "Dostavka (Yandex Go)", "Rozilik berildi")
	}
	h.clearOrderSession(userID)
}

// UI helpers for order flow
func (h *BotHandler) sendPhoneRequest(chatID int64) {
	lang := h.getUserLang(chatID)
	kb := h.phoneRequestKeyboard(chatID)
	msg := tgbotapi.NewMessage(chatID, t(lang, "📞 Telefon raqamingizni yuboring (yoki button bosib jo'nating).", "📞 Отправьте номер телефона (или нажмите кнопку)."))
	msg.ReplyMarkup = kb
	_, _ = h.sendAndLog(msg)
}

// sendPhonePrompt - telefon so'rovi va msgID qaytaradi (profil oqimi uchun)
func (h *BotHandler) sendPhonePrompt(chatID int64, lang string) int {
	kb := h.phoneRequestKeyboard(chatID)
	msg := tgbotapi.NewMessage(chatID, t(lang, "📞 Telefon raqamingizni yuboring (yoki button bosib jo'nating).", "📞 Отправьте номер телефона (или нажмите кнопку)."))
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err == nil {
		return sent.MessageID
	}
	return 0
}

func (h *BotHandler) phoneRequestKeyboard(chatID int64) tgbotapi.ReplyKeyboardMarkup {
	lang := h.getUserLang(chatID)
	btn := tgbotapi.NewKeyboardButtonContact(t(lang, "📞 Telefon raqamni jo'natish", "📞 Отправить номер телефона"))
	back := tgbotapi.NewKeyboardButton(t(lang, "⬅️ Orqaga", "⬅️ Назад"))
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(btn),
		tgbotapi.NewKeyboardButtonRow(back),
	)
	kb.OneTimeKeyboard = true
	kb.ResizeKeyboard = true
	return kb
}

func (h *BotHandler) sendLocationRequest(chatID int64) {
	lang := h.getUserLang(chatID)
	kb := h.locationRequestKeyboard(chatID)
	msg := tgbotapi.NewMessage(chatID, t(lang, "📍 Lokatsiyangizni yuboring yoki manzilni matn ko'rinishida yozing.", "📍 Отправьте геолокацию или адрес текстом."))
	msg.ReplyMarkup = kb
	_, _ = h.sendAndLog(msg)
}

func (h *BotHandler) locationRequestKeyboard(chatID int64) tgbotapi.ReplyKeyboardMarkup {
	lang := h.getUserLang(chatID)
	locBtn := tgbotapi.NewKeyboardButtonLocation(t(lang, "📍 Lokatsiyani yuborish", "📍 Отправить локацию"))
	back := tgbotapi.NewKeyboardButton(t(lang, "⬅️ Orqaga", "⬅️ Назад"))
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(locBtn),
		tgbotapi.NewKeyboardButtonRow(back),
	)
	kb.OneTimeKeyboard = true
	kb.ResizeKeyboard = true
	return kb
}

func (h *BotHandler) sendDeliveryChoice(chatID int64) {
	lang := h.getUserLang(chatID)
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🏬 Olib ketaman", "🏬 Самовывоз"), "delivery_pickup"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🚚 Dostavka", "🚚 Доставка"), "delivery_courier"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, t(lang, "Mahsulotni qanday olasiz?", "Как хотите получить заказ?"))
	msg.ReplyMarkup = markup
	_, _ = h.sendAndLog(msg)
}

func (h *BotHandler) sendDeliveryConfirm(chatID int64) {
	lang := h.getUserLang(chatID)
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Ha ✅", "Да ✅"), "delivery_confirm_yes"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Yo'q ❌", "Нет ❌"), "delivery_confirm_no"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, t(lang, "🚚 Yetkazib berish Yandex Go orqali amalga oshiriladi. Operator sizga aniq narxni aytadi. Rozimisiz?", "🚚 Доставка будет осуществлена через Yandex Go. Оператор сообщит вам точную стоимость. Согласны?"))
	msg.ReplyMarkup = markup
	_, _ = h.sendAndLog(msg)
}

// Order form rendering/edit helper
func (h *BotHandler) sendOrderForm(userID int64, prompt string, kb *tgbotapi.InlineKeyboardMarkup) {
	h.orderMu.RLock()
	sess, ok := h.orderSessions[userID]
	h.orderMu.RUnlock()
	if !ok {
		return
	}
	lang := h.getUserLang(userID)
	if strings.TrimSpace(prompt) == "" {
		switch sess.Stage {
		case orderStageNeedName:
			prompt = t(lang, "📝 Iltimos, to'liq ismingizni yozing.", "📝 Пожалуйста, укажите полное имя.")
		case orderStageNeedPhone:
			prompt = t(lang, "📞 Telefon raqamingizni yuboring.", "📞 Отправьте номер телефона.")
		case orderStageNeedLocation:
			prompt = t(lang, "📍 Lokatsiyani yuboring yoki manzilni yozing.", "📍 Отправьте локацию или укажите адрес.")
		case orderStageNeedDeliveryChoice:
			prompt = t(lang, "Mahsulotni qanday olasiz?", "Как хотите получить заказ?")
		case orderStageNeedDeliveryConfirm:
			prompt = t(lang, "🚚 Yetkazib berish Yandex Go orqali amalga oshiriladi. Operator sizga aniq narxni aytadi. Rozimisiz?", "🚚 Доставка будет осуществлена через Yandex Go. Оператор сообщит вам точную стоимость. Согласны?")
		}
	}
	text := renderOrderForm(sess, lang, prompt)
	var inlineKB *tgbotapi.InlineKeyboardMarkup
	if kb != nil {
		inlineKB = kb
	}

	// Telefon bosqichi uchun contact button (reply keyboard) tayyorlab qo'yamiz
	var replyKB *tgbotapi.ReplyKeyboardMarkup
	if sess.Stage == orderStageNeedPhone {
		kbTmp := h.phoneRequestKeyboard(sess.ChatID)
		replyKB = &kbTmp
	} else if sess.Stage == orderStageNeedLocation {
		kbTmp := h.locationRequestKeyboard(sess.ChatID)
		replyKB = &kbTmp
	}

	// Orqaga tugmasi (reply keyboard bo'lmasa)
	if sess.Stage != orderStageNeedName && replyKB == nil {
		backBtn := tgbotapi.NewInlineKeyboardButtonData(t(lang, "❌ Yopish", "❌ Закрыть"), "order_close")
		if inlineKB == nil {
			tmp := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(backBtn),
			)
			inlineKB = &tmp
		} else {
			inlineKB.InlineKeyboard = append(inlineKB.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(backBtn))
		}
	}

	hasButtons := inlineKB != nil && inlineKB.InlineKeyboard != nil && len(inlineKB.InlineKeyboard) > 0

	if sess.MessageID != 0 {
		var err error
		if hasButtons {
			edit := tgbotapi.NewEditMessageTextAndMarkup(sess.ChatID, sess.MessageID, text, *inlineKB)
			_, err = h.bot.Send(edit)
		} else {
			edit := tgbotapi.NewEditMessageText(sess.ChatID, sess.MessageID, text)
			_, err = h.bot.Send(edit)
		}
		if err == nil {
			h.trackOrderMessageID(userID, sess.MessageID)
			return
		}
		log.Printf("order form edit failed user=%d chat=%d msg=%d err=%v", userID, sess.ChatID, sess.MessageID, err)
	}

	msg := tgbotapi.NewMessage(sess.ChatID, text)
	if hasButtons {
		msg.ReplyMarkup = *inlineKB
	} else if replyKB != nil && len(replyKB.Keyboard) > 0 {
		msg.ReplyMarkup = replyKB
	}
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setOrderMessageID(userID, sent.MessageID)
	} else {
		log.Printf("order form send failed user=%d chat=%d err=%v", userID, sess.ChatID, err)
	}
}

func renderOrderForm(sess *orderSession, lang, prompt string) string {
	var sb strings.Builder
	sb.WriteString(t(lang, "🧾 Buyurtma ma'lumotlari:\n", "🧾 Данные заказа:\n"))
	sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "Ism", "Имя"), nonEmpty(sess.Name, t(lang, "kiritilmagan", "не указано"))))
	sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "Telefon", "Телефон"), nonEmpty(sess.Phone, t(lang, "kiritilmagan", "не указан"))))
	sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "Manzil", "Адрес"), nonEmpty(sess.Location, t(lang, "kiritilmagan", "не указан"))))
	if sess.Delivery != "" {
		sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "Yetkazish", "Доставка"), deliveryDisplay(sess.Delivery, lang)))
	}
	if prompt != "" {
		sb.WriteString("\n")
		sb.WriteString(prompt)
	}
	return sb.String()
}

func (h *BotHandler) setOrderMessageID(userID int64, msgID int) {
	h.trackOrderMessageID(userID, msgID)
}

func (h *BotHandler) trackOrderMessageID(userID int64, msgID int) {
	if msgID == 0 {
		return
	}
	h.orderMu.Lock()
	if sess, ok := h.orderSessions[userID]; ok {
		sess.MessageID = msgID
		if !containsOrderMessageID(sess.FormMessageIDs, msgID) {
			sess.FormMessageIDs = append(sess.FormMessageIDs, msgID)
		}
		h.orderSessions[userID] = sess
	}
	h.orderMu.Unlock()
}

func (h *BotHandler) clearOrderFormMessages(userID, chatID int64, extraMsgID int) {
	ids := make(map[int]struct{})
	if extraMsgID != 0 {
		ids[extraMsgID] = struct{}{}
	}

	h.orderMu.Lock()
	cleanupHandled := false
	if info, ok := h.orderCleanup[userID]; ok {
		if extraMsgID != 0 && containsOrderMessageID(info.MessageIDs, extraMsgID) {
			if info.ChatID != 0 && chatID == 0 {
				chatID = info.ChatID
			}
			for _, msgID := range info.MessageIDs {
				if msgID != 0 {
					ids[msgID] = struct{}{}
				}
			}
			delete(h.orderCleanup, userID)
			cleanupHandled = true
		}
	}
	if !cleanupHandled {
		if sess, ok := h.orderSessions[userID]; ok && sess != nil {
			if sess.ChatID != 0 {
				chatID = sess.ChatID
			}
			if sess.MessageID != 0 {
				ids[sess.MessageID] = struct{}{}
			}
			for _, msgID := range sess.FormMessageIDs {
				if msgID != 0 {
					ids[msgID] = struct{}{}
				}
			}
			sess.MessageID = 0
			sess.FormMessageIDs = nil
			h.orderSessions[userID] = sess
		}
		if info, ok := h.orderCleanup[userID]; ok {
			if info.ChatID != 0 && chatID == 0 {
				chatID = info.ChatID
			}
			for _, msgID := range info.MessageIDs {
				if msgID != 0 {
					ids[msgID] = struct{}{}
				}
			}
			delete(h.orderCleanup, userID)
		}
	}
	h.orderMu.Unlock()

	if chatID == 0 {
		return
	}
	for msgID := range ids {
		h.deleteMessage(chatID, msgID)
	}
}

func containsOrderMessageID(ids []int, target int) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func uniqueOrderMessageIDs(ids []int) []int {
	if len(ids) == 0 {
		return ids
	}
	seen := make(map[int]struct{}, len(ids))
	var out []int
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// Send order summary to group 2
func (h *BotHandler) sendOrderToGroup2(userID int64, session *orderSession, delivery string, note string) {
	var summary string
	if strings.TrimSpace(session.ConfigTxt) != "" {
		summary = session.ConfigTxt
	}
	if strings.TrimSpace(session.Summary) != "" && !strings.EqualFold(strings.TrimSpace(session.Summary), strings.TrimSpace(summary)) {
		if summary != "" {
			summary += "\n\n"
		}
		summary += session.Summary
	}
	summaryClean := sanitizeOrderSummaryForAdmin(summary)

	// Narxni avval ConfigTxt dan, keyin umumiy summary dan qidiramiz
	totalPrice := extractTotalPrice(session.ConfigTxt)
	if totalPrice == "" {
		// Spek blokidan jami qidiramiz
		specBlock := normalizeSpecBlock(session.ConfigTxt)
		totalPrice = extractTotalPrice(specBlock)
	}
	if totalPrice == "" {
		totalPrice = extractTotalPrice(formatOrderStatusSummary(session.ConfigTxt))
	}
	if totalPrice == "" {
		totalPrice = sumPriceLines(session.ConfigTxt)
	}

	// Agar ConfigTxt bo'sh bo'lsa (savatdan kelgan orderlar), Summary dan izlaymiz
	specBlock := normalizeSpecBlock(summaryClean)
	if totalPrice == "" {
		totalPrice = extractTotalPrice(summaryClean)
	}
	if totalPrice == "" {
		totalPrice = extractTotalPrice(specBlock)
	}
	if totalPrice == "" {
		totalPrice = extractTotalPrice(formatOrderStatusSummary(summaryClean))
	}
	if totalPrice == "" {
		totalPrice = sumPriceLines(summaryClean)
	}
	if totalPrice == "" {
		totalPrice = sumPriceLines(specBlock)
	}

	totalPrice = h.formatTotalForDisplay(totalPrice)
	// PC konfiguratsiya orderlarni aniqlash: ConfigTxt mavjud bo'lsa, bu konfiguratsiyadan kelgan
	isConfig := strings.TrimSpace(session.ConfigTxt) != ""
	var displaySummary string
	// BARCHA orderlar uchun narxlarni saqlaymiz, faqat "Jami:" qatorini olib tashlaymiz
	displaySummary = removeJamiLines(specBlock)
	// Agar normalizeSpecBlock bo'sh qaytsa, original summary'dan formatDisplaySummary orqali foydalanamiz
	if strings.TrimSpace(displaySummary) == "" {
		displaySummary = formatDisplaySummary(summaryClean)
	}
	// So'nggi fallback: mutlaqo bo'sh qolsa, xom summary'ni qo'yamiz
	if strings.TrimSpace(displaySummary) == "" {
		displaySummary = summaryClean
	}
	// Jami: mavjud bo'lmasa, soddalashtirilgan summary bo'yicha hisoblaymiz
	if totalPrice == "" {
		if displayTotal := extractTotalPrice(displaySummary); displayTotal != "" {
			totalPrice = displayTotal
		} else if displayTotal := sumPriceLines(displaySummary); displayTotal != "" {
			totalPrice = displayTotal
		}
	}

	// Single item formatting (narx bilan)
	isSingle := strings.Count(displaySummary, "\n") < 2
	if isSingle && !isConfig {
		// Single item bo'lsa "-- Title - Narx" formatida
		if title := cartTitleFromText(displaySummary); title != "" {
			displaySummary = "-- " + displaySummary
		}
	}
	statusSummary := formatOrderStatusSummary(displaySummary)
	orderID := h.generateOrderID()
	location := nonEmpty(normalizeLocationText(session.Location), "ko'rsatilmagan")
	orderText := fmt.Sprintf(
		"🧾 Yangi buyurtma\nOrderID: %s\nUser: @%s\nIsm: %s\nTelefon: %s\nManzil:\n%s\nYetkazish: %s\nIzoh: %s",
		orderID,
		nonEmpty(session.Username, "nomalum"),
		nonEmpty(session.Name, "ko'rsatilmagan"),
		nonEmpty(session.Phone, "ko'rsatilmagan"),
		location,
		delivery,
		nonEmpty(note, "-"),
	)
	if totalPrice != "" {
		orderText += fmt.Sprintf("\nJami: %s", totalPrice)
	}
	orderText += fmt.Sprintf("\n\n%s", displaySummary)

	// Logistika (group_4/Topic 8) uchun - faqat active orders uchun
	if h.activeOrdersChatID != 0 {
		markup := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Tayyor", "order_ready|"+orderID),
				tgbotapi.NewInlineKeyboardButtonData("❌ Bekor qilish", "order_cancel|"+orderID),
			),
		)
		if msg, err := h.sendText(h.activeOrdersChatID, orderText, "", markup, h.activeOrdersThreadID); err != nil {
			log.Printf("Group order message send error: %v", err)
		} else {
			// Order status va mapping saqlash
			h.saveOrderStatus(orderID, orderStatusInfo{
				UserID:          userID,
				UserChat:        session.ChatID,
				Username:        session.Username,
				Phone:           session.Phone,
				Location:        location,
				Summary:         displaySummary,
				StatusSummary:   statusSummary,
				Config:          session.ConfigTxt,
				OrderID:         orderID,
				IsSingleItem:    isSingle,
				Delivery:        session.Delivery,
				Total:           totalPrice,
				Status:          "processing",
				ActiveChatID:    msg.Chat.ID,
				ActiveThreadID:  h.activeOrdersThreadID,
				ActiveMessageID: msg.MessageID,
				CreatedAt:       time.Now(),
			})
			skipInventory := isConfig && session != nil && session.InventoryReserved
			h.syncInventoryAfterOrder(displaySummary, skipInventory)
			// Agar logistika kanali group_2 bo'lsa, reply uchun mapping ham shu yerda saqlanadi
			if h.group2ChatID != 0 &&
				h.activeOrdersChatID == h.group2ChatID &&
				(h.group2ThreadID == 0 || h.activeOrdersThreadID == h.group2ThreadID) {
				h.saveGroupThread(msg.MessageID, groupThreadInfo{
					UserID:    userID,
					UserChat:  session.ChatID,
					Username:  session.Username,
					Summary:   displaySummary,
					Config:    session.ConfigTxt,
					OrderID:   orderID,
					ChatID:    msg.Chat.ID,
					ThreadID:  h.activeOrdersThreadID,
					CreatedAt: time.Now(),
				})
			}
		}
	}

	// NOTE: Topic 4 va Topic 6 ga yuborish approvaldan OLDIN bo'lishi kerak
	// Bu funksiya user formani to'ldirgandan KEYIN chaqiriladi
	// Demak faqat Topic 8 (active orders) uchun
	if session != nil && session.ChatID != 0 && shouldNotifyAfterHours(time.Now()) {
		lang := h.getUserLang(userID)
		h.sendMessage(session.ChatID, t(lang,
			"🕒 Hozir ish vaqti tugagan. Iltimos, ertalabgacha kuting — adminlar imkon qadar tez javob berishadi.",
			"🕒 Сейчас вне рабочего времени. Пожалуйста, дождитесь утра — админы ответят при первой возможности.",
		))
	}

	// Order to'liq rasmiylashtirilib tugadi:
	// - savatchadan rasmiylashtirilgan bo'lsa savatni tozalaymiz
	// - foydalanuvchi kontekstlarini (feedback/suggestion) tozalaymiz
	if session != nil && session.FromCart {
		h.clearCart(userID)
	}
	h.clearUserCart(userID)
}

func stripPriceLines(text string) string {
	var out []string
	for _, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		if strings.Contains(lower, "narxi") || strings.Contains(lower, "jami") || priceWithCurrencyRegex.MatchString(t) {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, "\n")
}
