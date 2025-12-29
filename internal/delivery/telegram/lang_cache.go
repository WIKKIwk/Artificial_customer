package telegram

// Language helpers
func (h *BotHandler) setUserLang(userID int64, lang string) {
	h.langMu.Lock()
	defer h.langMu.Unlock()
	if lang != "ru" {
		lang = "uz"
	}
	h.userLang[userID] = lang
}

func (h *BotHandler) getUserLang(userID int64) string {
	h.langMu.RLock()
	defer h.langMu.RUnlock()
	if lang, ok := h.userLang[userID]; ok {
		return lang
	}
	return "uz"
}

// Cache helper methods for worker pool
func (h *BotHandler) getCacheKey(userID int64, text string) string {
	return generateCacheKey(userID, text)
}

func (h *BotHandler) getCachedResponse(key string) (string, bool) {
	return h.cache.get(key)
}

func (h *BotHandler) cacheResponse(key, response string) {
	h.cache.set(key, response)
}

func (h *BotHandler) getCacheStats() (hits, misses int64, size int) {
	return h.cache.stats()
}
