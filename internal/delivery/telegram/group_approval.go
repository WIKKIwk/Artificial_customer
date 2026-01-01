package telegram

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var orderIDPattern = regexp.MustCompile(`(?i)orderid[:\s]*([0-9]{6,}-\d{2,})`)

// Group thread mapping
func (h *BotHandler) saveGroupThread(messageID int, info groupThreadInfo) {
	h.groupMu.Lock()
	defer h.groupMu.Unlock()
	h.groupThreads[messageID] = info
}

func (h *BotHandler) getGroupThread(messageID int) (groupThreadInfo, bool) {
	h.groupMu.RLock()
	defer h.groupMu.RUnlock()
	info, ok := h.groupThreads[messageID]
	return info, ok
}

// findGroupThreadByOrderID - OrderID bo'yicha mappingni topish
func (h *BotHandler) findGroupThreadByOrderID(orderID string) (groupThreadInfo, bool) {
	if strings.TrimSpace(orderID) == "" {
		return groupThreadInfo{}, false
	}
	h.groupMu.RLock()
	defer h.groupMu.RUnlock()
	for _, info := range h.groupThreads {
		if strings.EqualFold(info.OrderID, orderID) {
			return info, true
		}
	}
	return groupThreadInfo{}, false
}

// getLatestGroupThread returns the most recent mapping for a given group chat (fallback)
func (h *BotHandler) getLatestGroupThread(chatID int64) (groupThreadInfo, bool) {
	h.groupMu.RLock()
	defer h.groupMu.RUnlock()
	var latest groupThreadInfo
	var found bool
	for _, info := range h.groupThreads {
		if info.ChatID != 0 && info.ChatID != chatID {
			continue
		}
		// Agar ChatID bo'sh bo'lsa, shu chatga tegishli deb qabul qilamiz (retro mapping)
		info.ChatID = chatID
		if !found || info.CreatedAt.After(latest.CreatedAt) {
			latest = info
			found = true
		}
	}
	return latest, found
}

// Pending approval mapping
func (h *BotHandler) savePendingApproval(userID int64, info pendingApproval) {
	h.approvalMu.Lock()
	defer h.approvalMu.Unlock()
	h.pendingApprove[userID] = info
}

func (h *BotHandler) popPendingApproval(userID int64) (pendingApproval, bool) {
	h.approvalMu.Lock()
	defer h.approvalMu.Unlock()
	info, ok := h.pendingApprove[userID]
	if ok {
		delete(h.pendingApprove, userID)
	}
	return info, ok
}

// Groupdagi xabarlarni qayta ishlash (AI yo'q)
func (h *BotHandler) handleGroupMessage(ctx context.Context, message *tgbotapi.Message) {
	log.Printf("[group_reply] chat=%d msg=%d replyTo=%d text=%q", message.Chat.ID, message.MessageID, replyID(message), truncateForLog(message.Text, 200))
	// Faqat group_1 dan reply bo'lsa yuboramiz
	if message.Chat == nil || message.Chat.ID != h.group1ChatID {
		log.Printf("[group_reply] skip: wrong chat chat=%d", message.Chat.ID)
		return
	}
	if message.ReplyToMessage == nil {
		log.Printf("[group_reply] skip: non-reply chat=%d msg=%d", message.Chat.ID, message.MessageID)
		return
	}

	targetInfo, ok := h.getGroupThread(message.ReplyToMessage.MessageID)
	if !ok {
		// Fallback: reply ichidagi OrderID bo'yicha aniqlash
		if oid := extractOrderIDFromText(message.ReplyToMessage.Text); oid != "" {
			if ord, found := h.getOrderStatus(oid); found {
				targetInfo = groupThreadInfo{
					UserID:    ord.UserID,
					UserChat:  ord.UserChat,
					Username:  ord.Username,
					Summary:   ord.Summary,
					Config:    ord.Config,
					OrderID:   ord.OrderID,
					ChatID:    h.group1ChatID,
					ThreadID:  h.group1ThreadID,
					CreatedAt: ord.CreatedAt,
				}
				h.saveGroupThread(message.ReplyToMessage.MessageID, targetInfo)
				ok = true
			}
		}
	}
	if !ok {
		log.Printf("[group_reply] skip: no mapping for reply msg=%d", message.ReplyToMessage.MessageID)
		return
	}
	// Thread cheklovi bo'lsa tekshiramiz
	if h.group1ThreadID > 0 && targetInfo.ThreadID != 0 && targetInfo.ThreadID != h.group1ThreadID {
		log.Printf("[group_reply] skip: thread mismatch (want=%d got=%d)", h.group1ThreadID, targetInfo.ThreadID)
		return
	}

	lang := h.getUserLang(targetInfo.UserID)
	adminMsg := fmt.Sprintf("%s\n%s",
		t(lang, "🔔 Admindan javob:", "🔔 Ответ админа:"),
		message.Text,
	)
	adminMsg = fmt.Sprintf("%s\n\n%s", adminMsg, t(lang, "Rasmiylashtiramizmi?", "Оформляем?"))

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Ha ✅", "Да ✅"), "order_yes"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Yo'q ❌", "Нет ❌"), "order_no"),
		),
	)

	msg := tgbotapi.NewMessage(targetInfo.UserChat, adminMsg)
	msg.ReplyMarkup = markup
	if _, err := h.sendAndLog(msg); err != nil {
		log.Printf("[group_reply] send to user failed chat=%d user=%d err=%v", targetInfo.UserChat, targetInfo.UserID, err)
		return
	}

	h.savePendingApproval(targetInfo.UserID, pendingApproval{
		UserID:   targetInfo.UserID,
		UserChat: targetInfo.UserChat,
		Summary:  targetInfo.Summary,
		Config:   targetInfo.Config,
		Username: targetInfo.Username,
		SentAt:   time.Now(),
	})
	h.clearGroup1PendingApproval(message.ReplyToMessage.MessageID)

	h.sendMessage(message.Chat.ID, t(lang, "📨 Foydalanuvchiga yuborildi.", "📨 Отправлено пользователю."))
}

func extractOrderIDFromText(text string) string {
	m := orderIDPattern.FindStringSubmatch(text)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}
