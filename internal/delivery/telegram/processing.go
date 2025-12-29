package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Processing guard: AI javobi tayyorlanayotganda parallel so'rovlarni to'xtatish
func (h *BotHandler) startProcessing(userID int64) bool {
	h.processingMu.Lock()
	defer h.processingMu.Unlock()
	if h.processing == nil {
		h.processing = make(map[int64]bool)
	}
	if h.processing[userID] {
		return false
	}
	h.processing[userID] = true
	return true
}

func (h *BotHandler) endProcessing(userID int64) {
	h.processingMu.Lock()
	delete(h.processing, userID)
	h.processingMu.Unlock()
}

func (h *BotHandler) isProcessing(userID int64) bool {
	h.processingMu.RLock()
	defer h.processingMu.RUnlock()
	return h.processing[userID]
}

// Processing warning helpers
func (h *BotHandler) incProcessingWarn(userID int64) int {
	h.warnMu.Lock()
	defer h.warnMu.Unlock()
	h.processingWarn[userID]++
	return h.processingWarn[userID]
}

func (h *BotHandler) resetProcessingWarn(userID int64) {
	h.warnMu.Lock()
	delete(h.processingWarn, userID)
	ids := h.warnMsgs[userID]
	delete(h.warnMsgs, userID)
	h.warnMu.Unlock()
	if h.bot == nil {
		return
	}
	for _, wm := range ids {
		del := tgbotapi.NewDeleteMessage(wm.ChatID, wm.MessageID)
		h.bot.Request(del)
	}
}

func (h *BotHandler) addWarnMessage(userID, chatID int64, msgID int) {
	h.warnMu.Lock()
	defer h.warnMu.Unlock()
	h.warnMsgs[userID] = append(h.warnMsgs[userID], waitingMessage{ChatID: chatID, MessageID: msgID})
}

// Waiting message helpers
func (h *BotHandler) setWaitingMessage(userID, chatID int64, msgID int) {
	h.waitingMu.Lock()
	defer h.waitingMu.Unlock()
	if h.waitingMsgs == nil {
		h.waitingMsgs = make(map[int64]waitingMessage)
	}
	h.waitingMsgs[userID] = waitingMessage{ChatID: chatID, MessageID: msgID}
}

func (h *BotHandler) clearWaitingMessage(userID int64) {
	h.waitingMu.Lock()
	msg, ok := h.waitingMsgs[userID]
	if ok {
		delete(h.waitingMsgs, userID)
	}
	h.waitingMu.Unlock()

	if ok {
		if h.bot == nil {
			return
		}
		del := tgbotapi.NewDeleteMessage(msg.ChatID, msg.MessageID)
		if _, err := h.bot.Request(del); err != nil {
			log.Printf("Waiting message delete failed: %v", err)
		}
	}
}

func (h *BotHandler) getWaitingMessage(userID int64) (waitingMessage, bool) {
	h.waitingMu.RLock()
	defer h.waitingMu.RUnlock()
	msg, ok := h.waitingMsgs[userID]
	return msg, ok
}
