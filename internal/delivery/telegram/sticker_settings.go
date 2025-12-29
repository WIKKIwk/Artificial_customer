package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type stickerSlot string

const (
	stickerSlotLogin      stickerSlot = "login"
	stickerSlotOrderPlaced stickerSlot = "order_placed"
)

const stickerConfigFile = "data/sticker_config.json"

type stickerConfig struct {
	Enabled     *bool  `json:"enabled,omitempty"`
	Login       string `json:"login,omitempty"`
	OrderPlaced string `json:"order_placed,omitempty"`
}

func (cfg stickerConfig) isEnabled() bool {
	if cfg.Enabled == nil {
		return true
	}
	return *cfg.Enabled
}

func (h *BotHandler) loadStickerConfigFromDisk() {
	cfg, err := loadStickerConfigFile(stickerConfigFile)
	if err != nil {
		return
	}
	h.stickerMu.Lock()
	h.stickerCfg = &cfg
	h.stickerMu.Unlock()
}

func loadStickerConfigFile(path string) (stickerConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return stickerConfig{}, err
	}
	var cfg stickerConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return stickerConfig{}, err
	}
	cfg.Login = strings.TrimSpace(cfg.Login)
	cfg.OrderPlaced = strings.TrimSpace(cfg.OrderPlaced)
	return cfg, nil
}

func saveStickerConfigFile(path string, cfg stickerConfig) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func (h *BotHandler) getStickerConfig() stickerConfig {
	h.stickerMu.RLock()
	cfgPtr := h.stickerCfg
	h.stickerMu.RUnlock()
	if cfgPtr != nil {
		return *cfgPtr
	}
	return stickerConfig{}
}

func (h *BotHandler) setStickerEnabled(enabled bool) error {
	cfg := h.getStickerConfig()
	cfg.Enabled = &enabled
	if err := saveStickerConfigFile(stickerConfigFile, cfg); err != nil {
		return err
	}
	h.stickerMu.Lock()
	h.stickerCfg = &cfg
	h.stickerMu.Unlock()
	return nil
}

func (h *BotHandler) clearStickerForSlot(slot stickerSlot) error {
	cfg := h.getStickerConfig()
	switch slot {
	case stickerSlotLogin:
		cfg.Login = ""
	case stickerSlotOrderPlaced:
		cfg.OrderPlaced = ""
	default:
		return fmt.Errorf("unknown sticker slot: %s", slot)
	}
	if err := saveStickerConfigFile(stickerConfigFile, cfg); err != nil {
		return err
	}
	h.stickerMu.Lock()
	h.stickerCfg = &cfg
	h.stickerMu.Unlock()
	return nil
}

func (h *BotHandler) setStickerForSlot(slot stickerSlot, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return fmt.Errorf("empty sticker file id")
	}

	cfg := h.getStickerConfig()
	switch slot {
	case stickerSlotLogin:
		cfg.Login = fileID
	case stickerSlotOrderPlaced:
		cfg.OrderPlaced = fileID
	default:
		return fmt.Errorf("unknown sticker slot: %s", slot)
	}

	if err := saveStickerConfigFile(stickerConfigFile, cfg); err != nil {
		return err
	}
	h.stickerMu.Lock()
	h.stickerCfg = &cfg
	h.stickerMu.Unlock()
	return nil
}

func (h *BotHandler) getStickerFileID(slot stickerSlot) string {
	cfg := h.getStickerConfig()
	switch slot {
	case stickerSlotLogin:
		return strings.TrimSpace(cfg.Login)
	case stickerSlotOrderPlaced:
		return strings.TrimSpace(cfg.OrderPlaced)
	default:
		return ""
	}
}

func (h *BotHandler) sendStickerIfConfigured(chatID int64, slot stickerSlot) {
	if h == nil || h.bot == nil || chatID == 0 {
		return
	}
	if !h.getStickerConfig().isEnabled() {
		return
	}
	fileID := h.getStickerFileID(slot)
	if fileID == "" {
		return
	}
	msg := tgbotapi.NewSticker(chatID, tgbotapi.FileID(fileID))
	if _, err := h.sendAndLog(msg); err != nil {
		log.Printf("send sticker failed slot=%s chat=%d err=%v", slot, chatID, err)
	}
}

func (h *BotHandler) setStickerAwait(adminID int64, slot stickerSlot) {
	h.stickerMu.Lock()
	if slot == "" {
		delete(h.stickerAwait, adminID)
		h.stickerMu.Unlock()
		return
	}
	h.stickerAwait[adminID] = slot
	h.stickerMu.Unlock()
}

func (h *BotHandler) getStickerAwait(adminID int64) (stickerSlot, bool) {
	h.stickerMu.RLock()
	slot, ok := h.stickerAwait[adminID]
	h.stickerMu.RUnlock()
	return slot, ok
}

func (h *BotHandler) clearStickerAwait(adminID int64) {
	h.setStickerAwait(adminID, "")
}

func stickerSlotLabel(lang string, slot stickerSlot) string {
	switch slot {
	case stickerSlotLogin:
		return t(lang, "login (kirish)", "вход")
	case stickerSlotOrderPlaced:
		return t(lang, "buyurtma rasmiylashtirilganda", "оформление заказа")
	default:
		return string(slot)
	}
}

func stickerStatusMark(fileID string) string {
	if strings.TrimSpace(fileID) == "" {
		return "—"
	}
	return "✅"
}

func stickerEnabledMark(enabled bool) string {
	if enabled {
		return "🟢 ON"
	}
	return "🔴 OFF"
}

func (h *BotHandler) buildStickerMenu(lang string) (string, tgbotapi.InlineKeyboardMarkup) {
	cfg := h.getStickerConfig()

	text := fmt.Sprintf(
		"%s\n\n🔔 %s: %s\n👋 %s: %s\n🧾 %s: %s\n\n%s",
		t(lang, "🧩 Sticker sozlamalari", "🧩 Настройки стикеров"),
		t(lang, "Holat", "Статус"),
		stickerEnabledMark(cfg.isEnabled()),
		t(lang, "Login", "Вход"),
		stickerStatusMark(cfg.Login),
		t(lang, "Buyurtma", "Заказ"),
		stickerStatusMark(cfg.OrderPlaced),
		t(lang, "Nimani sozlamoqchisiz?", "Что настроить?"),
	)

	toggleText := t(lang, "🔴 O‘chirish", "🔴 Выключить")
	toggleData := "sticker_enabled|0"
	if !cfg.isEnabled() {
		toggleText = t(lang, "🟢 Yoqish", "🟢 Включить")
		toggleData = "sticker_enabled|1"
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggleText, toggleData),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "👋 Login sticker", "👋 Стикер входа"), "sticker_set|login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🧾 Buyurtma sticker", "🧾 Стикер заказа"), "sticker_set|order_placed"),
		),
	)
	return text, kb
}

func (h *BotHandler) buildStickerSlotPrompt(lang string, slot stickerSlot) (string, *tgbotapi.InlineKeyboardMarkup) {
	fileID := h.getStickerFileID(slot)

	text := fmt.Sprintf(
		"%s: %s\n%s: %s\n\n%s\n\n%s",
		t(lang, "Tanlandi", "Выбрано"),
		stickerSlotLabel(lang, slot),
		t(lang, "Hozirgi", "Текущий"),
		stickerStatusMark(fileID),
		t(lang, "Endi qaysi sticker tashlashni istasangiz shu sticker'ni shu yerga yuboring.", "Теперь отправьте сюда стикер, который нужно отправлять."),
		t(lang, "Bekor qilish: /cancel", "Отмена: /cancel"),
	)

	if strings.TrimSpace(fileID) == "" {
		return text, nil
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🚫 Sticker OFF", "🚫 Стикер OFF"), "sticker_clear|"+string(slot)),
		),
	)
	return text, &kb
}

func (h *BotHandler) handleStickerCommand(ctx context.Context, message *tgbotapi.Message) {
	if message == nil || message.From == nil {
		return
	}
	adminID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.deleteCommandMessage(message)
	lang := h.getUserLang(adminID)
	text, kb := h.buildStickerMenu(lang)
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(message.Chat.ID, sent.MessageID)
	}
}

func (h *BotHandler) handleStickerEnabledCallback(ctx context.Context, chatID int64, adminID int64, enabled bool, srcMsg *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}

	if err := h.setStickerEnabled(enabled); err != nil {
		h.sendMessage(chatID, "❌ Saqlashda xatolik. Qayta urinib ko'ring.")
		return
	}

	lang := h.getUserLang(adminID)
	text, kb := h.buildStickerMenu(lang)
	if srcMsg != nil && srcMsg.MessageID != 0 {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, srcMsg.MessageID, text, kb)
		if _, err := h.bot.Send(edit); err != nil {
			h.sendMessage(chatID, t(lang, "✅ Saqlandi.", "✅ Сохранено."))
		}
		return
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(chatID, sent.MessageID)
	}
}

func (h *BotHandler) handleStickerSelectCallback(ctx context.Context, chatID int64, adminID int64, slot stickerSlot) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}
	if slot != stickerSlotLogin && slot != stickerSlotOrderPlaced {
		h.sendMessage(chatID, "❌ Noto'g'ri tanlov.")
		return
	}

	h.setStickerAwait(adminID, slot)
	lang := h.getUserLang(adminID)
	text, kb := h.buildStickerSlotPrompt(lang, slot)
	msg := tgbotapi.NewMessage(chatID, text)
	if kb != nil {
		msg.ReplyMarkup = *kb
	}
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(chatID, sent.MessageID)
	}
}

func (h *BotHandler) handleStickerClearCallback(ctx context.Context, chatID int64, adminID int64, slot stickerSlot, srcMsg *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}
	if slot != stickerSlotLogin && slot != stickerSlotOrderPlaced {
		h.sendMessage(chatID, "❌ Noto'g'ri tanlov.")
		return
	}

	if err := h.clearStickerForSlot(slot); err != nil {
		h.sendMessage(chatID, "❌ Saqlashda xatolik. Qayta urinib ko'ring.")
		return
	}

	h.setStickerAwait(adminID, slot)
	lang := h.getUserLang(adminID)
	baseText, kb := h.buildStickerSlotPrompt(lang, slot)
	text := fmt.Sprintf("%s\n\n%s", t(lang, "✅ Sticker o‘chirildi.", "✅ Стикер выключен."), baseText)

	if srcMsg != nil && srcMsg.MessageID != 0 {
		markup := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
		if kb != nil {
			markup = *kb
		}
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, srcMsg.MessageID, text, markup)
		if _, err := h.bot.Send(edit); err != nil {
			h.sendMessage(chatID, t(lang, "✅ Sticker o‘chirildi.", "✅ Стикер выключен."))
		}
		return
	}

	msg := tgbotapi.NewMessage(chatID, text)
	if kb != nil {
		msg.ReplyMarkup = *kb
	}
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(chatID, sent.MessageID)
	}
}

// handleStickerSetupInput consumes messages while admin is in /sticker setup mode.
func (h *BotHandler) handleStickerSetupInput(ctx context.Context, message *tgbotapi.Message) bool {
	if message == nil || message.From == nil || message.Chat == nil {
		return false
	}
	adminID := message.From.ID

	slot, ok := h.getStickerAwait(adminID)
	if !ok {
		return false
	}

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.clearStickerAwait(adminID)
		return false
	}

	lang := h.getUserLang(adminID)

	if message.IsCommand() {
		cmd := extractCommand(message)
		if cmd == "cancel" {
			h.clearStickerAwait(adminID)
			h.deleteCommandMessage(message)
			h.sendMessage(message.Chat.ID, t(lang, "❌ Bekor qilindi.", "❌ Отмена."))
			return true
		}
		// Any other command cancels sticker setup and continues as usual.
		h.clearStickerAwait(adminID)
		return false
	}

	if txt := strings.TrimSpace(message.Text); txt != "" {
		lower := strings.ToLower(txt)
		if lower == "cancel" || lower == "bekor" || lower == "otmena" {
			h.clearStickerAwait(adminID)
			h.deleteUserMessage(message.Chat.ID, message)
			h.sendMessage(message.Chat.ID, t(lang, "❌ Bekor qilindi.", "❌ Отмена."))
			return true
		}
	}

	if message.Sticker == nil {
		h.sendMessage(message.Chat.ID, fmt.Sprintf(
			t(lang, "Iltimos, %s uchun sticker yuboring.\n\nBekor qilish: /cancel", "Пожалуйста, отправьте стикер для события: %s.\n\nОтмена: /cancel"),
			stickerSlotLabel(lang, slot),
		))
		return true
	}

	fileID := strings.TrimSpace(message.Sticker.FileID)
	if fileID == "" {
		h.sendMessage(message.Chat.ID, t(lang, "❌ Sticker file_id topilmadi. Qayta yuboring.", "❌ Не удалось получить file_id стикера. Отправьте ещё раз."))
		return true
	}

	if err := h.setStickerForSlot(slot, fileID); err != nil {
		h.sendMessage(message.Chat.ID, t(lang, "❌ Saqlashda xatolik. Qayta urinib ko'ring.", "❌ Ошибка сохранения. Попробуйте ещё раз."))
		return true
	}

	h.clearStickerAwait(adminID)
	h.deleteUserMessage(message.Chat.ID, message)
	h.sendMessage(message.Chat.ID, t(lang, "✅ Sticker saqlandi.", "✅ Стикер сохранён."))
	return true
}
