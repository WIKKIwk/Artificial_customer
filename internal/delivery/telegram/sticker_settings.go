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
	Login       string `json:"login,omitempty"`
	OrderPlaced string `json:"order_placed,omitempty"`
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
	if cfg.Login == "" && cfg.OrderPlaced == "" {
		return stickerConfig{}, fmt.Errorf("empty config")
	}
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

	cfg := h.getStickerConfig()
	text := fmt.Sprintf(
		"%s\n\n👋 %s: %s\n🧾 %s: %s\n\n%s",
		t(lang, "🧩 Sticker sozlamalari", "🧩 Настройки стикеров"),
		t(lang, "Login", "Вход"),
		stickerStatusMark(cfg.Login),
		t(lang, "Buyurtma", "Заказ"),
		stickerStatusMark(cfg.OrderPlaced),
		t(lang, "Qaysi holat uchun sticker belgilaysiz?", "Для какого события задать стикер?"),
	)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "👋 Login", "👋 Вход"), "sticker_set|login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🧾 Buyurtma rasmiylashtirildi", "🧾 Заказ оформлен"), "sticker_set|order_placed"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(message.Chat.ID, sent.MessageID)
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
	h.sendMessage(chatID, fmt.Sprintf(
		t(lang, "Tanlandi: %s.\nEndi qaysi sticker tashlashni istasangiz shu sticker'ni shu yerga yuboring.\n\nBekor qilish: /cancel", "Выбрано: %s.\nТеперь отправьте сюда стикер, который нужно отправлять.\n\nОтмена: /cancel"),
		stickerSlotLabel(lang, slot),
	))
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

