package telegram

import (
	"context"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandler) handleConfigTypeSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	switch value {
	case "office", "gaming", "developer", "design", "montaj", "server":
		h.setConfigTypeAndAskBudget(userID, chatID, strings.Title(value))
	case "other":
		h.configMu.Lock()
		if sess, ok := h.configSessions[userID]; ok {
			sess.Stage = configStageNeedType
			sess.LastUpdate = time.Now()
			h.configSessions[userID] = sess
		}
		h.configMu.Unlock()
		h.promptConfigTypeText(userID, chatID)
	default:
		h.promptConfigTypeText(userID, chatID)
	}
}

func (h *BotHandler) handleConfigCPUSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	switch value {
	case "intel":
		h.setConfigCPUAndAskCPUCooler(userID, chatID, "Intel")
	case "amd":
		h.setConfigCPUAndAskCPUCooler(userID, chatID, "AMD")
	case "other":
		h.configMu.Lock()
		if sess, ok := h.configSessions[userID]; ok {
			sess.Stage = configStageNeedCPU
			sess.LastUpdate = time.Now()
			h.configSessions[userID] = sess
		}
		h.configMu.Unlock()
		h.promptConfigCPUText(userID, chatID)
	default:
		h.promptConfigCPUText(userID, chatID)
	}
}

func (h *BotHandler) handleConfigStorageSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	switch value {
	case "ssd":
		h.setConfigStorageAndAskGPU(userID, chatID, "SSD")
	case "nvme":
		h.setConfigStorageAndAskGPU(userID, chatID, "NVMe")
	case "hdd":
		h.setConfigStorageAndAskGPU(userID, chatID, "HDD")
	default:
		h.setConfigStorageAndAskGPU(userID, chatID, value)
	}
}

func (h *BotHandler) handleConfigGPUSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	h.setConfigGPUAndAskMonitor(userID, chatID, normalizeGPUSelection(value))
}

func (h *BotHandler) handleConfigMonitorSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	switch value {
	case "yes":
		h.setConfigMonitorYesAndAskHz(userID, chatID)
	case "no":
		h.setConfigMonitorNoAndAskPeripherals(userID, chatID)
	default:
		h.setConfigMonitorNoAndAskPeripherals(userID, chatID)
	}
}

func (h *BotHandler) handleConfigMonitorHzSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	hz := value
	switch value {
	case "60":
		hz = "60Hz"
	case "144":
		hz = "144Hz"
	case "240":
		hz = "240Hz"
	case "300":
		hz = "300Hz+"
	}
	h.setConfigMonitorHzAndAskDisplay(userID, chatID, hz)
}

func (h *BotHandler) handleConfigMonitorDisplaySelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	display := value
	switch value {
	case "ips":
		display = "IPS"
	case "va":
		display = "VA"
	case "tn":
		display = "TN"
	case "oled":
		display = "OLED"
	case "miniled":
		display = "miniLED"
	}
	h.setConfigMonitorDisplayAndAskPeripherals(userID, chatID, display)
}

func (h *BotHandler) handleConfigPeripheralsSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	switch value {
	case "yes":
		h.setConfigPeripheralsYesAndFinish(ctx, userID, username, chatID)
	case "no":
		h.setConfigPeripheralsNoAndFinish(ctx, userID, username, chatID)
	default:
		h.setConfigPeripheralsNoAndFinish(ctx, userID, username, chatID)
	}
}

func (h *BotHandler) handleConfigColorSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	lang := h.getUserLang(userID)
	switch value {
	case "black":
		h.setConfigColorAndAskCPU(userID, chatID, t(lang, "Qora", "Чёрный"))
	case "white":
		h.setConfigColorAndAskCPU(userID, chatID, t(lang, "Oq", "Белый"))
	default:
		h.setConfigColorAndAskCPU(userID, chatID, t(lang, "Qora", "Чёрный")) // Default black
	}
}

func (h *BotHandler) handleConfigCPUCoolerSelection(ctx context.Context, userID, chatID int64, msg *tgbotapi.Message, username, value string) {
	switch value {
	case "air":
		h.setConfigCPUCoolerAndAskStorage(userID, chatID, "Air")
	case "liquid":
		h.setConfigCPUCoolerAndAskStorage(userID, chatID, "Liquid")
	case "other":
		h.configMu.Lock()
		if sess, ok := h.configSessions[userID]; ok {
			sess.Stage = configStageNeedCPUCooler
			sess.LastUpdate = time.Now()
			h.configSessions[userID] = sess
		}
		h.configMu.Unlock()
		h.promptConfigCPUCoolerText(userID, chatID)
	default:
		h.promptConfigCPUCoolerText(userID, chatID)
	}
}
