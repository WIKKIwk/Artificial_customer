package telegram

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	addProductInlinePrefix       = "add_product:"
	addProductInlineResultsLimit = 20
)

func (h *BotHandler) handleAddProductCommand(ctx context.Context, message *tgbotapi.Message) {
	h.handleInventoryAdjustCommand(ctx, message, 1)
}

func (h *BotHandler) handleRemoveProductCommand(ctx context.Context, message *tgbotapi.Message) {
	h.handleInventoryAdjustCommand(ctx, message, -1)
}

func (h *BotHandler) handleInventoryAdjustCommand(ctx context.Context, message *tgbotapi.Message, delta int) {
	if message == nil || message.From == nil {
		return
	}
	userID := message.From.ID
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	h.startAdminSession(userID, message.Chat.ID)
	h.startAddProductFlow(userID, message.Chat.ID, delta)
	h.setAwaitingSearch(userID, false)

	query := ""
	btn := tgbotapi.InlineKeyboardButton{
		Text:                         "🔎 Qidirish",
		SwitchInlineQueryCurrentChat: &query,
	}
	prompt := "➕ Qaysi mahsulotni qo'shmoqchisiz? Pastdagi qidiruv tugmasini bosing."
	if delta < 0 {
		prompt = "➖ Qaysi mahsulotni kamaytirmoqchisiz? Pastdagi qidiruv tugmasini bosing."
	}
	msg := tgbotapi.NewMessage(message.Chat.ID, prompt)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackAdminMessage(message.Chat.ID, sent.MessageID)
	}
}

func (h *BotHandler) handleAddProductQuantityInput(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil {
		return false
	}
	_ = ctx
	userID := msg.From.ID
	state, ok := h.getAddProductState(userID)
	if !ok || state.Stage != addProductStageNeedQty {
		return false
	}

	amount, ok := parsePositiveInt(msg.Text)
	if !ok {
		chatID := state.ChatID
		if chatID == 0 {
			chatID = msg.Chat.ID
		}
		h.sendMessage(chatID, "❌ Iltimos, musbat butun son kiriting. Masalan: 5")
		return true
	}

	h.clearAddProductState(userID)

	chatID := state.ChatID
	if chatID == 0 {
		chatID = msg.Chat.ID
	}
	productName := strings.TrimSpace(state.ProductName)
	delta := state.Delta
	if delta == 0 {
		delta = 1
	}
	verb := "qo'shilyapti"
	if delta < 0 {
		verb = "kamaytiryapti"
	}
	h.sendMessage(chatID, fmt.Sprintf("⏳ \"%s\" uchun %d ta %s...", productName, amount, verb))

	go func(chatID int64, name string, qty int) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		updated, err := h.adjustInventoryByName(ctx, name, delta*qty)
		if err != nil {
			h.sendMessage(chatID, fmt.Sprintf("❌ Xatolik: %v", err))
			return
		}
		if updated == 0 {
			h.sendMessage(chatID, fmt.Sprintf("❌ \"%s\" mahsuloti sheetda topilmadi.", name))
			return
		}
		sign := "+"
		if delta < 0 {
			sign = "-"
		}
		h.sendMessage(chatID, fmt.Sprintf("✅ \"%s\" uchun %s%d ta yangilandi.", name, sign, qty))
	}(chatID, productName, amount)

	return true
}

func (h *BotHandler) handleAddProductInlineSelectionMessage(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil {
		return false
	}
	_ = ctx
	if msg.ViaBot == nil || msg.ViaBot.ID != h.bot.Self.ID {
		return false
	}
	userID := msg.From.ID
	state, ok := h.getAddProductState(userID)
	if !ok || state.Stage != addProductStageNeedSelect {
		return false
	}
	productName := extractAddProductSelection(msg.Text)
	if productName == "" {
		return false
	}

	h.setAwaitingSearch(userID, false)
	nextState := h.setAddProductSelection(userID, "", productName)
	chatID := nextState.ChatID
	if chatID == 0 {
		chatID = msg.Chat.ID
	}
	h.sendMessage(chatID, h.inventoryAdjustCountPrompt(productName, nextState.Delta))
	return true
}

func (h *BotHandler) handleInlineQuery(ctx context.Context, query *tgbotapi.InlineQuery) {
	if query == nil || query.From == nil {
		return
	}
	userID := query.From.ID

	state, ok := h.getAddProductState(userID)
	if !ok || state.Stage != addProductStageNeedSelect {
		if h.isAwaitingUserHistory(userID) {
			h.handleUserChatHistoryInlineQuery(ctx, query)
			return
		}
		h.answerInlineQuery(query.ID, nil)
		return
	}

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.answerInlineQuery(query.ID, nil)
		return
	}

	q := strings.TrimSpace(query.Query)
	if q == "" {
		h.answerInlineQuery(query.ID, nil)
		return
	}

	products, err := h.productUseCase.Search(ctx, q)
	if err != nil || len(products) == 0 {
		h.answerInlineQuery(query.ID, nil)
		return
	}

	results := make([]interface{}, 0, addProductInlineResultsLimit)
	for _, p := range products {
		if len(results) >= addProductInlineResultsLimit {
			break
		}
		resultID := addProductInlinePrefix + p.ID
		messageText := fmt.Sprintf("✅ Tanlandi: %s", p.Name)
		result := tgbotapi.NewInlineQueryResultArticle(resultID, p.Name, messageText)
		descParts := make([]string, 0, 3)
		if p.Category != "" {
			descParts = append(descParts, p.Category)
		}
		if p.Price > 0 {
			descParts = append(descParts, fmt.Sprintf("$%.2f", p.Price))
		}
		descParts = append(descParts, fmt.Sprintf("Soni: %d", p.Stock))
		result.Description = strings.Join(descParts, " | ")
		results = append(results, result)
	}

	h.answerInlineQuery(query.ID, results)
}

func (h *BotHandler) handleChosenInlineResult(ctx context.Context, chosen *tgbotapi.ChosenInlineResult) {
	if chosen == nil || chosen.From == nil {
		return
	}
	if !strings.HasPrefix(chosen.ResultID, addProductInlinePrefix) {
		return
	}

	userID := chosen.From.ID
	state, ok := h.getAddProductState(userID)
	if !ok || state.Stage != addProductStageNeedSelect {
		return
	}

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		return
	}

	productID := strings.TrimPrefix(chosen.ResultID, addProductInlinePrefix)
	if productID == "" {
		return
	}
	productName, ok := h.findProductNameByID(ctx, productID)
	if !ok {
		chatID := state.ChatID
		if chatID == 0 {
			chatID = userID
		}
		h.sendMessage(chatID, "❌ Mahsulot topilmadi. Qayta qidirib ko'ring.")
		return
	}

	h.setAwaitingSearch(userID, false)
	nextState := h.setAddProductSelection(userID, productID, productName)
	chatID := nextState.ChatID
	if chatID == 0 {
		chatID = userID
	}
	h.sendMessage(chatID, h.inventoryAdjustCountPrompt(productName, nextState.Delta))
}

func (h *BotHandler) answerInlineQuery(queryID string, results []interface{}) {
	if results == nil {
		results = []interface{}{}
	}
	cfg := tgbotapi.InlineConfig{
		InlineQueryID: queryID,
		Results:       results,
		IsPersonal:    true,
		CacheTime:     1,
	}
	if _, err := h.bot.Request(cfg); err != nil {
		log.Printf("[inline] answer query failed: %v", err)
	}
}

func (h *BotHandler) findProductNameByID(ctx context.Context, productID string) (string, bool) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return "", false
	}
	products, err := h.productUseCase.GetAll(ctx)
	if err != nil {
		return "", false
	}
	for _, p := range products {
		if p.ID == productID {
			return p.Name, true
		}
	}
	return "", false
}

func parsePositiveInt(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	val, ok := parseNumberWithSeparators(raw)
	if !ok {
		return 0, false
	}
	rounded := int(math.Round(val))
	if rounded <= 0 {
		return 0, false
	}
	if math.Abs(val-float64(rounded)) > 0.000001 {
		return 0, false
	}
	return rounded, true
}

func (h *BotHandler) inventoryAdjustCountPrompt(productName string, delta int) string {
	if delta < 0 {
		return fmt.Sprintf("📦 \"%s\" tanlandi. Nechta kamaytirmoqchisiz?", productName)
	}
	return fmt.Sprintf("📦 \"%s\" tanlandi. Nechta qo'shmoqchisiz?", productName)
}

func extractAddProductSelection(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "tanlandi")
	if idx == -1 {
		return ""
	}
	rest := strings.TrimSpace(text[idx+len("tanlandi"):])
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, "—") {
		rest = strings.TrimSpace(rest[1:])
	}
	if rest == "" {
		if colon := strings.LastIndex(text, ":"); colon >= 0 && colon+1 < len(text) {
			rest = strings.TrimSpace(text[colon+1:])
		}
	}
	rest = strings.Trim(rest, "\"'` ")
	return rest
}

func (h *BotHandler) startAddProductFlow(userID, chatID int64, delta int) {
	h.addProductMu.Lock()
	h.addProductState[userID] = &addProductState{
		ChatID: chatID,
		Stage:  addProductStageNeedSelect,
		Delta:  delta,
	}
	h.addProductMu.Unlock()
}

func (h *BotHandler) setAddProductSelection(userID int64, productID, productName string) addProductState {
	h.addProductMu.Lock()
	state := h.addProductState[userID]
	if state == nil {
		state = &addProductState{Delta: 1}
		h.addProductState[userID] = state
	}
	state.ProductID = productID
	state.ProductName = productName
	if state.ChatID == 0 {
		state.ChatID = userID
	}
	state.Stage = addProductStageNeedQty
	snapshot := *state
	h.addProductMu.Unlock()
	return snapshot
}

func (h *BotHandler) getAddProductState(userID int64) (addProductState, bool) {
	h.addProductMu.RLock()
	state := h.addProductState[userID]
	h.addProductMu.RUnlock()
	if state == nil {
		return addProductState{}, false
	}
	return *state, true
}

func (h *BotHandler) clearAddProductState(userID int64) {
	h.addProductMu.Lock()
	delete(h.addProductState, userID)
	h.addProductMu.Unlock()
}
