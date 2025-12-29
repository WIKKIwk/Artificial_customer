package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleOrderReadyCallback when admin confirms order is ready
func (h *BotHandler) handleOrderReadyCallback(chatID int64, threadID int, orderID string, srcMsg *tgbotapi.Message) {
	info, ok := h.getOrderStatus(orderID)
	if !ok {
		h.sendText(chatID, "❌ Order topilmadi.", "", nil, threadID)
		return
	}
	// Active orderdagi xabarni keyinroq o'chirish uchun (delivery/ETA bosqichida ham kerak bo'ladi).
	if srcMsg != nil && srcMsg.MessageID != 0 {
		info.ActiveChatID = chatID
		info.ActiveThreadID = threadID
		info.ActiveMessageID = srcMsg.MessageID
		h.orderStatusMu.Lock()
		h.orderStatuses[orderID] = info
		h.orderStatusMu.Unlock()
	}
	pickup := strings.ToLower(info.Delivery) == "pickup"
	lang := h.getUserLang(info.UserID)
	status := "ready_delivery"
	readyAlready := info.Status == "ready_delivery" || info.Status == "ready_pickup"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n🆔 OrderID: %s\n", t(lang, "✅ Buyurtma tayyor!", "✅ Заказ готов!"), orderID))
	if pickup {
		status = "ready_pickup"
		sb.WriteString(t(lang, "📍 Olib ketish: punktdan olib ketishingiz mumkin.", "📍 Самовывоз: можете забрать в пункте.") + "\n")
	} else {
		sb.WriteString(t(lang, "🚚 Yetkazish: tayyor.", "🚚 Доставка: готов.") + "\n")
	}

	if info.Total != "" {
		sb.WriteString(fmt.Sprintf("%s: %s\n", t(lang, "💰 Jami", "💰 Итого"), h.formatTotalForDisplay(info.Total)))
	}

	detail := nonEmpty(info.Summary, info.StatusSummary)
	if detail != "" {
		sb.WriteString(fmt.Sprintf("%s:\n%s\n", t(lang, "🧾 Tafsilotlar", "🧾 Детали"), detail))
	}

	if !readyAlready {
		h.sendMessage(info.UserChat, sb.String())
	}
	h.setOrderStatus(orderID, status)

	// Groupdagi xabarni edit qilish (yangi xabar tashlamaslik uchun)
	loc := nonEmpty(normalizeLocationText(info.Location), "ko'rsatilmagan")
	editText := fmt.Sprintf("✅ Tasdiqlangan buyurtma\nOrderID: %s\nUsername: @%s\nTelefon: %s\nManzil:\n%s\nYetkazish: %s\nJami: %s\n\n%s",
		orderID,
		nonEmpty(info.Username, "nomalum"),
		nonEmpty(info.Phone, "ko'rsatilmagan"),
		loc,
		nonEmpty(deliveryDisplay(info.Delivery, "uz"), "-"),
		nonEmpty(h.formatTotalForDisplay(info.Total), "-"),
		nonEmpty(info.Summary, info.StatusSummary),
	)

	var markup *tgbotapi.InlineKeyboardMarkup
	if h.activeOrdersChatID != 0 && !pickup {
		m := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🚚 Yo'lga chiqdi", "order_onway|"+orderID),
			),
		)
		markup = &m
	}

	if srcMsg != nil {
		edit := tgbotapi.NewEditMessageText(chatID, srcMsg.MessageID, editText)
		edit.ReplyMarkup = markup
		if _, err := h.bot.Send(edit); err != nil {
			log.Printf("order ready edit failed: %v", err)
		}
	}

	// Group3 (tasdiqlangan buyurtmalar) ga yuborish:
	//  - pickup bo'lsa darhol
	//  - delivery bo'lsa keyinroq (ETA kiritilgach) handleGroup2Message da yuboriladi
	if pickup && h.group3ChatID != 0 && !readyAlready && !(h.group3ChatID == chatID && h.group3ThreadID == threadID) {
		grpSummary := sanitizeSummaryForGroup3(nonEmpty(info.Summary, info.StatusSummary))
		loc := nonEmpty(normalizeLocationText(info.Location), "ko'rsatilmagan")
		msg := fmt.Sprintf("✅ Tasdiqlangan buyurtma\nOrderID: %s\nUsername: @%s\nTelefon: %s\nManzil:\n%s\nYetkazish: %s\nJami: %s\n\n%s",
			orderID,
			nonEmpty(info.Username, "nomalum"),
			nonEmpty(info.Phone, "ko'rsatilmagan"),
			loc,
			nonEmpty(deliveryDisplay(info.Delivery, "uz"), "-"),
			nonEmpty(h.applyCurrencyPreference(info.Total), "-"),
			grpSummary,
		)
		if _, err := h.sendText(h.group3ChatID, msg, "", nil, h.group3ThreadID); err != nil {
			log.Printf("group3 send failed (ready/pickup) order=%s err=%v", orderID, err)
		} else {
			// Completed (group3) ga tushgach, active orderdagi xabarni o'chiramiz.
			if srcMsg != nil {
				h.deleteMessage(chatID, srcMsg.MessageID)
			} else if info.ActiveChatID != 0 && info.ActiveMessageID != 0 {
				h.deleteMessage(info.ActiveChatID, info.ActiveMessageID)
			}
		}
	}

}

func (h *BotHandler) handleOrderOnWayCallback(chatID int64, threadID int, adminID int64, orderID string) {
	info, ok := h.getOrderStatus(orderID)
	if !ok {
		h.sendText(chatID, "❌ Order topilmadi.", "", nil, threadID)
		return
	}
	if strings.EqualFold(info.Delivery, "pickup") {
		h.sendText(chatID, "🏬 Bu buyurtma olib ketish uchun. Yetkazib berish kerak emas.", "", nil, threadID)
		return
	}
	h.setPendingETA(adminID, orderID, chatID, threadID)
	if sent, err := h.sendText(chatID, "🚚 Necha vaqt ichida yetib boradi? Masalan: 2 soatda yoki 30 daqiqada.", "", nil, threadID); err != nil {
		log.Printf("eta prompt send failed order=%s err=%v", orderID, err)
	} else if sent != nil {
		info.ETAPromptChatID = sent.Chat.ID
		info.ETAPromptThread = threadID
		info.ETAPromptMsgID = sent.MessageID
		h.orderStatusMu.Lock()
		h.orderStatuses[orderID] = info
		h.orderStatusMu.Unlock()
	}
	h.setOrderStatus(orderID, "onway")
}

// Group2 ETA/input handler
func (h *BotHandler) handleGroup2Message(ctx context.Context, message *tgbotapi.Message) {
	adminID := message.From.ID
	if message.Text == "" {
		return
	}

	orderID, ok := h.getPendingETA(adminID)
	threadID := h.activeOrdersThreadID
	if message.ReplyToMessage != nil {
		if info, ok := h.getGroupThread(message.ReplyToMessage.MessageID); ok && info.ThreadID != 0 {
			threadID = info.ThreadID
		}
	}
	if !ok {
		orderID, ok = h.popPendingETAByChat(message.Chat.ID, threadID)
		if !ok && h.activeOrdersThreadID > 0 {
			orderID, ok = h.popPendingETAByChat(message.Chat.ID, h.activeOrdersThreadID)
		}
		if !ok {
			h.sendText(message.Chat.ID, "ℹ️ Yetkazish vaqti kiritish rejimi topilmadi. \"🚚 Yo'lga chiqdi\" tugmasini bosib qayta urinib ko'ring.", "", nil, threadID)
			return
		}
	}

	// Mavjud bo'lmagan chatda yozilgan bo'lsa tozalaymiz va ogohlantiramiz
	if threadID == 0 && h.threadIDForChat(message.Chat.ID) != 0 {
		h.clearETA(orderID, adminID, message.Chat.ID, h.threadIDForChat(message.Chat.ID))
		h.sendText(message.Chat.ID, "ℹ️ Iltimos, shu buyurtma uchun yaratilgan topikda yozing (\"🚚 Yo'lga chiqdi\" tugmasini qayta bosing).", "", nil, threadID)
		return
	}

	info, ok := h.getOrderStatus(orderID)
	if !ok {
		h.sendText(message.Chat.ID, "❌ Order topilmadi.", "", nil, threadID)
		return
	}

	duration := strings.TrimSpace(message.Text)
	if duration == "" {
		h.setPendingETA(adminID, orderID, message.Chat.ID, threadID)
		h.sendText(message.Chat.ID, "❌ Bo'sh vaqt. Iltimos, necha soat/daqiqada yetishini yozing.", "", nil, threadID)
		return
	}

	onWayText := fmt.Sprintf("🚚 Buyurtmangiz yo'lga chiqdi. (OrderID: %s)\nTaxminiy yetib borish vaqti: %s.\n%s",
		orderID,
		duration,
		nonEmpty(info.StatusSummary, "Tafsilotlar mavjud emas."),
	)
	if _, err := h.sendText(info.UserChat, onWayText, "", nil, 0); err != nil {
		h.setPendingETA(adminID, orderID, message.Chat.ID, threadID)
		h.sendText(message.Chat.ID, "❌ Foydalanuvchiga yuborib bo'lmadi. Qayta urinib ko'ring yoki foydalanuvchi chatini tekshiring.", "", nil, threadID)
		return
	}

	// Muvaffaqiyatli bo'lsa, holatni tozalaymiz
	h.clearETA(orderID, adminID, message.Chat.ID, threadID)

	confirmMsg, _ := h.sendText(message.Chat.ID, fmt.Sprintf("🚚 Foydalanuvchiga yo'lga chiqdi (%s) deb yuborildi.", duration), "", nil, threadID)

	// Delivery bo'lsa: group3 ga endi yuboramiz
	if h.group3ChatID != 0 && strings.ToLower(info.Delivery) != "pickup" {
		grpSummary := sanitizeSummaryForGroup3(nonEmpty(info.Summary, info.StatusSummary))
		loc := nonEmpty(normalizeLocationText(info.Location), "ko'rsatilmagan")
		msg := fmt.Sprintf("🚚 Yo'lga chiqdi\nOrderID: %s\nUsername: @%s\nTelefon: %s\nManzil:\n%s\nYetkazish: %s\nJami: %s\n\n%s",
			orderID,
			nonEmpty(info.Username, "nomalum"),
			nonEmpty(info.Phone, "ko'rsatilmagan"),
			loc,
			nonEmpty(deliveryDisplay(info.Delivery, "uz"), "-"),
			nonEmpty(h.formatTotalForDisplay(info.Total), "-"),
			grpSummary,
		)
		if _, err := h.sendText(h.group3ChatID, msg, "", nil, h.group3ThreadID); err != nil {
			log.Printf("group3 send failed (onway/delivery) order=%s err=%v", orderID, err)
		} else {
			// Completed (group3) ga tushgach, active orderdagi xabarlarni to'liq tozalaymiz.
			if info.ActiveChatID != 0 && info.ActiveMessageID != 0 {
				h.deleteMessage(info.ActiveChatID, info.ActiveMessageID)
			}
			if info.ETAPromptChatID != 0 && info.ETAPromptMsgID != 0 {
				h.deleteMessage(info.ETAPromptChatID, info.ETAPromptMsgID)
			}
			if confirmMsg != nil {
				h.deleteMessage(confirmMsg.Chat.ID, confirmMsg.MessageID)
			}
			// Admin yozgan vaqt xabarini ham o'chiramiz
			h.deleteMessage(message.Chat.ID, message.MessageID)

			info.ETAPromptChatID = 0
			info.ETAPromptThread = 0
			info.ETAPromptMsgID = 0
			h.orderStatusMu.Lock()
			h.orderStatuses[orderID] = info
			h.orderStatusMu.Unlock()
		}
	}
}

func sanitizeSummaryForGroup3(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		// Ha, bizda bor: -- ... dan faqat model nomini ajratamiz
		if strings.HasPrefix(lower, "ha, bizda bor") {
			if idx := strings.Index(t, "--"); idx >= 0 {
				model := strings.TrimSpace(t[idx+2:])
				model = strings.Trim(model, "-: ")
				if model != "" {
					out = append(out, model)
					continue
				}
			}
			continue
		}
		if strings.Contains(lower, "narxi") || strings.Contains(lower, "jami") {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, "\n")
}
