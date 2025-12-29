package telegram

import (
	"strings"
)

func (h *BotHandler) setProfileStage(userID int64, stage string) {
	h.profileMu.Lock()
	defer h.profileMu.Unlock()
	if stage == "" {
		delete(h.profileStage, userID)
		return
	}
	h.profileStage[userID] = stage
}

func (h *BotHandler) getProfileStage(userID int64) string {
	h.profileMu.RLock()
	defer h.profileMu.RUnlock()
	return h.profileStage[userID]
}

func (h *BotHandler) setProfile(userID int64, upd userProfile) {
	h.profileMu.Lock()
	prof := h.profiles[userID]
	changed := false
	if strings.TrimSpace(upd.Name) != "" {
		name := strings.TrimSpace(upd.Name)
		if name != prof.Name {
			prof.Name = name
			changed = true
		}
	}
	if strings.TrimSpace(upd.Phone) != "" {
		phone := strings.TrimSpace(upd.Phone)
		if phone != prof.Phone {
			prof.Phone = phone
			changed = true
		}
	}
	h.profiles[userID] = prof
	h.profileMu.Unlock()
	if changed {
		h.scheduleAboutUserSheetSync("profile")
	}
}

func (h *BotHandler) getProfile(userID int64) (userProfile, bool) {
	h.profileMu.RLock()
	defer h.profileMu.RUnlock()
	prof, ok := h.profiles[userID]
	return prof, ok
}

// helper to copy profile without data race
func cloneProfile(p userProfile) userProfile {
	return userProfile{
		Name:  p.Name,
		Phone: p.Phone,
	}
}

func (h *BotHandler) setProfilePrompt(userID int64, msgID int) {
	h.profileMu.Lock()
	defer h.profileMu.Unlock()
	meta := h.profileMeta[userID]
	meta.PromptMsgID = msgID
	h.profileMeta[userID] = meta
}

func (h *BotHandler) getProfilePrompt(userID int64) int {
	h.profileMu.RLock()
	defer h.profileMu.RUnlock()
	return h.profileMeta[userID].PromptMsgID
}

func (h *BotHandler) clearProfilePrompt(userID int64) {
	h.profileMu.Lock()
	defer h.profileMu.Unlock()
	delete(h.profileMeta, userID)
}

// Profilni to'ldirishga chaqirish
func (h *BotHandler) maybeAskProfile(userID, chatID int64, lang string) {
	prof, ok := h.getProfile(userID)
	if !ok || strings.TrimSpace(prof.Name) == "" {
		h.setProfileStage(userID, "need_name")
		if sent, err := h.sendMessageWithResp(chatID, t(lang, "Ismingizni yozing.", "Напишите ваше имя.")); err == nil {
			h.setProfilePrompt(userID, sent.MessageID)
		}
		return
	}
	if strings.TrimSpace(prof.Phone) == "" {
		h.setProfileStage(userID, "need_phone")
		npid := h.sendPhonePrompt(chatID, lang)
		if npid != 0 {
			h.setProfilePrompt(userID, npid)
		}
	}
}

// Shaxsiy salomlashuv
func (h *BotHandler) sendGreeting(chatID int64, lang, name string) {
	// Welcome message'ni yuborish
	h.cleanupWelcomeMessages(chatID)
	text := h.getWelcomeMessage(lang, name)
	h.sendMessage(chatID, text)
	h.sendStickerIfConfigured(chatID, stickerSlotLogin)
}
