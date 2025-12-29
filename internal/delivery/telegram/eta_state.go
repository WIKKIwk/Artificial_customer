package telegram

import "fmt"

func etaChatKey(chatID int64, threadID int) string {
	return fmt.Sprintf("%d:%d", chatID, threadID)
}

func (h *BotHandler) setPendingETA(adminID int64, orderID string, chatID int64, threadID int) {
	h.pendingETAMu.Lock()
	defer h.pendingETAMu.Unlock()
	h.pendingETAs[adminID] = orderID
	if chatID != 0 {
		h.pendingETAChat[etaChatKey(chatID, threadID)] = orderID
	}
}

func (h *BotHandler) getPendingETA(adminID int64) (string, bool) {
	h.pendingETAMu.RLock()
	defer h.pendingETAMu.RUnlock()
	ord, ok := h.pendingETAs[adminID]
	return ord, ok
}

func (h *BotHandler) hasPendingETA(adminID, chatID int64, threadID int) bool {
	h.pendingETAMu.RLock()
	defer h.pendingETAMu.RUnlock()
	if _, ok := h.pendingETAs[adminID]; ok {
		return true
	}

	var keys []string
	if chatID != 0 {
		if threadID > 0 {
			keys = append(keys, etaChatKey(chatID, threadID))
		}
		if tid := h.threadIDForChat(chatID); tid > 0 {
			keys = append(keys, etaChatKey(chatID, tid))
		}
		if chatID == h.group1ChatID && h.group1ThreadID > 0 {
			keys = append(keys, etaChatKey(chatID, h.group1ThreadID))
		}
		if chatID == h.group2ChatID && h.group2ThreadID > 0 {
			keys = append(keys, etaChatKey(chatID, h.group2ThreadID))
		}
		if chatID == h.group4ChatID && h.group4ThreadID > 0 {
			keys = append(keys, etaChatKey(chatID, h.group4ThreadID))
		}
	}

	for _, k := range keys {
		if _, ok := h.pendingETAChat[k]; ok {
			return true
		}
	}
	return false
}

func (h *BotHandler) popPendingETA(adminID int64) (string, bool) {
	h.pendingETAMu.Lock()
	defer h.pendingETAMu.Unlock()
	ord, ok := h.pendingETAs[adminID]
	if ok {
		delete(h.pendingETAs, adminID)
	}
	return ord, ok
}

func (h *BotHandler) popPendingETAByChat(chatID int64, threadID int) (string, bool) {
	key := etaChatKey(chatID, threadID)
	h.pendingETAMu.Lock()
	defer h.pendingETAMu.Unlock()
	ord, ok := h.pendingETAChat[key]
	if ok {
		delete(h.pendingETAChat, key)
	}
	return ord, ok
}

func (h *BotHandler) clearETA(orderID string, adminID int64, chatID int64, threadID int) {
	h.pendingETAMu.Lock()
	defer h.pendingETAMu.Unlock()
	if adminID != 0 {
		delete(h.pendingETAs, adminID)
	}
	if chatID != 0 {
		delete(h.pendingETAChat, etaChatKey(chatID, threadID))
	}
}
