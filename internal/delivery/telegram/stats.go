package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	dayStart, dayEnd, dayLabel, err := parseStatsDayRange(message.CommandArguments(), time.Now())
	if err != nil {
		h.sendMessage(message.Chat.ID, t(lang,
			"❌ Sana formati noto'g'ri. Misol: /stats 2026-01-01 yoki /stats bugun yoki /stats kecha",
			"❌ Неверный формат даты. Пример: /stats 2026-01-01 или /stats today или /stats yesterday",
		))
		return
	}

	counts := make(map[string]int)
	var dayOrders, dayCanceled, dayComponents int
	for _, ord := range orders {
		st := strings.TrimSpace(ord.Status)
		if st == "" {
			st = "processing"
		}
		counts[st]++

		if ord.CreatedAt.IsZero() {
			continue
		}
		created := ord.CreatedAt.In(time.Local)
		if created.Before(dayStart) || !created.Before(dayEnd) {
			continue
		}
		dayOrders++
		if st == "canceled" {
			dayCanceled++
			continue
		}
		dayComponents += len(extractComponentsForStats(ord.Summary))
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
	sb.WriteString(fmt.Sprintf("📅 %s: *%s* (%s)\n",
		t(lang, "Sana", "Дата"),
		dayLabel,
		time.Local.String(),
	))
	sb.WriteString(fmt.Sprintf("🧩 %s: *%d*\n", t(lang, "Sotilgan komponentlar", "Продано компонентов"), dayComponents))
	sb.WriteString(fmt.Sprintf("🛒 %s: *%d*\n", t(lang, "Buyurtmalar (shu kunda)", "Заказов (за день)"), dayOrders))
	sb.WriteString(fmt.Sprintf("❌ %s: *%d*\n", t(lang, "Bekor qilingan (shu kunda)", "Отменено (за день)"), dayCanceled))

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s: *%d*\n", t(lang, "Jami buyurtmalar", "Всего заказов"), len(orders)))

	h.sendMessageMarkdown(message.Chat.ID, sb.String())
}

func parseStatsDayRange(rawArg string, now time.Time) (time.Time, time.Time, string, error) {
	arg := strings.TrimSpace(rawArg)
	if arg != "" {
		arg = strings.Fields(arg)[0]
	}
	now = now.In(time.Local)
	switch strings.ToLower(arg) {
	case "", "today", "bugun", "сегодня":
		year, month, day := now.Date()
		start := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		return start, start.AddDate(0, 0, 1), start.Format("2006-01-02"), nil
	case "yesterday", "kecha", "вчера":
		t := now.AddDate(0, 0, -1)
		year, month, day := t.Date()
		start := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		return start, start.AddDate(0, 0, 1), start.Format("2006-01-02"), nil
	}

	for _, layout := range []string{"2006-01-02", "02.01.2006", "02/01/2006"} {
		if t, err := time.ParseInLocation(layout, arg, time.Local); err == nil {
			year, month, day := t.Date()
			start := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
			return start, start.AddDate(0, 0, 1), start.Format("2006-01-02"), nil
		}
	}
	return time.Time{}, time.Time{}, "", fmt.Errorf("invalid date: %q", arg)
}

func extractComponentsForStats(summary string) []string {
	items := extractConfigItemNames(summary)
	if len(items) == 0 {
		items = extractOrderItemNames(summary)
	}
	return items
}
