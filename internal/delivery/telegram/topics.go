package telegram

// matchesTopic checks chat + thread equality, treating targetThreadID==0 as wildcard.
func (h *BotHandler) matchesTopic(chatID int64, threadID int, targetChatID int64, targetThreadID int) bool {
	if chatID == 0 || targetChatID == 0 {
		return false
	}
	if chatID != targetChatID {
		return false
	}
	if targetThreadID == 0 {
		return true
	}
	if threadID == 0 {
		return false
	}
	return threadID == targetThreadID
}

// isKnownTopic returns true if chat+thread belongs to any configured group topic.
func (h *BotHandler) isKnownTopic(chatID int64, threadID int) bool {
	return h.matchesTopic(chatID, threadID, h.group1ChatID, h.group1ThreadID) ||
		h.matchesTopic(chatID, threadID, h.group2ChatID, h.group2ThreadID) ||
		h.matchesTopic(chatID, threadID, h.group3ChatID, h.group3ThreadID) ||
		h.matchesTopic(chatID, threadID, h.group4ChatID, h.group4ThreadID)
}

// resolveThreadID picks a thread ID using message thread if present, otherwise provided default, else known mapping.
func (h *BotHandler) resolveThreadID(chatID int64, messageThreadID int, defaultThread int) int {
	if messageThreadID > 0 {
		return messageThreadID
	}
	if defaultThread > 0 {
		return defaultThread
	}
	return h.threadIDForChat(chatID)
}
