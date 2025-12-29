package telegram

// Parol kutish holatini boshqarish
func (h *BotHandler) isAwaitingPassword(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.awaitingPassword[userID]
}

func (h *BotHandler) setAwaitingPassword(userID int64, awaiting bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if awaiting {
		h.awaitingPassword[userID] = true
	} else {
		delete(h.awaitingPassword, userID)
	}
}
