package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type sheetMasterSetupStage int

const (
	sheetMasterStageNeedBaseURL sheetMasterSetupStage = iota
	sheetMasterStageNeedAPIKey
	sheetMasterStageNeedFileID
)

type sheetMasterSetupState struct {
	stage         sheetMasterSetupStage
	chatID        int64
	pendingAction string // "", "sync", "status"
	cfg           sheetMasterConfig
}

type sheetMasterFilesResponse struct {
	Files []sheetMasterFileMeta `json:"files"`
}

type sheetMasterFileMeta struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	UpdatedAt  time.Time `json:"updated_at"`
	OwnerID    uint      `json:"owner_id"`
	AccessRole string    `json:"access_role"`
}

const sheetMasterConfigFile = "data/sheetmaster_config.json"

func (h *BotHandler) loadSheetMasterConfigFromDisk() {
	cfg, err := loadSheetMasterConfigFile(sheetMasterConfigFile)
	if err != nil {
		return
	}
	h.sheetMasterMu.Lock()
	h.sheetMasterCfg = &cfg
	h.sheetMasterMu.Unlock()
}

func loadSheetMasterConfigFile(path string) (sheetMasterConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return sheetMasterConfig{}, err
	}
	var cfg sheetMasterConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return sheetMasterConfig{}, err
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.FileID = strings.TrimSpace(cfg.FileID)
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.FileID == "" {
		return sheetMasterConfig{}, fmt.Errorf("incomplete config")
	}
	return cfg, nil
}

func saveSheetMasterConfigFile(path string, cfg sheetMasterConfig) error {
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

func (h *BotHandler) getSheetMasterConfig() (sheetMasterConfig, bool) {
	h.sheetMasterMu.RLock()
	cfgPtr := h.sheetMasterCfg
	h.sheetMasterMu.RUnlock()
	if cfgPtr != nil && cfgPtr.BaseURL != "" && cfgPtr.APIKey != "" && cfgPtr.FileID != "" {
		return *cfgPtr, true
	}
	return sheetMasterConfig{}, false
}

func (h *BotHandler) setSheetMasterConfig(cfg sheetMasterConfig) error {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.FileID = strings.TrimSpace(cfg.FileID)
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.FileID == "" {
		return fmt.Errorf("incomplete config")
	}
	if err := saveSheetMasterConfigFile(sheetMasterConfigFile, cfg); err != nil {
		return err
	}
	h.sheetMasterMu.Lock()
	h.sheetMasterCfg = &cfg
	h.sheetMasterMu.Unlock()
	return nil
}

func (h *BotHandler) clearSheetMasterConfig() {
	_ = os.Remove(sheetMasterConfigFile)
	h.sheetMasterMu.Lock()
	h.sheetMasterCfg = nil
	h.sheetMasterMu.Unlock()
}

func (h *BotHandler) resolveSheetMasterConfig() (sheetMasterConfig, error) {
	if cfg, ok := h.getSheetMasterConfig(); ok {
		return cfg, nil
	}
	// Fallback: env
	if cfg, err := sheetMasterConfigFromEnv(); err == nil {
		return cfg, nil
	}
	return sheetMasterConfig{}, errSheetMasterNotConfigured
}

func (h *BotHandler) beginSheetMasterSetup(userID, chatID int64, pendingAction string) {
	h.sheetMasterSetupMu.Lock()
	h.sheetMasterSetup[userID] = &sheetMasterSetupState{
		stage:         sheetMasterStageNeedBaseURL,
		chatID:        chatID,
		pendingAction: pendingAction,
		cfg: sheetMasterConfig{
			BaseURL: "http://backend-go:8080",
		},
	}
	h.sheetMasterSetupMu.Unlock()

	h.sendMessage(chatID, "🗄️ Database (SheetMaster) sozlanmagan.\n\n1) API base URL ni yuboring.\nDefault: http://backend-go:8080\n\nDefault uchun `-` yuboring.\nBekor qilish: /db_cancel")
}

func (h *BotHandler) getSheetMasterSetupState(userID int64) (*sheetMasterSetupState, bool) {
	h.sheetMasterSetupMu.RLock()
	st, ok := h.sheetMasterSetup[userID]
	h.sheetMasterSetupMu.RUnlock()
	return st, ok
}

func (h *BotHandler) setSheetMasterSetupState(userID int64, st *sheetMasterSetupState) {
	h.sheetMasterSetupMu.Lock()
	h.sheetMasterSetup[userID] = st
	h.sheetMasterSetupMu.Unlock()
}

func (h *BotHandler) clearSheetMasterSetupState(userID int64) {
	h.sheetMasterSetupMu.Lock()
	delete(h.sheetMasterSetup, userID)
	h.sheetMasterSetupMu.Unlock()
}

func (h *BotHandler) handleDBSetCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}
	if message.Chat == nil || !message.Chat.IsPrivate() {
		h.sendMessage(message.Chat.ID, "🔒 Xavfsizlik uchun bu sozlashni faqat bot bilan privat chatda qiling.")
		return
	}
	h.beginSheetMasterSetup(userID, message.Chat.ID, "")
}

func (h *BotHandler) handleDBCancelCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}
	h.clearSheetMasterSetupState(userID)
	h.sendMessage(message.Chat.ID, "❌ Database setup bekor qilindi.")
}

func (h *BotHandler) handleDBClearCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}
	h.clearSheetMasterConfig()
	h.sendMessage(message.Chat.ID, "✅ Database (SheetMaster) sozlamalari tozalandi.")
}

func (h *BotHandler) handleDBFilesCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	cfg, err := h.resolveSheetMasterConfig()
	if err != nil {
		if message.Chat != nil && message.Chat.IsPrivate() {
			h.beginSheetMasterSetup(userID, message.Chat.ID, "files")
			return
		}
		h.sendMessage(message.Chat.ID, "❌ Database sozlanmagan. Privat chatda /db_set qiling.")
		return
	}

	files, err := sheetMasterListFiles(ctx, cfg)
	if err != nil {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Files olish xatosi: %v", err))
		return
	}

	h.sendMessage(message.Chat.ID, formatSheetMasterFilesList(files))
}

func sheetMasterListFiles(ctx context.Context, cfg sheetMasterConfig) ([]sheetMasterFileMeta, error) {
	client := sheetMasterHTTPClient()
	var out sheetMasterFilesResponse
	u := cfg.BaseURL + "/api/v1/files?limit=20"
	if err := sheetMasterDoJSON(ctx, client, http.MethodGet, u, cfg.APIKey, &out); err != nil {
		return nil, err
	}
	return out.Files, nil
}

func formatSheetMasterFilesList(files []sheetMasterFileMeta) string {
	if len(files) == 0 {
		return "📂 Database fayllari topilmadi."
	}
	var sb strings.Builder
	sb.WriteString("📂 Database fayllari (oxirgi 20 ta):\n\n")
	for _, f := range files {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = "(no name)"
		}
		sb.WriteString(fmt.Sprintf("• %d — %s\n", f.ID, name))
	}
	sb.WriteString("\nFayl ID ni tanlab /db_set orqali kiriting yoki /import_now qiling.")
	return sb.String()
}

func (h *BotHandler) handleSheetMasterSetupInput(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil || msg.Chat == nil {
		return false
	}
	userID := msg.From.ID
	st, ok := h.getSheetMasterSetupState(userID)
	if !ok {
		return false
	}

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.clearSheetMasterSetupState(userID)
		return false
	}
	if !msg.Chat.IsPrivate() {
		h.sendMessage(msg.Chat.ID, "🔒 Xavfsizlik uchun bu sozlashni faqat privat chatda qiling.")
		return true
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		h.sendMessage(msg.Chat.ID, "Iltimos, bo'sh bo'lmagan matn yuboring yoki /db_cancel bosing.")
		return true
	}

	switch strings.ToLower(text) {
	case "/db_cancel", "/cancel", "cancel", "stop", "bekor":
		h.clearSheetMasterSetupState(userID)
		h.sendMessage(msg.Chat.ID, "❌ Database setup bekor qilindi.")
		return true
	}

	switch st.stage {
	case sheetMasterStageNeedBaseURL:
		if text == "-" {
			text = st.cfg.BaseURL
		}
		text = strings.TrimRight(strings.TrimSpace(text), "/")
		if !strings.HasPrefix(text, "http://") && !strings.HasPrefix(text, "https://") {
			h.sendMessage(msg.Chat.ID, "❌ Base URL noto'g'ri. Misol: http://backend-go:8080 yoki http://localhost:8080\nDefault uchun '-' yuboring.")
			return true
		}
		st.cfg.BaseURL = text
		st.stage = sheetMasterStageNeedAPIKey
		h.setSheetMasterSetupState(userID, st)
		h.sendMessage(msg.Chat.ID, "✅ Base URL saqlandi.\n\n2) Endi API key ni yuboring (sk_...).\n\nBekor qilish: /db_cancel")
		return true

	case sheetMasterStageNeedAPIKey:
		key := strings.TrimSpace(text)
		if !strings.HasPrefix(key, "sk_") || len(key) < 10 {
			h.sendMessage(msg.Chat.ID, "❌ API key noto'g'ri ko'rinyapti. Misol: sk_... (Dev bo'limidan oling)")
			return true
		}
		st.cfg.APIKey = key

		// Fetch files list to help choose file ID
		files, err := sheetMasterListFiles(ctx, st.cfg)
		if err != nil {
			h.sendMessage(msg.Chat.ID, fmt.Sprintf("⚠️ API key saqlandi, lekin fayllarni olish xatosi: %v\n\n3) Fayl ID ni qo'lda yuboring (raqam).", err))
		} else {
			h.sendMessage(msg.Chat.ID, formatSheetMasterFilesList(files))
			h.sendMessage(msg.Chat.ID, "3) Endi katalog fayl ID ni yuboring (raqam).")
		}

		st.stage = sheetMasterStageNeedFileID
		h.setSheetMasterSetupState(userID, st)
		return true

	case sheetMasterStageNeedFileID:
		idStr := strings.TrimSpace(text)
		idStr = strings.TrimPrefix(strings.ToLower(idStr), "file_id=")
		idStr = strings.Trim(idStr, "`\"'")
		if idStr == "" {
			h.sendMessage(msg.Chat.ID, "❌ Fayl ID bo'sh. Misol: 1")
			return true
		}
		if _, err := strconv.ParseUint(idStr, 10, 64); err != nil {
			h.sendMessage(msg.Chat.ID, "❌ Fayl ID raqam bo'lishi kerak. Misol: 1")
			return true
		}
		st.cfg.FileID = idStr

		// Save config
		if err := h.setSheetMasterConfig(st.cfg); err != nil {
			h.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Sozlamalarni saqlashda xatolik: %v", err))
			return true
		}

		pending := st.pendingAction
		h.clearSheetMasterSetupState(userID)

		h.sendMessage(msg.Chat.ID, "✅ Database (SheetMaster) sozlandi!")

		// Continue pending action by asking admin to retry the command.
		switch pending {
		case "sync":
			h.sendMessage(msg.Chat.ID, "Endi /db_sync yoki /import_now ni qayta bosing.")
		case "status":
			h.sendMessage(msg.Chat.ID, "Endi /db_status ni qayta bosing.")
		case "files":
			h.sendMessage(msg.Chat.ID, "Endi /db_files ni qayta bosing.")
		}
		return true
	default:
		h.clearSheetMasterSetupState(userID)
		return false
	}
}

// ensureSheetMasterConfig returns cfg if available; if not, starts setup wizard and returns false.
func (h *BotHandler) ensureSheetMasterConfig(ctx context.Context, message *tgbotapi.Message, pendingAction string) (sheetMasterConfig, bool) {
	if cfg, err := h.resolveSheetMasterConfig(); err == nil {
		return cfg, true
	}

	if message == nil || message.Chat == nil {
		return sheetMasterConfig{}, false
	}
	if !message.Chat.IsPrivate() {
		h.sendMessage(message.Chat.ID, "❌ Database sozlanmagan. Xavfsizlik uchun privat chatda /admin qilib, /import_now yoki /db_set qiling.")
		return sheetMasterConfig{}, false
	}
	userID := message.From.ID
	h.beginSheetMasterSetup(userID, message.Chat.ID, pendingAction)
	return sheetMasterConfig{}, false
}

var errSheetMasterNotConfigured = errors.New("sheetmaster not configured")

func (h *BotHandler) sheetMasterConfigured() bool {
	_, ok := h.getSheetMasterConfig()
	if ok {
		return true
	}
	_, err := sheetMasterConfigFromEnv()
	return err == nil
}

func (h *BotHandler) safeSheetMasterKeyHint(cfg sheetMasterConfig) string {
	k := strings.TrimSpace(cfg.APIKey)
	if k == "" {
		return ""
	}
	if len(k) <= 6 {
		return "sk_***"
	}
	return k[:3] + "***" + k[len(k)-3:]
}

func (h *BotHandler) dbConfigSummary(cfg sheetMasterConfig) string {
	base := cfg.BaseURL
	if base == "" {
		base = "—"
	}
	file := cfg.FileID
	if file == "" {
		file = "—"
	}
	key := h.safeSheetMasterKeyHint(cfg)
	if key == "" {
		key = "—"
	}
	return fmt.Sprintf("BaseURL: %s\nFileID: %s\nAPIKey: %s", base, file, key)
}

func (h *BotHandler) handleDBConfigCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}
	cfg, ok := h.getSheetMasterConfig()
	if !ok {
		h.sendMessage(message.Chat.ID, "ℹ️ Database (SheetMaster) sozlanmagan.\n\nSozlash: /db_set")
		return
	}
	h.sendMessage(message.Chat.ID, "🗄️ Database sozlamalari:\n\n"+h.dbConfigSummary(cfg))
}

// Cleanup helpers (only for setup messages)
func (h *BotHandler) maybeClearSheetMasterSetupOnAdminLogout(userID int64) {
	h.clearSheetMasterSetupState(userID)
}

func isErrUnauthorizedSheetMaster(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid api key")
}

func isErrNotFoundSheetMaster(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}

func (h *BotHandler) maybeSuggestDBSetOnErr(chatID int64, err error) {
	if err == nil {
		return
	}
	if isErrUnauthorizedSheetMaster(err) {
		h.sendMessage(chatID, "⚠️ API key noto'g'ri yoki eskirgan ko'rinadi. Qayta sozlash: /db_set")
		return
	}
	if isErrNotFoundSheetMaster(err) {
		h.sendMessage(chatID, "⚠️ Fayl ID topilmadi. Fayllarni ko'rish: /db_files yoki qayta sozlash: /db_set")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		h.sendMessage(chatID, "⚠️ Database API javob bermadi (timeout). BaseURL to'g'riligini tekshiring: /db_set")
	}
}
