package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	defaultImportAutoInterval = 15 * time.Minute
	minImportAutoInterval     = 1 * time.Minute
	maxImportAutoInterval     = 24 * time.Hour
)

type importAutoInputState struct {
	chatID int64
}

func (h *BotHandler) clampImportAutoInterval(d time.Duration) time.Duration {
	if d <= 0 {
		return defaultImportAutoInterval
	}
	if d < minImportAutoInterval {
		return minImportAutoInterval
	}
	if d > maxImportAutoInterval {
		return maxImportAutoInterval
	}
	return d
}

func (h *BotHandler) startImportAutoLocked(interval time.Duration, runAsUserID int64) chan struct{} {
	interval = h.clampImportAutoInterval(interval)

	if h.importAutoStop != nil {
		close(h.importAutoStop)
		h.importAutoStop = nil
	}

	stop := make(chan struct{})
	h.importAutoStop = stop
	h.importAutoEnabled = true
	h.importAutoInterval = interval
	h.importAutoRunAsUserID = runAsUserID
	h.importAutoLastErr = ""
	return stop
}

func (h *BotHandler) stopImportAutoLocked() {
	if h.importAutoStop != nil {
		close(h.importAutoStop)
		h.importAutoStop = nil
	}
	h.importAutoEnabled = false
}

func (h *BotHandler) beginImportAutoInput(userID, chatID int64) {
	h.importAutoMu.Lock()
	h.importAutoInput[userID] = &importAutoInputState{chatID: chatID}
	h.importAutoMu.Unlock()
}

func (h *BotHandler) clearImportAutoInput(userID int64) {
	h.importAutoMu.Lock()
	delete(h.importAutoInput, userID)
	h.importAutoMu.Unlock()
}

func (h *BotHandler) isAwaitingImportAutoInput(userID int64) bool {
	h.importAutoMu.RLock()
	_, ok := h.importAutoInput[userID]
	h.importAutoMu.RUnlock()
	return ok
}

func (h *BotHandler) importAutoStatusText() string {
	h.importAutoMu.RLock()
	enabled := h.importAutoEnabled
	interval := h.importAutoInterval
	runAs := h.importAutoRunAsUserID
	lastFileID := h.importAutoLastFileID
	lastUpdated := h.importAutoLastUpdatedAt
	lastRun := h.importAutoLastRunAt
	lastCount := h.importAutoLastCount
	lastErr := h.importAutoLastErr
	h.importAutoMu.RUnlock()

	if interval <= 0 {
		interval = defaultImportAutoInterval
	}

	status := "OFF"
	if enabled {
		status = "ON"
	}

	lastRunStr := "—"
	if !lastRun.IsZero() {
		lastRunStr = lastRun.Format("2006-01-02 15:04:05")
	}

	lastFileStr := "—"
	if lastFileID != 0 {
		lastFileStr = fmt.Sprintf("%d", lastFileID)
		if !lastUpdated.IsZero() {
			lastFileStr += " (" + lastUpdated.Format("2006-01-02 15:04:05") + ")"
		}
	}

	errLine := "—"
	if strings.TrimSpace(lastErr) != "" {
		errLine = lastErr
	}

	return fmt.Sprintf(
		"🕒 Auto import: *%s*\n\n⏱ Interval: %s\n👤 Run as: %d\n📄 Oxirgi fayl: %s\n📦 Oxirgi yuklash: %d ta\n🕓 Oxirgi urinish: %s\n⚠️ Oxirgi xatolik: %s\n\nKomandalar:\n- `/import` (import menyusi)\n- `/import_now` (darhol import)\n- `/import_auto` (interval so'raydi)\n- `/import_auto_status`\n- `/import_auto_off`",
		status,
		interval.String(),
		runAs,
		lastFileStr,
		lastCount,
		lastRunStr,
		errLine,
	)
}

func (h *BotHandler) importAutoRunOnce(ctx context.Context) (bool, int, error) {
	selectedID, _ := sheetMasterSelectedFileID()
	var exp sheetMasterDBExport
	var err error
	if cfg, cfgErr := h.resolveSheetMasterConfig(); cfgErr == nil {
		if selectedID != 0 {
			cfg.FileID = strconv.FormatUint(uint64(selectedID), 10)
		}
		exp, err = sheetMasterExportFromAPI(ctx, cfg)
		if err != nil {
			log.Printf("[import_auto] api export error: %v", err)
		}
	}
	if exp.File.ID == 0 {
		exp, err = sheetMasterExportFromDB(ctx, selectedID)
		if err != nil {
			return false, 0, err
		}
	}

	h.importAutoMu.RLock()
	lastID := h.importAutoLastFileID
	lastUpdated := h.importAutoLastUpdatedAt
	runAs := h.importAutoRunAsUserID
	h.importAutoMu.RUnlock()

	// If nothing changed since last successful import, skip.
	if lastID != 0 && exp.File.ID == lastID && exp.File.UpdatedAt.Equal(lastUpdated) {
		return false, 0, nil
	}

	if runAs == 0 {
		return false, 0, fmt.Errorf("runAsUserID is not set")
	}

	// Avoid concurrent imports clobbering catalog.
	h.sheetMasterSyncMu.Lock()
	defer h.sheetMasterSyncMu.Unlock()

	count, err := h.adminUseCase.UploadCatalog(ctx, runAs, exp.XLSXBytes, exp.Filename)
	if err != nil {
		return false, 0, err
	}

	h.importAutoMu.Lock()
	h.importAutoLastFileID = exp.File.ID
	h.importAutoLastUpdatedAt = exp.File.UpdatedAt
	h.importAutoLastCount = count
	h.importAutoMu.Unlock()

	return true, count, nil
}

func (h *BotHandler) importAutoLoop(stop <-chan struct{}) {
	h.importAutoMu.RLock()
	interval := h.importAutoInterval
	h.importAutoMu.RUnlock()
	interval = h.clampImportAutoInterval(interval)

	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		changed, count, err := h.importAutoRunOnce(ctx)

		h.importAutoMu.Lock()
		h.importAutoLastRunAt = time.Now()
		if err != nil {
			h.importAutoLastErr = err.Error()
		} else {
			h.importAutoLastErr = ""
		}
		h.importAutoMu.Unlock()

		if err != nil {
			log.Printf("[import_auto] error: %v", err)
			return
		}
		if changed {
			log.Printf("[import_auto] updated catalog: %d products", count)
		}
	}

	// Run once immediately when enabled.
	run()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			run()
		}
	}
}

func (h *BotHandler) handleImportMenuCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	text := "📤 Import menyusi\n\n" +
		"/import_now — Database'dagi eng oxirgi Excel'dan katalogni yangilaydi (XLSX ham yuboradi)\n" +
		"/import_auto — Avtomatik import intervalini sozlash (keyin son yuborasiz, masalan: 15)\n" +
		"/import_auto_status — Hozirgi holat / oxirgi urinishlar\n" +
		"/import_auto_off — Auto importni o'chirish\n\n" +
		"Auto import faqat oxirgi fayl o'zgarsa qayta import qiladi."
	h.sendMessage(message.Chat.ID, text)
}

func (h *BotHandler) handleImportAutoCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.beginImportAutoInput(userID, message.Chat.ID)

	h.importAutoMu.RLock()
	current := h.importAutoInterval
	enabled := h.importAutoEnabled
	h.importAutoMu.RUnlock()
	if current <= 0 {
		current = defaultImportAutoInterval
	}
	status := "OFF"
	if enabled {
		status = "ON"
	}

	h.sendMessage(message.Chat.ID, fmt.Sprintf("⏱ Auto import intervalini kiriting (daqiqada).\n\nHozirgi: %s (%s)\nMisol: 15\n\nBekor qilish: /import_auto_off\nHolat: /import_auto_status", current.String(), status))
}

func (h *BotHandler) handleImportAutoStatusCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}
	h.sendMessageMarkdown(message.Chat.ID, h.importAutoStatusText())
}

func (h *BotHandler) handleImportAutoOffCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}
	h.importAutoMu.Lock()
	h.stopImportAutoLocked()
	h.importAutoRunAsUserID = userID
	delete(h.importAutoInput, userID)
	h.importAutoMu.Unlock()
	h.sendMessage(message.Chat.ID, "✅ Auto import o'chirildi.")
}

func (h *BotHandler) handleImportAutoInput(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil || msg.Chat == nil {
		return false
	}
	userID := msg.From.ID

	h.importAutoMu.RLock()
	_, ok := h.importAutoInput[userID]
	h.importAutoMu.RUnlock()
	if !ok {
		return false
	}

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.clearImportAutoInput(userID)
		return false
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		h.sendMessage(msg.Chat.ID, "Iltimos, daqiqani son bilan yuboring. Misol: 15")
		return true
	}

	switch strings.ToLower(text) {
	case "/import_auto_off", "/off", "off", "stop", "cancel", "bekor":
		h.importAutoMu.Lock()
		h.stopImportAutoLocked()
		h.importAutoRunAsUserID = userID
		delete(h.importAutoInput, userID)
		h.importAutoMu.Unlock()
		h.sendMessage(msg.Chat.ID, "✅ Auto import o'chirildi.")
		return true
	}

	minutes, err := strconv.Atoi(text)
	if err != nil || minutes <= 0 {
		h.sendMessage(msg.Chat.ID, "❌ Noto'g'ri format. Misol: 15 (daqiqada)")
		return true
	}
	interval := time.Duration(minutes) * time.Minute

	h.importAutoMu.Lock()
	stop := h.startImportAutoLocked(interval, userID)
	delete(h.importAutoInput, userID)
	h.importAutoMu.Unlock()
	go h.importAutoLoop(stop)

	h.sendMessage(msg.Chat.ID, fmt.Sprintf("✅ Auto import yoqildi. Interval: %s", h.clampImportAutoInterval(interval).String()))
	return true
}
