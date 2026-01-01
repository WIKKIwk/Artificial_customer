package telegram

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/xuri/excelize/v2"
)

const (
	hisobotModeDay   = "day"
	hisobotModeMonth = "month"

	hisobotDaysPerPage   = 18
	hisobotMonthsPerPage = 18

	defaultHisobotTZ = "Asia/Tashkent"
	hisobotStateFile = "data/hisobot_state.json"
)

type hisobotStats struct {
	TotalOrders    int
	ComponentsSold int

	ActiveOrders int
	InProgress   int
	Delivered    int
	Canceled     int

	RawStatusCounts map[string]int
}

type hisobotState struct {
	StartedAt time.Time `json:"started_at"`
}

func (h *BotHandler) handleHisobotCommand(ctx context.Context, message *tgbotapi.Message) {
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
	text, kb := buildHisobotMenu(lang)
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = *kb
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(message.Chat.ID, sent.MessageID)
	}
}

func buildHisobotMenu(lang string) (string, *tgbotapi.InlineKeyboardMarkup) {
	text := t(lang,
		"📑 *Hisobot*\n\nQaysi turini ko'rmoqchisiz?",
		"📑 *Отчёт*\n\nКакой тип отчёта показать?",
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "📅 Kunlik hisobot", "📅 Дневной отчёт"), "hisobot_mode|day|0"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🗓 Oylik hisobot", "🗓 Месячный отчёт"), "hisobot_mode|month|0"),
		),
	)
	return text, &kb
}

func (h *BotHandler) handleHisobotMenuCallback(ctx context.Context, chatID int64, adminID int64, srcMsg *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}
	lang := h.getUserLang(adminID)
	text, kb := buildHisobotMenu(lang)
	h.editOrSendHisobotMessage(chatID, text, kb, srcMsg)
}

func (h *BotHandler) handleHisobotModeCallback(ctx context.Context, chatID int64, adminID int64, mode string, page int, srcMsg *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != hisobotModeDay && mode != hisobotModeMonth {
		h.sendMessage(chatID, "❌ Noto'g'ri tanlov.")
		return
	}
	if page < 0 {
		page = 0
	}
	lang := h.getUserLang(adminID)

	var (
		text string
		kb   *tgbotapi.InlineKeyboardMarkup
		err  error
	)
	switch mode {
	case hisobotModeDay:
		text, kb, err = h.buildHisobotDaysList(ctx, lang, page)
	case hisobotModeMonth:
		text, kb, err = h.buildHisobotMonthsList(ctx, lang, page)
	}
	if err != nil {
		h.sendMessage(chatID, t(lang, "❌ Hisobotni tayyorlashda xatolik. Qayta urinib ko'ring.", "❌ Ошибка при формировании отчёта. Попробуйте ещё раз."))
		return
	}
	h.editOrSendHisobotMessage(chatID, text, kb, srcMsg)
}

func (h *BotHandler) handleHisobotDayCallback(ctx context.Context, chatID int64, adminID int64, date string, page int, srcMsg *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}
	lang := h.getUserLang(adminID)
	text, kb, xlsxBytes, filename, caption, err := h.buildHisobotDayReportPayload(ctx, lang, date, page)
	if err != nil {
		h.sendMessage(chatID, t(lang, "❌ Sana noto'g'ri. Iltimos, qayta tanlang.", "❌ Неверная дата. Пожалуйста, выберите снова."))
		return
	}
	h.editOrSendHisobotMessage(chatID, text, kb, srcMsg)
	if len(xlsxBytes) > 0 {
		h.sendHisobotXLSX(chatID, filename, caption, xlsxBytes)
	}
}

func (h *BotHandler) handleHisobotMonthCallback(ctx context.Context, chatID int64, adminID int64, ym string, page int, srcMsg *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, adminID)
	if !isAdmin {
		h.sendMessage(chatID, "❌ Bu funksiya faqat adminlar uchun.")
		return
	}
	lang := h.getUserLang(adminID)
	text, kb, xlsxBytes, filename, caption, err := h.buildHisobotMonthReportPayload(ctx, lang, ym, page)
	if err != nil {
		h.sendMessage(chatID, t(lang, "❌ Oy formati noto'g'ri. Iltimos, qayta tanlang.", "❌ Неверный формат месяца. Пожалуйста, выберите снова."))
		return
	}
	h.editOrSendHisobotMessage(chatID, text, kb, srcMsg)
	if len(xlsxBytes) > 0 {
		h.sendHisobotXLSX(chatID, filename, caption, xlsxBytes)
	}
}

func (h *BotHandler) editOrSendHisobotMessage(chatID int64, text string, kb *tgbotapi.InlineKeyboardMarkup, srcMsg *tgbotapi.Message) {
	if srcMsg != nil && srcMsg.MessageID != 0 {
		markup := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
		if kb != nil {
			markup = *kb
		}
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, srcMsg.MessageID, text, markup)
		edit.ParseMode = "Markdown"
		if _, err := h.sendAndLog(edit); err == nil {
			return
		}
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if kb != nil {
		msg.ReplyMarkup = *kb
	}
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(chatID, sent.MessageID)
	}
}

func (h *BotHandler) buildHisobotDaysList(ctx context.Context, lang string, page int) (string, *tgbotapi.InlineKeyboardMarkup, error) {
	days, err := h.listHisobotDays(ctx)
	if err != nil {
		return "", nil, err
	}
	if len(days) == 0 {
		text := t(lang, "📑 *Kunlik hisobot*\n\nHali hisobot mavjud kunlar yo'q.", "📑 *Дневной отчёт*\n\nПока нет дней с отчётом.")
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "hisobot_menu"),
			),
		)
		return text, &kb, nil
	}

	page = clampPage(page, len(days), hisobotDaysPerPage)
	totalPages := calcTotalPages(len(days), hisobotDaysPerPage)
	start := page * hisobotDaysPerPage
	end := start + hisobotDaysPerPage
	if end > len(days) {
		end = len(days)
	}
	pageDays := days[start:end]

	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}
	for _, day := range pageDays {
		label := day.Format("2006-01-02")
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("hisobot_day|%s|%d", label, page)))
		if len(row) == 3 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	var nav []tgbotapi.InlineKeyboardButton
	if page < totalPages-1 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Eski", "⬅️ Старее"), fmt.Sprintf("hisobot_mode|day|%d", page+1)))
	}
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData(t(lang, "Yangiroq ➡️", "Новее ➡️"), fmt.Sprintf("hisobot_mode|day|%d", page-1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "hisobot_menu"),
	))

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	text := fmt.Sprintf("%s\n%s\n\n%s (%d/%d)",
		t(lang, "📑 *Kunlik hisobot*", "📑 *Дневной отчёт*"),
		t(lang, "Qaysi sanani ko'rishni istaysiz?", "Какую дату показать?"),
		t(lang, "Sahifa", "Страница"),
		page+1,
		totalPages,
	)
	return text, &kb, nil
}

func (h *BotHandler) buildHisobotMonthsList(ctx context.Context, lang string, page int) (string, *tgbotapi.InlineKeyboardMarkup, error) {
	months, err := h.listHisobotMonths(ctx)
	if err != nil {
		return "", nil, err
	}
	if len(months) == 0 {
		text := t(lang, "📑 *Oylik hisobot*\n\nHali hisobot mavjud oylar yo'q.", "📑 *Месячный отчёт*\n\nПока нет месяцев с отчётом.")
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "hisobot_menu"),
			),
		)
		return text, &kb, nil
	}

	page = clampPage(page, len(months), hisobotMonthsPerPage)
	totalPages := calcTotalPages(len(months), hisobotMonthsPerPage)
	start := page * hisobotMonthsPerPage
	end := start + hisobotMonthsPerPage
	if end > len(months) {
		end = len(months)
	}
	pageMonths := months[start:end]

	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}
	for _, ym := range pageMonths {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(ym, fmt.Sprintf("hisobot_month|%s|%d", ym, page)))
		if len(row) == 3 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	var nav []tgbotapi.InlineKeyboardButton
	if page < totalPages-1 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Eski", "⬅️ Старее"), fmt.Sprintf("hisobot_mode|month|%d", page+1)))
	}
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData(t(lang, "Yangiroq ➡️", "Новее ➡️"), fmt.Sprintf("hisobot_mode|month|%d", page-1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "hisobot_menu"),
	))

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	text := fmt.Sprintf("%s\n%s\n\n%s (%d/%d)",
		t(lang, "📑 *Oylik hisobot*", "📑 *Месячный отчёт*"),
		t(lang, "Qaysi oyni ko'rishni istaysiz?", "Какой месяц показать?"),
		t(lang, "Sahifa", "Страница"),
		page+1,
		totalPages,
	)
	return text, &kb, nil
}

func (h *BotHandler) buildHisobotDayReportPayload(ctx context.Context, lang string, date string, page int) (string, *tgbotapi.InlineKeyboardMarkup, []byte, string, string, error) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(date), time.Local)
	if err != nil {
		return "", nil, nil, "", "", err
	}
	year, month, d := day.Date()
	start := time.Date(year, month, d, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1)

	orders, err := h.listOrdersCreatedBetween(ctx, start, end)
	if err != nil {
		return "", nil, nil, "", "", err
	}
	stats := computeHisobotStats(orders)

	var sb strings.Builder
	sb.WriteString(t(lang, "📑 *Kunlik hisobot*\n", "📑 *Дневной отчёт*\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("📅 %s: *%s* (%s)\n\n", t(lang, "Sana", "Дата"), start.Format("2006-01-02"), hisobotTZName()))
	sb.WriteString(fmt.Sprintf("🧩 %s: *%d*\n", t(lang, "Sotilgan komponentlar", "Продано компонентов"), stats.ComponentsSold))
	sb.WriteString(fmt.Sprintf("🛒 %s: *%d*\n", t(lang, "Buyurtmalar", "Заказы"), stats.TotalOrders))
	sb.WriteString(fmt.Sprintf("🟡 %s: *%d*\n", t(lang, "Active (processing/ready)", "Активные (processing/ready)"), stats.ActiveOrders))
	sb.WriteString(fmt.Sprintf("🛠 %s: *%d*\n", t(lang, "Jarayonda (pickup/onway)", "В процессе (pickup/onway)"), stats.InProgress))
	sb.WriteString(fmt.Sprintf("🏁 %s: *%d*\n", t(lang, "Yakunlangan (delivered)", "Завершённые (delivered)"), stats.Delivered))
	sb.WriteString(fmt.Sprintf("❌ %s: *%d*\n", t(lang, "Bekor qilingan", "Отменённые"), stats.Canceled))

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), fmt.Sprintf("hisobot_mode|day|%d", maxInt(page, 0))),
		),
	)
	xlsxBytes, xlsxErr := buildHisobotXLSX(
		fmt.Sprintf("Kunlik hisobot: %s", start.Format("2006-01-02")),
		stats,
		orders,
	)
	if xlsxErr != nil {
		xlsxBytes = nil
	}
	filename := fmt.Sprintf("hisobot_day_%s.xlsx", start.Format("2006-01-02"))
	caption := fmt.Sprintf("📑 Kunlik hisobot\n📅 %s", start.Format("2006-01-02"))
	return sb.String(), &kb, xlsxBytes, filename, caption, nil
}

func (h *BotHandler) buildHisobotMonthReportPayload(ctx context.Context, lang string, ym string, page int) (string, *tgbotapi.InlineKeyboardMarkup, []byte, string, string, error) {
	ym = strings.TrimSpace(ym)
	if ym == "" {
		return "", nil, nil, "", "", fmt.Errorf("empty month")
	}
	parts := strings.SplitN(ym, "-", 2)
	if len(parts) != 2 {
		return "", nil, nil, "", "", fmt.Errorf("invalid month format")
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil || year < 2000 || year > 3000 {
		return "", nil, nil, "", "", fmt.Errorf("invalid year")
	}
	monthInt, err := strconv.Atoi(parts[1])
	if err != nil || monthInt < 1 || monthInt > 12 {
		return "", nil, nil, "", "", fmt.Errorf("invalid month")
	}
	start := time.Date(year, time.Month(monthInt), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)

	orders, err := h.listOrdersCreatedBetween(ctx, start, end)
	if err != nil {
		return "", nil, nil, "", "", err
	}
	stats := computeHisobotStats(orders)

	var sb strings.Builder
	sb.WriteString(t(lang, "📑 *Oylik hisobot*\n", "📑 *Месячный отчёт*\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("🗓 %s: *%s* (%s)\n\n", t(lang, "Oy", "Месяц"), start.Format("2006-01"), hisobotTZName()))
	sb.WriteString(fmt.Sprintf("🧩 %s: *%d*\n", t(lang, "Sotilgan komponentlar", "Продано компонентов"), stats.ComponentsSold))
	sb.WriteString(fmt.Sprintf("🛒 %s: *%d*\n", t(lang, "Buyurtmalar", "Заказы"), stats.TotalOrders))
	sb.WriteString(fmt.Sprintf("🟡 %s: *%d*\n", t(lang, "Active (processing/ready)", "Активные (processing/ready)"), stats.ActiveOrders))
	sb.WriteString(fmt.Sprintf("🛠 %s: *%d*\n", t(lang, "Jarayonda (pickup/onway)", "В процессе (pickup/onway)"), stats.InProgress))
	sb.WriteString(fmt.Sprintf("🏁 %s: *%d*\n", t(lang, "Yakunlangan (delivered)", "Завершённые (delivered)"), stats.Delivered))
	sb.WriteString(fmt.Sprintf("❌ %s: *%d*\n", t(lang, "Bekor qilingan", "Отменённые"), stats.Canceled))

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), fmt.Sprintf("hisobot_mode|month|%d", maxInt(page, 0))),
		),
	)
	xlsxBytes, xlsxErr := buildHisobotXLSX(
		fmt.Sprintf("Oylik hisobot: %s", start.Format("2006-01")),
		stats,
		orders,
	)
	if xlsxErr != nil {
		xlsxBytes = nil
	}
	filename := fmt.Sprintf("hisobot_month_%s.xlsx", start.Format("2006-01"))
	caption := fmt.Sprintf("📑 Oylik hisobot\n🗓 %s", start.Format("2006-01"))
	return sb.String(), &kb, xlsxBytes, filename, caption, nil
}

func (h *BotHandler) listHisobotDays(ctx context.Context) ([]time.Time, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	startDay, err := h.ensureHisobotStartDay(ctx)
	if err != nil {
		return nil, err
	}

	// Prefer DB aggregation when available.
	if store, ok := h.orderStore.(*postgresStore); ok && store != nil && store.db != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		rows, err := store.db.QueryContext(ctx, `
			SELECT DISTINCT (created_at AT TIME ZONE $1)::date AS local_day
			FROM orders
			WHERE created_at >= $2
			ORDER BY local_day DESC
		`, hisobotTZName(), startDay)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var res []time.Time
		for rows.Next() {
			var d time.Time
			if err := rows.Scan(&d); err != nil {
				return nil, err
			}
			y, m, day := d.Date()
			res = append(res, time.Date(y, m, day, 0, 0, 0, 0, time.Local))
		}
		return res, nil
	}

	// Fallback: group in-memory orders.
	h.orderStatusMu.RLock()
	defer h.orderStatusMu.RUnlock()

	seen := make(map[string]struct{})
	var res []time.Time
	for _, ord := range h.orderStatuses {
		if ord.CreatedAt.IsZero() {
			continue
		}
		local := ord.CreatedAt.In(time.Local)
		if local.Before(startDay) {
			continue
		}
		y, m, d := local.Date()
		key := fmt.Sprintf("%04d-%02d-%02d", y, m, d)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		res = append(res, time.Date(y, m, d, 0, 0, 0, 0, time.Local))
	}
	sort.Slice(res, func(i, j int) bool { return res[i].After(res[j]) })
	return res, nil
}

func (h *BotHandler) listHisobotMonths(ctx context.Context) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	startDay, err := h.ensureHisobotStartDay(ctx)
	if err != nil {
		return nil, err
	}

	// Prefer DB aggregation when available.
	if store, ok := h.orderStore.(*postgresStore); ok && store != nil && store.db != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		rows, err := store.db.QueryContext(ctx, `
			SELECT DISTINCT to_char(created_at AT TIME ZONE $1, 'YYYY-MM') AS ym
			FROM orders
			WHERE created_at >= $2
			ORDER BY ym DESC
		`, hisobotTZName(), startDay)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var res []string
		for rows.Next() {
			var ym sql.NullString
			if err := rows.Scan(&ym); err != nil {
				return nil, err
			}
			if strings.TrimSpace(ym.String) == "" {
				continue
			}
			res = append(res, ym.String)
		}
		return res, nil
	}

	// Fallback: group in-memory orders.
	h.orderStatusMu.RLock()
	defer h.orderStatusMu.RUnlock()

	seen := make(map[string]struct{})
	var res []string
	for _, ord := range h.orderStatuses {
		if ord.CreatedAt.IsZero() {
			continue
		}
		local := ord.CreatedAt.In(time.Local)
		if local.Before(startDay) {
			continue
		}
		key := local.Format("2006-01")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		res = append(res, key)
	}
	sort.Slice(res, func(i, j int) bool { return res[i] > res[j] })
	return res, nil
}

func (h *BotHandler) listOrdersCreatedBetween(ctx context.Context, start, end time.Time) ([]orderStatusInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if store, ok := h.orderStore.(*postgresStore); ok && store != nil && store.db != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		rows, err := store.db.QueryContext(ctx, `
			SELECT order_id, user_id, user_chat, username, phone, location, summary, status_summary, total, delivery, status, is_single, created_at
			FROM orders
			WHERE created_at >= $1 AND created_at < $2
			ORDER BY created_at DESC
		`, start, end)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var res []orderStatusInfo
		for rows.Next() {
			var ord orderStatusInfo
			var isSingle sql.NullBool
			var created time.Time
			if err := rows.Scan(&ord.OrderID, &ord.UserID, &ord.UserChat, &ord.Username, &ord.Phone, &ord.Location, &ord.Summary, &ord.StatusSummary, &ord.Total, &ord.Delivery, &ord.Status, &isSingle, &created); err != nil {
				return nil, err
			}
			if isSingle.Valid {
				ord.IsSingleItem = isSingle.Bool
			}
			ord.CreatedAt = created
			res = append(res, ord)
		}
		return res, nil
	}

	h.orderStatusMu.RLock()
	defer h.orderStatusMu.RUnlock()

	var res []orderStatusInfo
	for _, ord := range h.orderStatuses {
		if ord.CreatedAt.IsZero() {
			continue
		}
		if ord.CreatedAt.Before(start) || !ord.CreatedAt.Before(end) {
			continue
		}
		res = append(res, ord)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.After(res[j].CreatedAt) })
	return res, nil
}

func computeHisobotStats(orders []orderStatusInfo) hisobotStats {
	stats := hisobotStats{
		TotalOrders:     len(orders),
		RawStatusCounts: make(map[string]int),
	}
	for _, ord := range orders {
		st := strings.TrimSpace(ord.Status)
		if st == "" {
			st = "processing"
		}
		stats.RawStatusCounts[st]++

		if st == "canceled" {
			continue
		}
		stats.ComponentsSold += len(extractComponentsForStats(ord.Summary))
	}

	processing := stats.RawStatusCounts["processing"]
	readyDelivery := stats.RawStatusCounts["ready_delivery"]
	readyPickup := stats.RawStatusCounts["ready_pickup"]
	onway := stats.RawStatusCounts["onway"]
	delivered := stats.RawStatusCounts["delivered"]
	canceled := stats.RawStatusCounts["canceled"]

	stats.ActiveOrders = processing + readyDelivery
	stats.InProgress = readyPickup + onway
	stats.Delivered = delivered
	stats.Canceled = canceled
	return stats
}

func (h *BotHandler) ensureHisobotStartDay(ctx context.Context) (time.Time, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	state, err := h.ensureHisobotState(ctx)
	if err != nil {
		return time.Time{}, err
	}
	started := state.StartedAt
	if started.IsZero() {
		started = time.Now()
	}
	started = started.In(time.Local)
	y, m, d := started.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local), nil
}

func (h *BotHandler) ensureHisobotState(ctx context.Context) (hisobotState, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if st, ok, err := loadHisobotStateFromDisk(); err == nil && ok && !st.StartedAt.IsZero() {
		return st, nil
	}

	startedAt := time.Now().In(time.Local)
	if earliest, ok, err := h.findEarliestOrderCreatedAt(ctx); err == nil && ok && !earliest.IsZero() {
		startedAt = earliest.In(time.Local)
	}

	state := hisobotState{StartedAt: startedAt}
	_ = saveHisobotStateToDisk(state)
	return state, nil
}

func loadHisobotStateFromDisk() (hisobotState, bool, error) {
	data, err := os.ReadFile(hisobotStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return hisobotState{}, false, nil
		}
		return hisobotState{}, false, err
	}
	var st hisobotState
	if err := json.Unmarshal(data, &st); err != nil {
		return hisobotState{}, true, err
	}
	return st, true, nil
}

func saveHisobotStateToDisk(st hisobotState) error {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(hisobotStateFile, buf, 0o644)
}

func (h *BotHandler) findEarliestOrderCreatedAt(ctx context.Context) (time.Time, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if store, ok := h.orderStore.(*postgresStore); ok && store != nil && store.db != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		row := store.db.QueryRowContext(ctx, `SELECT MIN(created_at) FROM orders`)
		var t sql.NullTime
		if err := row.Scan(&t); err != nil {
			return time.Time{}, false, err
		}
		if !t.Valid || t.Time.IsZero() {
			return time.Time{}, false, nil
		}
		return t.Time, true, nil
	}

	h.orderStatusMu.RLock()
	defer h.orderStatusMu.RUnlock()

	var min time.Time
	for _, ord := range h.orderStatuses {
		if ord.CreatedAt.IsZero() {
			continue
		}
		if min.IsZero() || ord.CreatedAt.Before(min) {
			min = ord.CreatedAt
		}
	}
	if min.IsZero() {
		return time.Time{}, false, nil
	}
	return min, true, nil
}

func (h *BotHandler) sendHisobotXLSX(chatID int64, filename string, caption string, bytes []byte) {
	if strings.TrimSpace(filename) == "" || len(bytes) == 0 {
		return
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{Name: filename, Bytes: bytes})
	if strings.TrimSpace(caption) != "" {
		doc.Caption = caption
	}
	sent, err := h.sendAndLog(doc)
	if err != nil {
		return
	}
	h.trackAdminMessage(chatID, sent.MessageID)
}

func buildHisobotXLSX(period string, stats hisobotStats, orders []orderStatusInfo) ([]byte, error) {
	f := excelize.NewFile()

	summarySheet := f.GetSheetName(0)
	_ = f.SetSheetName(summarySheet, "Summary")
	summarySheet = "Summary"

	if _, err := f.NewSheet("Orders"); err != nil {
		return nil, err
	}

	summary := [][]interface{}{
		{"Hisobot", ""},
		{"Davr", period},
		{"Timezone", hisobotTZName()},
		{"Jami buyurtmalar", stats.TotalOrders},
		{"Sotilgan komponentlar", stats.ComponentsSold},
		{"Active (processing/ready)", stats.ActiveOrders},
		{"Jarayonda (pickup/onway)", stats.InProgress},
		{"Yakunlangan (delivered)", stats.Delivered},
		{"Bekor qilingan", stats.Canceled},
	}
	for i, row := range summary {
		for j, v := range row {
			cell, err := excelize.CoordinatesToCellName(j+1, i+1)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(summarySheet, cell, v); err != nil {
				return nil, err
			}
		}
	}

	headers := []string{
		"CreatedAt (UZ)",
		"OrderID",
		"Status",
		"Username",
		"Phone",
		"Location",
		"Delivery",
		"Total",
		"ComponentsCount",
		"Components",
		"Summary",
		"StatusSummary",
	}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue("Orders", cell, h); err != nil {
			return nil, err
		}
	}

	for idx, ord := range orders {
		rowIdx := idx + 2
		created := ord.CreatedAt
		if !created.IsZero() {
			created = created.In(time.Local)
		}
		components := extractComponentsForStats(ord.Summary)
		values := []interface{}{
			formatOptionalTime(created),
			ord.OrderID,
			ord.Status,
			ord.Username,
			ord.Phone,
			ord.Location,
			ord.Delivery,
			ord.Total,
			len(components),
			strings.Join(components, ", "),
			strings.TrimSpace(ord.Summary),
			strings.TrimSpace(ord.StatusSummary),
		}
		for c, v := range values {
			cell, err := excelize.CoordinatesToCellName(c+1, rowIdx)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue("Orders", cell, v); err != nil {
				return nil, err
			}
		}
	}

	f.SetActiveSheet(0)

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func calcTotalPages(total int, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 1
	}
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	if pages < 1 {
		pages = 1
	}
	return pages
}

func clampPage(page int, totalItems int, pageSize int) int {
	if page < 0 {
		return 0
	}
	totalPages := calcTotalPages(totalItems, pageSize)
	if page >= totalPages {
		return totalPages - 1
	}
	return page
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func hisobotTZName() string {
	name := strings.TrimSpace(time.Local.String())
	if name == "" || strings.EqualFold(name, "local") {
		return defaultHisobotTZ
	}
	return name
}
