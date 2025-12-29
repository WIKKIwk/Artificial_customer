package telegram

func (h *BotHandler) setConfigOrderLocked(userID int64, locked bool) {
	if userID == 0 {
		return
	}
	h.configOrderMu.Lock()
	if locked {
		h.configOrderLocked[userID] = true
	} else {
		delete(h.configOrderLocked, userID)
	}
	h.configOrderMu.Unlock()
}

func (h *BotHandler) isConfigOrderLocked(userID int64) bool {
	h.configOrderMu.RLock()
	_, ok := h.configOrderLocked[userID]
	h.configOrderMu.RUnlock()
	return ok
}
