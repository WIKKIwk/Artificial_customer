package telegram

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandler) markGroup1PendingApproval(messageID int) {
	if messageID == 0 {
		return
	}
	h.group1PendingMu.Lock()
	h.group1PendingApprovals[messageID] = struct{}{}
	h.group1PendingMu.Unlock()
}

func (h *BotHandler) clearGroup1PendingApproval(messageID int) {
	if messageID == 0 {
		return
	}
	h.group1PendingMu.Lock()
	delete(h.group1PendingApprovals, messageID)
	h.group1PendingMu.Unlock()
}

func (h *BotHandler) countGroup1PendingApprovals() int {
	h.group1PendingMu.RLock()
	n := len(h.group1PendingApprovals)
	h.group1PendingMu.RUnlock()
	return n
}

func (h *BotHandler) handleStatsCommand(ctx context.Context, message *tgbotapi.Message) {
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

	const maxOrdersForStats = 10000
	orders := h.listRecentOrders(maxOrdersForStats)

	counts := make(map[string]int)
	for _, ord := range orders {
		st := strings.TrimSpace(ord.Status)
		if st == "" {
			st = "processing"
		}
		counts[st]++
	}

	processing := counts["processing"]
	readyDelivery := counts["ready_delivery"]
	readyPickup := counts["ready_pickup"]
	onway := counts["onway"]
	delivered := counts["delivered"]
	canceled := counts["canceled"]

	activeOrders := processing + readyDelivery
	group3Like := readyPickup + onway

	var sb strings.Builder
	sb.WriteString(t(lang, "📊 *Buyurtma statistikasi*\n", "📊 *Статистика заказов*\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")

	if h.group1ChatID != 0 {
		sb.WriteString(fmt.Sprintf("🖥️ %s: *%d*\n", t(lang, "PC konfiguratsiya (admin javobini kutyapti)", "Конфигурации ПК (ждут ответа админа)"), h.countGroup1PendingApprovals()))
	}

	sb.WriteString(fmt.Sprintf("🟡 %s: *%d*\n", t(lang, "Active orders (processing/ready)", "Активные заказы (processing/ready)"), activeOrders))
	sb.WriteString(fmt.Sprintf("✅ %s: *%d*\n", t(lang, "Group 3 (tasdiqlangan)", "Group 3 (подтверждённые)"), group3Like))
	sb.WriteString(fmt.Sprintf("🏁 %s: *%d*\n", t(lang, "Yakunlangan (delivered)", "Завершённые (delivered)"), delivered))
	sb.WriteString(fmt.Sprintf("❌ %s: *%d*\n", t(lang, "Bekor qilingan", "Отменённые"), canceled))

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s: *%d*\n", t(lang, "Jami buyurtmalar", "Всего заказов"), len(orders)))

	h.sendMessageMarkdown(message.Chat.ID, sb.String())
}

