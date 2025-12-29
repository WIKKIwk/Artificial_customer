package telegram

import (
	"context"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Start botni ishga tushirish
func (h *BotHandler) Start(ctx context.Context) error {
	h.workerPool.start(ctx)
	go h.cleanupSessions(ctx)
	go h.cache.cleanup(ctx)
	go h.ensureAboutUserSheetOnStart(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			h.workerPool.shutdown()
			return ctx.Err()
		case update := <-updates:
			if update.InlineQuery != nil {
				go h.handleInlineQuery(ctx, update.InlineQuery)
				continue
			}
			if update.ChosenInlineResult != nil {
				go h.handleChosenInlineResult(ctx, update.ChosenInlineResult)
				continue
			}
			if update.CallbackQuery != nil {
				go h.handleCallback(ctx, update.CallbackQuery)
				continue
			}
			if update.Message == nil {
				continue
			}
			go h.handleMessage(ctx, update.Message)
		}
	}
}

// handleMessage xabarni qayta ishlash
func (h *BotHandler) handleMessage(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	username := message.From.UserName
	if username == "" {
		username = message.From.FirstName
	}
	isNewUser := h.touchLastSeen(userID, username)
	if isNewUser && message.Chat != nil && message.Chat.IsPrivate() {
		h.scheduleAboutUserSheetSync("new_user")
	}
	if message.Chat != nil && message.Chat.IsPrivate() {
		h.logIncomingChatMessage(message)
	}
	if h.group1ChatID != 0 {
		h.maybeSendLiveFeed(userID, username, message.Chat.ID)
	}

	if message.ViaBot != nil && message.ViaBot.ID == h.bot.Self.ID {
		if h.handleAddProductInlineSelectionMessage(ctx, message) {
			return
		}
		if h.handleUserChatHistoryInlineSelectionMessage(ctx, message) {
			return
		}
		if _, ok := h.getAddProductState(userID); ok {
			return
		}
		if h.isAwaitingUserHistory(userID) {
			return
		}
	}

	if message.Document != nil {
		h.handleDocumentMessage(ctx, message)
		return
	}
	if h.isAwaitingPassword(userID) {
		h.handlePasswordInput(ctx, message)
		return
	}
	if message.IsCommand() || strings.HasPrefix(strings.TrimSpace(message.Text), "/") {
		h.handleCommand(ctx, message)
		return
	}

	if h.isAdminActive(userID) && message.MessageID != 0 {
		h.addAdminMessage(userID, message.Chat.ID, message.MessageID)
	}
	if message.MessageID != 0 {
		h.markUserMessage(message.Chat.ID, message.MessageID)
	}

	// Admin group reply mappingini faqat group_1 dagi replylar uchun tekshiramiz
	if message.Chat != nil && (message.Chat.IsGroup() || message.Chat.IsSuperGroup()) &&
		message.Chat.ID == h.group1ChatID && message.ReplyToMessage != nil {
		if info, ok := h.getGroupThread(message.ReplyToMessage.MessageID); ok {
			if h.group1ThreadID == 0 || info.ThreadID == 0 || info.ThreadID == h.group1ThreadID {
				log.Printf("[router] group reply detected chat=%d msg=%d replyTo=%d user=%d", message.Chat.ID, message.MessageID, message.ReplyToMessage.MessageID, info.UserID)
				h.handleGroupMessage(ctx, message)
				return
			}
		}
	}

	if message.Chat != nil && (message.Chat.IsGroup() || message.Chat.IsSuperGroup()) {
		// ETA uchun alohida tekshiruv
		if h.activeOrdersChatID != 0 && message.Chat.ID == h.activeOrdersChatID {
			if h.hasPendingETA(userID, message.Chat.ID, 0) {
				h.handleGroup2Message(ctx, message)
				return
			}
		} else if h.hasPendingETA(userID, message.Chat.ID, 0) {
			h.handleGroup2Message(ctx, message)
			return
		}

		// Faqat group_1 dagi mappingi bo'lgan reply-lar orqali foydalanuvchiga javob beramiz
		if message.Chat != nil && message.Chat.ID == h.group1ChatID && message.ReplyToMessage != nil {
			if info, ok := h.getGroupThread(message.ReplyToMessage.MessageID); ok {
				if h.group1ThreadID == 0 || info.ThreadID == 0 || info.ThreadID == h.group1ThreadID {
					h.handleGroupMessage(ctx, message)
				}
			}
		}
		return
	}

	if message.Text != "" || message.Contact != nil || message.Location != nil {
		h.handleTextMessage(ctx, userID, username, message.Text, message.Chat.ID, message)
	}
}

func (h *BotHandler) touchLastSeen(userID int64, username string) bool {
	now := time.Now()
	h.lastSeenMu.Lock()
	_, existed := h.lastSeen[userID]
	h.lastSeen[userID] = now
	h.lastSeenMu.Unlock()

	if trim := strings.TrimSpace(username); trim != "" {
		h.nameMu.Lock()
		h.lastName[userID] = trim
		h.nameMu.Unlock()
	}
	return !existed
}

func (h *BotHandler) maybeSendLiveFeed(userID int64, username string, chatID int64) {
	now := time.Now()
	h.liveFeedMu.Lock()
	h.liveFeedSent[userID] = now
	h.liveFeedMu.Unlock()
}
