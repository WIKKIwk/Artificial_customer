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

// Callback query larini qayta ishlash
func (h *BotHandler) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	userID := cq.From.ID
	data := cq.Data
	chatID := cq.Message.Chat.ID
	username := cq.From.UserName
	if username == "" {
		username = cq.From.FirstName
	}

	// Guruhlarda AI callbacklari ishlatilmaydi (faqat ma'lum topiklar)
	// Guruhlarda AI callbacklari faqat ma'lum chatlar uchun ishlaydi
	if cq.Message.Chat != nil && (cq.Message.Chat.IsGroup() || cq.Message.Chat.IsSuperGroup()) &&
		chatID != h.group1ChatID && chatID != h.group2ChatID && chatID != h.group4ChatID {
		return
	}

	// Callback ga javob (spinnerni to'xtatish)
	callback := tgbotapi.NewCallback(cq.ID, "")
	if _, err := h.bot.Request(callback); err != nil {
		log.Printf("Callback javobida xatolik: %v", err)
	}

	if strings.HasPrefix(data, "order_ready|") {
		orderID := strings.TrimPrefix(data, "order_ready|")
		threadID := h.activeOrdersThreadID
		if info, ok := h.getGroupThread(cq.Message.MessageID); ok {
			if info.ChatID != 0 {
				chatID = info.ChatID
			}
			if info.ThreadID != 0 {
				threadID = info.ThreadID
			}
		}
		if chatID != h.activeOrdersChatID || threadID == 0 {
			return
		}
		h.handleOrderReadyCallback(chatID, threadID, orderID, cq.Message)
		return
	}

	if strings.HasPrefix(data, "order_onway|") {
		orderID := strings.TrimPrefix(data, "order_onway|")
		threadID := h.activeOrdersThreadID
		if info, ok := h.getGroupThread(cq.Message.MessageID); ok {
			if info.ChatID != 0 {
				chatID = info.ChatID
			}
			if info.ThreadID != 0 {
				threadID = info.ThreadID
			}
		}
		if chatID != h.activeOrdersChatID || threadID == 0 {
			return
		}
		h.handleOrderOnWayCallback(chatID, threadID, userID, orderID)
		return
	}

	if strings.HasPrefix(data, "order_cancel|") {
		orderID := strings.TrimPrefix(data, "order_cancel|")
		threadID := h.activeOrdersThreadID
		if info, ok := h.getGroupThread(cq.Message.MessageID); ok {
			if info.ChatID != 0 {
				chatID = info.ChatID
			}
			if info.ThreadID != 0 {
				threadID = info.ThreadID
			}
		}
		if chatID != h.activeOrdersChatID || threadID == 0 {
			return
		}
		h.handleOrderCancelCallback(chatID, threadID, orderID, cq.Message)
		return
	}

	if strings.HasPrefix(data, "sticker_set|") {
		raw := strings.TrimPrefix(data, "sticker_set|")
		var slot stickerSlot
		switch raw {
		case "login":
			slot = stickerSlotLogin
		case "order_placed":
			slot = stickerSlotOrderPlaced
		default:
			h.sendMessage(chatID, "❌ Noto'g'ri tanlov.")
			return
		}
		h.handleStickerSelectCallback(ctx, chatID, userID, slot)
		return
	}

	if strings.HasPrefix(data, "sticker_clear|") {
		raw := strings.TrimPrefix(data, "sticker_clear|")
		var slot stickerSlot
		switch raw {
		case "login":
			slot = stickerSlotLogin
		case "order_placed":
			slot = stickerSlotOrderPlaced
		default:
			h.sendMessage(chatID, "❌ Noto'g'ri tanlov.")
			return
		}
		h.handleStickerClearCallback(ctx, chatID, userID, slot, cq.Message)
		return
	}

	if strings.HasPrefix(data, "sticker_enabled|") {
		raw := strings.TrimPrefix(data, "sticker_enabled|")
		enabled := raw == "1" || strings.EqualFold(raw, "on") || strings.EqualFold(raw, "true")
		h.handleStickerEnabledCallback(ctx, chatID, userID, enabled, cq.Message)
		return
	}

	if strings.HasPrefix(data, "db_select|") {
		isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
		if !isAdmin {
			h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
			return
		}
		arg := strings.TrimPrefix(data, "db_select|")
		arg = strings.TrimSpace(strings.ToLower(arg))
		if arg == "latest" || arg == "clear" || arg == "reset" || arg == "0" {
			if err := clearSheetMasterDBSelection(); err != nil {
				h.sendMessage(chatID, fmt.Sprintf("❌ Tanlovni tozalashda xatolik: %v", err))
				return
			}
			h.sendMessage(chatID, "✅ Tanlov tozalandi. Endi /import_now oxirgi faylni oladi.")
			return
		}
		id, err := strconv.ParseUint(arg, 10, 64)
		if err != nil || id == 0 {
			h.sendMessage(chatID, "❌ Fayl ID noto'g'ri.")
			return
		}
		h.applyDatabaseSelection(ctx, chatID, uint(id))
		return
	}
	if strings.HasPrefix(data, "order_view|") {
		orderID := strings.TrimPrefix(data, "order_view|")
		h.handleOrderViewCallback(chatID, userID, orderID)
		return
	}
	if data == "order_clear_all" {
		h.handleOrderClearAll(chatID, userID, cq.Message)
		return
	}

	if strings.HasPrefix(data, "cfg_type_") {
		val := strings.TrimPrefix(data, "cfg_type_")
		h.handleConfigTypeSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_color_") {
		val := strings.TrimPrefix(data, "cfg_color_")
		h.handleConfigColorSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_cpu_") {
		val := strings.TrimPrefix(data, "cfg_cpu_")
		h.handleConfigCPUSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_cooler_") {
		val := strings.TrimPrefix(data, "cfg_cooler_")
		h.handleConfigCPUCoolerSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_storage_") {
		val := strings.TrimPrefix(data, "cfg_storage_")
		h.handleConfigStorageSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_gpu_") {
		val := strings.TrimPrefix(data, "cfg_gpu_")
		h.handleConfigGPUSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_monitor_hz_") {
		val := strings.TrimPrefix(data, "cfg_monitor_hz_")
		h.handleConfigMonitorHzSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_monitor_display_") {
		val := strings.TrimPrefix(data, "cfg_monitor_display_")
		h.handleConfigMonitorDisplaySelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_monitor_") {
		val := strings.TrimPrefix(data, "cfg_monitor_")
		h.handleConfigMonitorSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_peripherals_") {
		val := strings.TrimPrefix(data, "cfg_peripherals_")
		h.handleConfigPeripheralsSelection(ctx, userID, chatID, cq.Message, username, val)
		return
	}
	if strings.HasPrefix(data, "cfg_del_") {
		val := strings.TrimPrefix(data, "cfg_del_")
		h.handleDeleteAction(ctx, userID, chatID, val)
		return
	}

	// Broadcast callbacks
	if strings.HasPrefix(data, "broadcast_confirm:") {
		isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
		if !isAdmin {
			h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
			return
		}
		broadcastMsg := strings.TrimPrefix(data, "broadcast_confirm:")
		h.handleBroadcastConfirm(ctx, chatID, userID, broadcastMsg)
		return
	}
	if data == "broadcast_cancel" {
		h.sendMessage(chatID, "❌ Broadcast bekor qilindi.")
		return
	}

	// Order status change callbacks
	if strings.HasPrefix(data, "ordstat_") {
		isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
		if !isAdmin {
			h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
			return
		}
		// Format: ordstat_<status>:<orderID>
		parts := strings.SplitN(strings.TrimPrefix(data, "ordstat_"), ":", 2)
		if len(parts) == 2 {
			newStatus := parts[0]
			orderID := parts[1]
			h.handleOrderStatusChange(ctx, chatID, userID, orderID, newStatus)
		}
		return
	}

	// Message to customer callback
	if strings.HasPrefix(data, "ordmsg:") {
		isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
		if !isAdmin {
			h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
			return
		}
		// Format: ordmsg:<orderID>:<userChatID>
		parts := strings.SplitN(strings.TrimPrefix(data, "ordmsg:"), ":", 2)
		if len(parts) == 2 {
			orderID := parts[0]
			// userChatID := parts[1] - saved for future use
			h.sendMessage(chatID, fmt.Sprintf("💬 Mijozga xabar yuborish uchun:\n\nOrderID: %s\n\nXabar matnini yozing:", orderID))
		}
		return
	}

	parts := strings.SplitN(data, "|", 2)
	cmd := parts[0]
	offerID := ""
	if len(parts) == 2 {
		offerID = parts[1]
	}
	lang := h.getUserLang(userID)

	switch cmd {
	case "order_close":
		h.releaseReservedInventory(userID)
		extraMsgID := 0
		if cq.Message != nil {
			extraMsgID = cq.Message.MessageID
		}
		h.clearOrderFormMessages(userID, chatID, extraMsgID)
		h.clearOrderSession(userID)
		return
	case "lang":
		if len(parts) == 2 {
			lang := parts[1]
			h.setUserLang(userID, lang)
			h.maybeAskProfile(userID, chatID, lang)
		}
	case "lang_ru":
		h.setUserLang(userID, "ru")
		h.maybeAskProfile(userID, chatID, "ru")
	case "lang_uz":
		h.setUserLang(userID, "uz")
		h.maybeAskProfile(userID, chatID, "uz")
	case "cfg_fb_yes":
		h.keepAnalyzePCButtonOnly(cq, offerID, lang)
		h.setConfigOrderLocked(userID, true)
		// Config session'ni yopish (agar ochiq bo'lsa)
		h.configMu.Lock()
		if _, exists := h.configSessions[userID]; exists {
			delete(h.configSessions, userID)
			log.Printf("[cfg_fb_yes] Config session yopildi: userID=%d", userID)
		}
		h.configMu.Unlock()

		// Feedback tozalanmasin - faqat o'qiymiz
		// Order to'liq rasmiylashtirilib tugagandan keyin tozalanadi
		info, ok := h.getFeedbackByID(offerID)
		if !ok {
			info, ok = h.getFeedback(userID)
		}
		var orderID string
		if ok {
			orderID = info.OrderID
			if orderID == "" {
				orderID = h.generateOrderID()
			}
		}
		// Admin approval uchun Topic 4 ga yuborish (PC configuration)
		if ok && h.group1ChatID != 0 {
			info.OrderID = orderID
			specBlock := normalizeSpecBlock(info.ConfigText)
			if specBlock == "" {
				specBlock = formatDisplaySummary(info.ConfigText)
			}
			approvalText := fmt.Sprintf(
				"🖥️ Yangi PC konfiguratsiya so'rovi\nUser: @%s\n\n%s\n\nReply qiling mijozga javob berish uchun.",
				nonEmpty(info.Username, "nomalum"),
				specBlock,
			)
			if msg, err := h.sendText(h.group1ChatID, approvalText, "", nil, h.group1ThreadID); err != nil {
				log.Printf("❌ PC config approval request send error: %v", err)
			} else {
				log.Printf("✅ PC config approval request sent to Topic 4 for user %d, messageID=%d", userID, msg.MessageID)
				h.markGroup1PendingApproval(msg.MessageID)
				h.saveGroupThread(msg.MessageID, groupThreadInfo{
					UserID:    userID,
					UserChat:  info.ChatID,
					Username:  info.Username,
					Summary:   specBlock,
					Config:    specBlock,
					OrderID:   orderID,
					ChatID:    h.group1ChatID,
					ThreadID:  h.group1ThreadID,
					CreatedAt: time.Now(),
				})
			}
		}

		// Userga: admin tasdiqlashi kutilmoqda, rasmiylashtirish keyin
		if ok {
			lang := h.getUserLang(userID)
			h.sendMessage(chatID, adminApprovalWaitMessage(lang, time.Now()))
		} else {
			h.sendMessage(chatID, "Konfiguratsiya ma'lumoti topilmadi. Iltimos qayta urinib ko'ring.")
		}
	case "cfg_fb_no":
		// Config session'ni yopish (agar ochiq bo'lsa)
		h.configMu.Lock()
		if _, exists := h.configSessions[userID]; exists {
			delete(h.configSessions, userID)
			log.Printf("[cfg_fb_no] Config session yopildi: userID=%d", userID)
		}
		h.configMu.Unlock()

		h.sendConfigRetryPrompt(chatID, lang)
		if offerID != "" {
			h.popFeedbackByID(offerID)
		} else {
			h.popFeedback(userID) // tozalash
		}
	case "cfg_fb_change":
		if offerID != "" {
			if info, ok := h.getFeedbackByID(offerID); ok {
				h.setPendingChange(userID, changeRequest{Component: "other", Spec: info.Spec})
			}
		} else if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "other", Spec: info.Spec})
		}
		h.sendMessage(chatID, "Qaysi komponentni o'zgartirish kerakligini yozib yuboring.")
	case "cfg_fb_delete":
		h.sendDeleteComponentPrompt(chatID)
	case "cfg_analyze_pc":
		// PC Analyze tugmasi bosildi
		var configText string
		var purposeHint string
		if offerID != "" {
			if info, ok := h.getFeedbackByID(offerID); ok {
				configText = info.ConfigText
				purposeHint = info.Spec.PCType
			}
		}
		if configText == "" {
			if info, ok := h.getLatestFeedback(userID); ok {
				configText = info.ConfigText
				if purposeHint == "" {
					purposeHint = info.Spec.PCType
				}
			}
		}
		if configText != "" {
			h.handlePCAnalysisRequest(ctx, userID, username, chatID, configText, purposeHint)
		} else {
			h.sendMessage(chatID, t(lang, "❌ PC konfiguratsiyasi topilmadi. Iltimos, /configuratsiya ni qaytadan bosing.", "❌ Конфигурация ПК не найдена. Пожалуйста, нажмите /configuratsiya еще раз."))
		}
	case "cfg_change_cpu":
		if offerID != "" {
			if info, ok := h.getFeedbackByID(offerID); ok {
				h.setPendingChange(userID, changeRequest{Component: "cpu", Spec: info.Spec})
			}
		} else if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "cpu", Spec: info.Spec})
		}
		h.sendMessage(chatID, "CPU o'rnida qanday modelni xohlaysiz? To'liq nomini yozib yuboring.")
	case "cfg_change_gpu":
		if offerID != "" {
			if info, ok := h.getFeedbackByID(offerID); ok {
				h.setPendingChange(userID, changeRequest{Component: "gpu", Spec: info.Spec})
			}
		} else if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "gpu", Spec: info.Spec})
		}
		h.sendMessage(chatID, "GPU o'rnida qanday modelni xohlaysiz? To'liq nomini yozib yuboring.")
	case "cfg_change_ram":
		if offerID != "" {
			if info, ok := h.getFeedbackByID(offerID); ok {
				h.setPendingChange(userID, changeRequest{Component: "ram", Spec: info.Spec})
			}
		} else if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "ram", Spec: info.Spec})
		}
		h.sendMessage(chatID, "RAM uchun qanday hajm/tezlikni xohlaysiz? Masalan: 32GB DDR5 6000MHz.")
	case "cfg_change_ssd":
		if offerID != "" {
			if info, ok := h.getFeedbackByID(offerID); ok {
				h.setPendingChange(userID, changeRequest{Component: "ssd", Spec: info.Spec})
			}
		} else if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "ssd", Spec: info.Spec})
		}
		h.sendMessage(chatID, "Qancha hajmdagi SSD xohlaysiz? Masalan: 1TB NVMe Gen4.")
	case "cfg_change_other":
		if offerID != "" {
			if info, ok := h.getFeedbackByID(offerID); ok {
				h.setPendingChange(userID, changeRequest{Component: "other", Spec: info.Spec})
			}
		} else if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "other", Spec: info.Spec})
		}
		h.sendMessage(chatID, "Qaysi komponentni o'zgartirish kerakligini yozib yuboring.")
	case "order_yes":
		info, ok := h.popPendingApproval(userID)
		log.Printf("[order_yes] user=%d pending found=%v", userID, ok)
		if !ok {
			// Fallback: oxirgi konfiguratsiya yoki taklifni ishlatamiz
			if fb, okFb := h.getLatestFeedback(userID); okFb && strings.TrimSpace(fb.ConfigText) != "" {
				info = pendingApproval{
					UserID:   userID,
					UserChat: chatID,
					Summary:  fb.ConfigText,
					Config:   fb.ConfigText,
					Username: fb.Username,
					SentAt:   time.Now(),
				}
				ok = true
			} else if last, okLast := h.getLastSuggestion(userID); okLast && strings.TrimSpace(last) != "" {
				info = pendingApproval{
					UserID:   userID,
					UserChat: chatID,
					Summary:  last,
					Config:   last,
					Username: username,
					SentAt:   time.Now(),
				}
				ok = true
			}
		}
		if !ok {
			h.sendMessage(chatID, "❌ Buyurtma ma'lumotlari topilmadi. Iltimos qaytadan urinib ko'ring.")
			return
		}

		// ChatID/Username ni to'ldirib qo'yamiz
		if info.UserChat == 0 {
			info.UserChat = chatID
		}
		if info.Username == "" {
			info.Username = username
		}

		// Order sessionni har doim yaratamiz
		h.startOrderSession(userID, info)
		log.Printf("[order_yes] session started user=%d chat=%d summary_len=%d", userID, info.UserChat, len(info.Config))
		h.sendOrderForm(userID, "", nil)
	case "order_no":
		info, ok := h.popPendingApproval(userID)
		if !ok {
			// Fallback: oxirgi taklifni ishlatamiz
			if fb, okFb := h.getLatestFeedback(userID); okFb {
				info = pendingApproval{Config: fb.ConfigText, Summary: fb.ConfigText}
				ok = true
			} else if last, okLast := h.getLastSuggestion(userID); okLast {
				info = pendingApproval{Config: last, Summary: last}
				ok = true
			}
		}
		h.sendMessage(chatID, "😔 Afsusdamiz. Ketkazgan vaqtingiz uchun uzr so'raymiz.")
		if ok && strings.TrimSpace(info.Config) != "" {
			// Otmen qilingan bo'lsa ham 5 daqiqa ichida eslatma yuboramiz
			h.scheduleConfigReminder(userID, chatID, info.Config)
		}
	case "delivery_pickup":
		h.handleDeliveryChoice(ctx, userID, "pickup", chatID)
	case "delivery_courier":
		h.handleDeliveryChoice(ctx, userID, "courier", chatID)
	case "delivery_confirm_yes":
		h.handleDeliveryConfirm(ctx, userID, true, chatID)
	case "delivery_confirm_no":
		h.handleDeliveryConfirm(ctx, userID, false, chatID)
	case "order_back":
		h.handleOrderBack(userID, chatID, cq.Message)
	case "purchase_yes":
		h.handlePurchaseYes(ctx, userID, username, chatID, offerID, cq)
	case "purchase_no":
		// Sotib olishdan voz kechdi
		// Config session'ni yopish (agar ochiq bo'lsa)
		h.configMu.Lock()
		if _, exists := h.configSessions[userID]; exists {
			delete(h.configSessions, userID)
			log.Printf("[purchase_no] Config session yopildi: userID=%d", userID)
		}
		h.configMu.Unlock()

		if offerID != "" {
			h.popFeedbackByID(offerID)
		} else {
			h.popFeedback(userID)
		}
		h.sendMessage(chatID, t(lang, "👌 Tushunarli. Boshqa savollar bo'lsa, menga yozishingiz mumkin!", "👌 Понял. Если будут вопросы, пишите!"))
	case "online_refresh":
		h.handleOnlineRefreshCallback(ctx, chatID, userID, cq.Message)
	case "online_back":
		h.handleOnlineBackCallback(chatID, userID, cq.Message)
	case "users_refresh":
		h.handleUsersRefreshCallback(ctx, chatID, userID, cq.Message)
	case "user_history_close":
		if cq.Message == nil {
			return
		}
		msgs := h.popUserHistoryMessages(cq.Message.Chat.ID, cq.Message.MessageID)
		if len(msgs) == 0 {
			h.deleteMessage(cq.Message.Chat.ID, cq.Message.MessageID)
			return
		}
		seen := make(map[string]struct{})
		for _, m := range msgs {
			key := fmt.Sprintf("%d-%d", m.chatID, m.msgID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			h.deleteMessage(m.chatID, m.msgID)
		}
		return
	case "val_sum":
		h.setAwaitingCurrencyRate(userID, true)
		h.setCurrencyMode("sum", 0)
		h.sendMessage(chatID, "USD -> so'm kursini kiriting. Masalan: 12500")
	case "val_usd":
		h.setAwaitingCurrencyRate(userID, false)
		h.setCurrencyMode("usd", 0)
		h.sendMessage(chatID, "✅ USD rejimi yoqildi. AI javoblari $ ko'rinishida bo'ladi.")
	case "config_start":
		if !h.isConfigCTAValid(chatID, cq.Message.MessageID) {
			return
		}
		h.clearConfigCTA(chatID)
		h.startConfigWizard(userID, chatID, cq.Message.MessageID)
	case "cart_add":
		h.handleCartAddCallback(chatID, userID, offerID, cq.Message)
	case "cart_checkout":
		h.handleCartCheckoutCallback(chatID, userID, offerID)
	case "cart_clear":
		h.handleCartClearCallback(chatID, userID, cq.Message)
	case "cart_clear_all":
		h.handleCartClearAllCallback(chatID, userID, cq.Message)
	case "cart_del":
		h.handleCartDeleteCallback(chatID, userID, offerID, cq.Message)
	case "cart_open":
		h.handleCartOpenCallback(chatID, userID, cq.Message)
	case "cart_back":
		h.handleCartBack(chatID, userID, cq.Message)
	case "cart_checkout_all":
		texts := h.listCart(userID)
		if len(texts) == 0 {
			h.sendMessage(chatID, "🛒 Savat bo'sh.")
			return
		}
		// Combine all items text
		var combined []string
		for _, it := range texts {
			if strings.TrimSpace(it.Text) != "" {
				combined = append(combined, it.Text)
			} else {
				combined = append(combined, it.Title)
			}
		}
		full := strings.Join(combined, "\n\n")

		// Order session boshlash (Topic 6 approval o'chirildi - to'g'ridan Topic 8 ga)
		h.startOrderSession(userID, pendingApproval{
			UserID:   userID,
			UserChat: chatID,
			Summary:  full,
			Config:   "", // Savatchadan kelgan orderlar uchun Config bo'sh
			Username: username,
			SentAt:   time.Now(),
		})
		// Savatchadan rasmiylashtirilgan order: jarayon tugagach savatni tozalash uchun belgilaymiz.
		h.orderMu.Lock()
		if sess, ok := h.orderSessions[userID]; ok && sess != nil {
			sess.FromCart = true
			h.orderSessions[userID] = sess
		}
		h.orderMu.Unlock()
		h.sendOrderForm(userID, "", nil)
	case "analyze_pc":
		// Analyze PC callback
		h.handleAnalyzePCCallback(ctx, cq)
	case "purchase_config":
		h.clearInlineButtons(cq)
		h.setConfigOrderLocked(userID, true)
		// Analyze PC dan keyin konfiguratsiyani sotib olish
		// Config session'ni yopib qo'yamiz (ehtiyot uchun)
		h.configMu.Lock()
		if _, exists := h.configSessions[userID]; exists {
			delete(h.configSessions, userID)
			log.Printf("[purchase_config] Config session yopildi: userID=%d", userID)
		}
		h.configMu.Unlock()

		info, ok := h.getLatestFeedback(userID)
		if !ok {
			info, ok = h.getFeedback(userID)
		}
		configText := ""
		if ok {
			configText = info.ConfigText
		}
		if strings.TrimSpace(configText) == "" {
			if last, okLast := h.getLastSuggestion(userID); okLast {
				configText = last
				info.Username = username
			}
		}
		if strings.TrimSpace(configText) == "" {
			h.sendMessage(chatID, t(lang, "❌ PC konfiguratsiyasi topilmadi. Iltimos, /configuratsiya ni qaytadan bosing.", "❌ Конфигурация ПК не найдена. Пожалуйста, нажмите /configuratsiya еще раз."))
			return
		}

		orderID := info.OrderID
		if orderID == "" {
			orderID = h.generateOrderID()
		}

		// Admin approval uchun Topic 4 ga yuborish (PC configuration)
		if h.group1ChatID != 0 {
			specBlock := normalizeSpecBlock(configText)
			if specBlock == "" {
				specBlock = formatDisplaySummary(configText)
			}
			approvalText := fmt.Sprintf(
				"🖥️ Yangi PC konfiguratsiya so'rovi\nUser: @%s\n\n%s\n\nReply qiling mijozga javob berish uchun.",
				nonEmpty(info.Username, "nomalum"),
				specBlock,
			)
			if msg, err := h.sendText(h.group1ChatID, approvalText, "", nil, h.group1ThreadID); err != nil {
				log.Printf("❌ PC config approval request send error: %v", err)
			} else {
				log.Printf("✅ PC config approval request sent to Topic 4 for user %d, messageID=%d", userID, msg.MessageID)
				h.markGroup1PendingApproval(msg.MessageID)
				h.saveGroupThread(msg.MessageID, groupThreadInfo{
					UserID:    userID,
					UserChat:  chatID,
					Username:  nonEmpty(info.Username, username),
					Summary:   specBlock,
					Config:    specBlock,
					OrderID:   orderID,
					ChatID:    h.group1ChatID,
					ThreadID:  h.group1ThreadID,
					CreatedAt: time.Now(),
				})
			}
		}

		h.sendMessage(chatID, adminApprovalWaitMessage(lang, time.Now()))
	case "new_config":
		// Yangi konfiguratsiya
		h.sendMessage(chatID, t(lang, "🔄 Yangi konfiguratsiya uchun /configuratsiya ni bosing", "🔄 Для новой конфигурации нажмите /configuratsiya"))
	default:
		// boshqa callback lar uchun hech narsa qilmaymiz
	}
}
