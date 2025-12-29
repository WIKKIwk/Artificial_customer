package telegram

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/xuri/excelize/v2"
)

type userExportRow struct {
	UserID                 int64
	ChatID                 int64
	Username               string
	Name                   string
	Phone                  string
	Location               string
	Lang                   string
	LastSeen               time.Time
	OrdersCount            int
	LastOrderID            string
	LastOrderStatus        string
	LastOrderTotal         string
	LastOrderDelivery      string
	LastOrderAt            time.Time
	LastOrderSummary       string
	LastOrderStatusSummary string
}

func (h *BotHandler) handleAboutUserCommand(ctx context.Context, message *tgbotapi.Message) {
	if message == nil || message.From == nil {
		return
	}
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.deleteCommandMessage(message)
	progress, _ := h.sendMessageWithResp(message.Chat.ID, "⏳ Foydalanuvchilar eksporti tayyorlanmoqda...")

	rows := h.buildUserExportRows()
	if err := h.syncAboutUserSheet(ctx, rows); err != nil {
		log.Printf("about_user sheet sync error: %v", err)
		h.sendMessage(message.Chat.ID, "⚠️ SheetMaster fayliga yozishda xatolik yuz berdi.")
	}

	xlsxBytes, err := buildUserExportXLSX(rows)
	if err != nil {
		log.Printf("user export xlsx error: %v", err)
		if progress != nil {
			h.deleteMessage(message.Chat.ID, progress.MessageID)
		}
		h.sendMessage(message.Chat.ID, "❌ Excel fayl tayyorlashda xatolik yuz berdi.")
		return
	}

	filename := fmt.Sprintf("users_%s.xlsx", time.Now().Format("20060102_150405"))
	doc := tgbotapi.NewDocument(message.Chat.ID, tgbotapi.FileBytes{Name: filename, Bytes: xlsxBytes})
	doc.Caption = fmt.Sprintf("👥 Foydalanuvchilar eksporti\nJami: %d ta", len(rows))
	sent, err := h.sendAndLog(doc)
	if err != nil {
		log.Printf("user export send error: %v", err)
		if progress != nil {
			h.deleteMessage(message.Chat.ID, progress.MessageID)
		}
		h.sendMessage(message.Chat.ID, "❌ Excel fayl yuborishda xatolik yuz berdi.")
		return
	}
	h.trackAdminMessage(message.Chat.ID, sent.MessageID)
	if progress != nil {
		h.deleteMessage(message.Chat.ID, progress.MessageID)
	}
}

func (h *BotHandler) buildUserExportRows() []userExportRow {
	rows := make(map[int64]*userExportRow)
	getRow := func(id int64) *userExportRow {
		if id == 0 {
			return nil
		}
		if row, ok := rows[id]; ok {
			return row
		}
		row := &userExportRow{UserID: id}
		rows[id] = row
		return row
	}

	h.lastSeenMu.RLock()
	h.nameMu.RLock()
	for id, ts := range h.lastSeen {
		row := getRow(id)
		if row == nil {
			continue
		}
		row.LastSeen = ts
		if row.Username == "" {
			row.Username = strings.TrimSpace(h.lastName[id])
		}
	}
	h.nameMu.RUnlock()
	h.lastSeenMu.RUnlock()

	h.profileMu.RLock()
	for id, prof := range h.profiles {
		row := getRow(id)
		if row == nil {
			continue
		}
		if row.Name == "" {
			row.Name = strings.TrimSpace(prof.Name)
		}
		if row.Phone == "" {
			row.Phone = strings.TrimSpace(prof.Phone)
		}
	}
	h.profileMu.RUnlock()

	h.langMu.RLock()
	for id, lang := range h.userLang {
		row := getRow(id)
		if row == nil {
			continue
		}
		if row.Lang == "" {
			row.Lang = strings.TrimSpace(lang)
		}
	}
	h.langMu.RUnlock()

	const maxOrdersForExport = 10000
	for _, ord := range h.listRecentOrders(maxOrdersForExport) {
		if ord.UserID == 0 {
			continue
		}
		row := getRow(ord.UserID)
		if row == nil {
			continue
		}
		row.OrdersCount++
		if row.ChatID == 0 && ord.UserChat != 0 {
			row.ChatID = ord.UserChat
		}
		if row.Username == "" {
			row.Username = strings.TrimSpace(ord.Username)
		}
		if row.Phone == "" {
			row.Phone = strings.TrimSpace(ord.Phone)
		}
		if row.Location == "" {
			row.Location = strings.TrimSpace(ord.Location)
		}
		if row.LastOrderAt.IsZero() || ord.CreatedAt.After(row.LastOrderAt) {
			row.LastOrderAt = ord.CreatedAt
			row.LastOrderID = ord.OrderID
			row.LastOrderStatus = ord.Status
			row.LastOrderTotal = ord.Total
			row.LastOrderDelivery = ord.Delivery
			row.LastOrderSummary = strings.TrimSpace(ord.Summary)
			row.LastOrderStatusSummary = strings.TrimSpace(ord.StatusSummary)
		}
	}

	out := make([]userExportRow, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.UserID == 0 {
			continue
		}
		if row.ChatID == 0 {
			row.ChatID = row.UserID
		}
		out = append(out, *row)
	}

	sort.Slice(out, func(i, j int) bool {
		ti := out[i].LastSeen
		if ti.IsZero() {
			ti = out[i].LastOrderAt
		}
		tj := out[j].LastSeen
		if tj.IsZero() {
			tj = out[j].LastOrderAt
		}
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return out[i].UserID < out[j].UserID
	})

	return out
}

func buildUserExportXLSX(rows []userExportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	headers := userExportHeaders()

	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}

	for i, row := range rows {
		values := userExportRowValues(row)
		rowIdx := i + 2
		for c, v := range values {
			cell, err := excelize.CoordinatesToCellName(c+1, rowIdx)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func userExportHeaders() []string {
	return []string{
		"User ID",
		"Chat ID",
		"Username",
		"Name",
		"Phone",
		"Location",
		"Language",
		"Last Seen",
		"Orders Count",
		"Last Order ID",
		"Last Order Status",
		"Last Order Total",
		"Last Order Delivery",
		"Last Order At",
		"Last Order Summary",
		"Last Order Status Summary",
	}
}

func userExportRowValues(row userExportRow) []interface{} {
	return []interface{}{
		row.UserID,
		row.ChatID,
		row.Username,
		row.Name,
		row.Phone,
		row.Location,
		row.Lang,
		formatExportTime(row.LastSeen),
		row.OrdersCount,
		row.LastOrderID,
		row.LastOrderStatus,
		row.LastOrderTotal,
		row.LastOrderDelivery,
		formatExportTime(row.LastOrderAt),
		row.LastOrderSummary,
		row.LastOrderStatusSummary,
	}
}

func userExportRowStrings(row userExportRow) []string {
	values := userExportRowValues(row)
	out := make([]string, len(values))
	for i, v := range values {
		switch t := v.(type) {
		case string:
			out[i] = t
		case nil:
			out[i] = ""
		default:
			out[i] = fmt.Sprint(t)
		}
	}
	return out
}

func formatExportTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format("2006-01-02 15:04:05")
}
