package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Cart helpers
func (h *BotHandler) addToCart(userID int64, item cartItem) {
	item.Title = strings.TrimSpace(item.Title)
	item.Text = strings.TrimSpace(item.Text)
	if item.Title == "" && item.Text == "" {
		return
	}
	h.cartMu.Lock()
	h.cartItems[userID] = append(h.cartItems[userID], item)
	h.cartMu.Unlock()
}

func (h *BotHandler) listCart(userID int64) []cartItem {
	h.cartMu.RLock()
	defer h.cartMu.RUnlock()
	return append([]cartItem(nil), h.cartItems[userID]...)
}

func (h *BotHandler) clearCart(userID int64) {
	h.cartMu.Lock()
	delete(h.cartItems, userID)
	h.cartMu.Unlock()
}

func (h *BotHandler) removeCartItem(userID int64, idx int) bool {
	h.cartMu.Lock()
	defer h.cartMu.Unlock()
	items := h.cartItems[userID]
	if idx < 0 || idx >= len(items) {
		return false
	}
	items = append(items[:idx], items[idx+1:]...)
	if len(items) == 0 {
		delete(h.cartItems, userID)
	} else {
		h.cartItems[userID] = items
	}
	return true
}

// normalizeCartTitleCandidate " -- " va narxdan oldingi qismini ajratib oladi
func normalizeCartTitleCandidate(line string) string {
	t := strings.TrimSpace(line)
	if t == "" {
		return ""
	}
	// bullet/prefixlarni olib tashlaymiz
	t = strings.TrimLeft(t, "*-• ")
	t = strings.TrimLeftFunc(t, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	lower := strings.ToLower(t)
	// "Ajoyib tanlov!" kabi marketing prefikslarni olib tashlaymiz
	if strings.HasPrefix(lower, "ajoyib tanlov!") {
		t = strings.TrimSpace(t[len("ajoyib tanlov!"):])
		lower = strings.ToLower(t)
	} else if strings.HasPrefix(lower, "ajyob tanlov!") {
		t = strings.TrimSpace(t[len("ajyob tanlov!"):])
		lower = strings.ToLower(t)
	}
	// Agar matnda qo'shtirnoqdagi model bo'lsa, ichkarisini olamiz
	if i := strings.Index(t, "\""); i >= 0 {
		if j := strings.LastIndex(t, "\""); j > i {
			inner := strings.TrimSpace(t[i+1 : j])
			if inner != "" {
				t = inner
				lower = strings.ToLower(t)
			}
		}
	}
	// "Ha, bizda" prefiksini tashlaymiz
	for _, pref := range []string{"ha, bizda", "bizda", "bizning"} {
		if strings.HasPrefix(lower, pref) {
			t = strings.TrimSpace(t[len(pref):])
			lower = strings.ToLower(t)
			break
		}
	}
	// "--" dan keyingi qismni olamiz
	if idx := strings.Index(t, "--"); idx >= 0 {
		t = strings.TrimSpace(t[idx+2:])
	}
	// Narxdan oldingi qismini olamiz
	if loc := priceWithCurrencyRegex.FindStringIndex(t); loc != nil && loc[0] > 0 {
		t = strings.TrimSpace(t[:loc[0]])
	}
	// Trailing so'zlar: bor, mavjud, narxi, narx, jami
	for _, kw := range []string{"mavjud", "bor", "narxi", "narx", "jami"} {
		lower := strings.ToLower(t)
		if idx := strings.LastIndex(lower, kw); idx > 0 {
			t = strings.TrimSpace(t[:idx])
			break
		}
	}
	// Marketing jumlalar: "modelini tanladingiz" va hokazo
	for _, kw := range []string{"modelini tanladingiz", "tanladingiz", "tanlagansiz"} {
		lower := strings.ToLower(t)
		if idx := strings.LastIndex(lower, kw); idx > 0 {
			t = strings.TrimSpace(t[:idx])
			break
		}
	}
	// Keraksiz delimiterlarni qirqamiz
	t = strings.Trim(t, " -–—:.,*_")
	return t
}

func cartTitleFromText(s string) string {
	lines := strings.Split(s, "\n")
	var bulletCandidates []string
	var pricedCandidate string
	var firstNonEmpty string

	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		norm := normalizeCartTitleCandidate(t)
		if norm == "" {
			continue
		}
		if strings.HasPrefix(t, "--") {
			bulletCandidates = append(bulletCandidates, norm)
			if priceWithCurrencyRegex.MatchString(ln) {
				pricedCandidate = norm
			}
			continue
		}
		if strings.HasPrefix(t, "*") || strings.HasPrefix(t, "-") || strings.HasPrefix(t, "•") {
			bulletCandidates = append(bulletCandidates, norm)
			if priceWithCurrencyRegex.MatchString(ln) {
				pricedCandidate = norm
			}
			continue
		}
		if firstNonEmpty == "" {
			firstNonEmpty = norm
		}
	}

	title := ""
	if pricedCandidate != "" {
		title = pricedCandidate
	} else if len(bulletCandidates) > 0 {
		title = bulletCandidates[0]
	} else if firstNonEmpty != "" {
		title = firstNonEmpty
	} else {
		title = s
	}

	// "Narxi" yoki "Jami" qismi qo'shilib ketgan bo'lsa, kesib tashlaymiz
	if idx := strings.Index(strings.ToLower(title), "narxi"); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	}
	if idx := strings.Index(strings.ToLower(title), "jami"); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	}
	// "bor"/"mavjud" iboralari ulashib ketgan bo'lsa, kesib tashlaymiz
	lower := strings.ToLower(title)
	for _, kw := range []string{" bor", "mavjud"} {
		if idx := strings.Index(lower, kw); idx > 0 {
			title = strings.TrimSpace(title[:idx])
			break
		}
	}

	title = strings.Trim(title, " -:.,")

	if len(title) > 120 {
		title = title[:120] + "..."
	}
	return title
}

// extractProductNames matndan mahsulot nomlarini chiqarib oladi (nomlari bilan qisqa ro'yxat uchun)
func extractProductNames(text string) []string {
	var names []string
	seen := make(map[string]struct{})
	for _, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if strings.Contains(strings.ToLower(t), "jami") {
			continue
		}
		norm := normalizeCartTitleCandidate(t)
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		names = append(names, norm)
	}
	return names
}

func containsThumbsUp(text string) bool {
	return strings.Contains(text, "👍")
}

func isAffirmativeShort(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	lower = strings.Trim(lower, ".!?,:;")
	if lower == "" {
		return false
	}
	affirmative := map[string]struct{}{
		"ha": {}, "xa": {}, "ха": {}, "да": {}, "yes": {}, "ok": {}, "okay": {}, "albatta": {}, "mayli": {},
	}
	if _, ok := affirmative[lower]; ok {
		return true
	}
	prefixes := []string{"ha ", "xa ", "ха ", "да ", "yes ", "ok ", "okay ", "albatta ", "mayli "}
	for _, pref := range prefixes {
		if strings.HasPrefix(lower, pref) {
			tail := strings.TrimSpace(strings.Trim(lower[len(pref):], ".!?,:;"))
			switch tail {
			case "kerak", "bo'ladi", "boladi", "to'g'ri", "togri", "qabul":
				return true
			}
		}
	}
	return false
}

// isConfigLikeResponse - to'liq PC konfiguratsiya matnini aniqlash (single item emas).
func isConfigLikeResponse(text string) bool {
	lower := strings.ToLower(text)

	if strings.Contains(lower, "pc konfiguratsiya") ||
		strings.Contains(lower, "pc конфигурация") ||
		strings.Contains(lower, "overall price") {
		return true
	}

	// Komponent kalit so'zlari 3 tadan ko'p uchrasa, konfiguratsiya deb hisoblaymiz.
	componentKeywords := []string{"cpu:", "gpu:", "ram:", "ssd:", "psu:", "cooler:", "case:", "monitor:"}
	count := 0
	for _, kw := range componentKeywords {
		if strings.Contains(lower, kw) {
			count++
		}
	}
	return count >= 3
}

func isBackCommand(text string) bool {
	trim := strings.TrimSpace(text)
	if trim == "" {
		return false
	}
	lower := strings.ToLower(trim)
	lower = strings.TrimPrefix(lower, "/")
	return strings.Contains(lower, "orqaga") || strings.Contains(lower, "назад") || lower == "back"
}

// isProductModelMention - matnda aniq model (CPU/GPU/umumiy) borligini tekshiradi
func isProductModelMention(text string) bool {
	if cpuModelRegex.MatchString(text) || gpuModelRegex.MatchString(text) || genericModelRegex.MatchString(text) {
		return true
	}
	return false
}

// componentLineFromConfig - konfiguratsiya matnidan kerakli komponent qatorini ajratib oladi.
// key: cpu/gpu/ram/ssd/motherboard/psu/case/cooler/monitor
func componentLineFromConfig(configText, key string) string {
	lines := strings.Split(configText, "\n")
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		lower := strings.ToLower(strings.TrimLeft(t, "*-• "))
		// Breakdown va jami qatorlarini o'tkazib yuboramiz
		if strings.HasPrefix(lower, "-case components") ||
			strings.HasPrefix(lower, "-monitor") ||
			strings.HasPrefix(lower, "-peripherals") ||
			strings.HasPrefix(lower, "price:") ||
			strings.HasPrefix(lower, "overall price") ||
			strings.HasPrefix(lower, "jami") {
			continue
		}
		colonIdx := strings.Index(lower, ":")
		if colonIdx == -1 {
			continue
		}
		label := strings.TrimSpace(lower[:colonIdx])
		if matchesComponentLabel(label, key) {
			return t
		}
	}
	return ""
}

func matchesComponentLabel(label, key string) bool {
	switch key {
	case "cooler":
		return strings.Contains(label, "cooler") || strings.Contains(label, "sovutgich") || strings.Contains(label, "охлаж")
	case "cpu":
		return strings.Contains(label, "cpu") && !strings.Contains(label, "cooler")
	case "gpu":
		return strings.Contains(label, "gpu") || strings.Contains(label, "video") || strings.Contains(label, "videokarta")
	case "ram":
		return strings.Contains(label, "ram") || strings.Contains(label, "operativ")
	case "ssd":
		return strings.Contains(label, "ssd") || strings.Contains(label, "hdd") || strings.Contains(label, "nvme")
	case "motherboard":
		return strings.Contains(label, "motherboard") || strings.Contains(label, "anakart") || strings.Contains(label, "mobo")
	case "psu":
		return strings.Contains(label, "psu") || strings.Contains(label, "power") || strings.Contains(label, "quvvat")
	case "case":
		return strings.Contains(label, "case") || strings.Contains(label, "korpus") || strings.Contains(label, "корпус")
	case "monitor":
		return strings.Contains(label, "monitor")
	default:
		return false
	}
}

func (h *BotHandler) handleCartAddCallback(chatID, userID int64, offerID string, msg *tgbotapi.Message) {
	lang := h.getUserLang(userID)
	info, ok := h.getFeedbackByID(offerID)
	if !ok || strings.TrimSpace(info.ConfigText) == "" {
		if last, ok2 := h.getLastSuggestion(userID); ok2 && strings.TrimSpace(last) != "" {
			info = feedbackInfo{
				ConfigText: last,
			}
			ok = true
		}
	}
	if !ok {
		h.sendMessage(chatID, t(lang, "❌ Mahsulot ma'lumotlari topilmadi.", "❌ Данные товара не найдены."))
		return
	}

	title := cartTitleFromText(info.ConfigText)
	if title == "" {
		title = t(lang, "Mahsulot", "Товар")
	}
	h.addToCart(userID, cartItem{Title: title, Text: info.ConfigText})
	if msg != nil {
		tag := t(lang,
			fmt.Sprintf("✅ Savatga qo'shildi: %s", title),
			fmt.Sprintf("✅ Добавлено в корзину: %s", title),
		)
		tag += "\n" + t(lang, "🛒 Savatdagi mahsulotlaringizni ko'rish uchun savatcha tugmasini bosing.", "🛒 Нажмите на корзину, чтобы увидеть товары.")
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, tag, tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🛒 Savatcha", "cart_open"),
			),
		))
		if _, err := h.bot.Send(edit); err != nil {
			log.Printf("Cart add edit failed: %v", err)
		}
	}
}

func (h *BotHandler) handleCartCheckoutCallback(chatID, userID int64, offerID string) {
	lang := h.getUserLang(userID)
	idx, err := strconv.Atoi(offerID)
	if err != nil {
		h.sendMessage(chatID, t(lang, "❌ Noto'g'ri savat indeksi.", "❌ Неверный индекс корзины."))
		return
	}
	items := h.listCart(userID)
	if idx < 0 || idx >= len(items) {
		h.sendMessage(chatID, t(lang, "❌ Savatdagi mahsulot topilmadi.", "❌ Товар в корзине не найден."))
		return
	}
	text := strings.TrimSpace(items[idx].Text)
	if text == "" {
		text = items[idx].Title
	}
	h.startOrderSession(userID, pendingApproval{
		UserID:   userID,
		UserChat: chatID,
		Summary:  text,
		Config:   "",
		Username: "",
		SentAt:   time.Now(),
	})
	h.clearCart(userID)
	h.sendOrderForm(userID, "", nil)
}

func (h *BotHandler) handleCartClearCallback(chatID, userID int64, msg *tgbotapi.Message) {
	items := h.listCart(userID)
	lang := h.getUserLang(userID)
	if len(items) == 0 {
		h.sendMessage(chatID, t(lang, "🛒 Savatingiz bo'sh.", "🛒 Корзина пуста."))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for i, it := range items {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("❌ %d) %s", i+1, it.Title), fmt.Sprintf("cart_del|%d", i)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "cart_back"),
	))
	text := t(lang, "Qaysi mahsulotni o'chiramiz?", "Какой товар удалить?")
	if msg != nil {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows})
		if _, err := h.bot.Send(edit); err != nil {
			log.Printf("Cart clear edit failed: %v", err)
		}
	} else {
		newMsg := tgbotapi.NewMessage(chatID, text)
		newMsg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
		_, _ = h.sendAndLog(newMsg)
	}
}

func (h *BotHandler) handleCartDeleteCallback(chatID, userID int64, offerID string, msg *tgbotapi.Message) {
	lang := h.getUserLang(userID)
	idx, err := strconv.Atoi(offerID)
	if err != nil {
		h.sendMessage(chatID, t(lang, "❌ Noto'g'ri indeks.", "❌ Неверный индекс."))
		return
	}
	if !h.removeCartItem(userID, idx) {
		h.sendMessage(chatID, t(lang, "❌ Savatdan o'chirib bo'lmadi.", "❌ Не удалось удалить из корзины."))
		return
	}
	if msg != nil {
		h.handleCartOpenCallback(chatID, userID, msg)
	} else {
		h.handleCartCommand(chatID, userID)
	}
}

func (h *BotHandler) handleCartClearAllCallback(chatID, userID int64, msg *tgbotapi.Message) {
	h.clearCart(userID)
	lang := h.getUserLang(userID)
	text := t(lang, "♻️ Savat tozalandi.", "♻️ Корзина очищена.")
	if msg != nil {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
		if _, err := h.bot.Send(edit); err != nil {
			log.Printf("Cart clear all edit failed: %v", err)
			h.sendMessage(chatID, text)
		}
	} else {
		h.sendMessage(chatID, text)
	}
}

func (h *BotHandler) handleCartCommand(chatID, userID int64) {
	items := h.listCart(userID)
	lang := h.getUserLang(userID)
	if len(items) == 0 {
		h.sendMessage(chatID, t(lang, "🛒 Savatingiz bo'sh. Mahsulot yonidagi \"Savatga qo'shish\" tugmasini bosing.", "🛒 Ваша корзина пуста. Нажмите \"Добавить в корзину\" возле товара."))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for i, it := range items {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d) %s", i+1, it.Title), fmt.Sprintf("cart_checkout|%d", i)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "🗑️ Tanlab o'chirish", "🗑️ Удалить выборочно"), "cart_clear"),
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "♻️ Hammasini tozalash", "♻️ Очистить всё"), "cart_clear_all"),
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "🛍️ Barchasini rasmiylashtirish", "🛍️ Оформить всё"), "cart_checkout_all"),
	))
	msg := tgbotapi.NewMessage(chatID, t(lang, "🛒 Savatdagi mahsulotlar:", "🛒 Товары в корзине:"))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	_, _ = h.sendAndLog(msg)
}

// Savatcha tugmasi bosilganda shu xabarni savat ko'rinishiga edit qilish
func (h *BotHandler) handleCartOpenCallback(chatID, userID int64, msg *tgbotapi.Message) {
	lang := h.getUserLang(userID)
	items := h.listCart(userID)
	if len(items) == 0 {
		text := t(lang, "🛒 Savatingiz bo'sh.", "🛒 Корзина пуста.")
		if msg != nil {
			edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
			_, _ = h.sendAndLog(edit)
		} else {
			h.sendMessage(chatID, text)
		}
		return
	}

	rows := [][]tgbotapi.InlineKeyboardButton{}
	for i, it := range items {
		title := cartTitleFromText(it.Text)
		if len(title) > 50 {
			title = title[:50] + "…"
		}
		btnText := fmt.Sprintf("🛒 %d) %s", i+1, title)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("cart_checkout|%d", i)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "🧹 Tanlab o'chirish", "🧹 Удалить выборочно"), "cart_clear"),
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "♻️ Hammasini tozalash", "♻️ Очистить всё"), "cart_clear_all"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "🛒 Barchasini rasmiylashtirish", "🛒 Оформить всё"), "cart_checkout_all"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "⬅️ Orqaga", "⬅️ Назад"), "cart_back"),
	))

	text := t(lang, "🛒 Savatdagi mahsulotlar (quyidagi tugmalardan tanlang):", "🛒 Товары в корзине (выберите кнопкой ниже):")
	if msg != nil {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows})
		if _, err := h.bot.Send(edit); err != nil {
			log.Printf("Cart open edit failed: %v", err)
			h.sendMessage(chatID, text)
		}
	} else {
		m := tgbotapi.NewMessage(chatID, text)
		m.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
		_, _ = h.sendAndLog(m)
	}
}

// Savatchadan orqaga: savatga qo'shildi xabariga qaytamiz (edit)
func (h *BotHandler) handleCartBack(chatID, userID int64, msg *tgbotapi.Message) {
	lang := h.getUserLang(userID)
	items := h.listCart(userID)
	if len(items) == 0 {
		text := t(lang, "🛒 Savatingiz bo'sh.", "🛒 Корзина пуста.")
		if msg != nil {
			edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
			_, _ = h.sendAndLog(edit)
		} else {
			h.sendMessage(chatID, text)
		}
		return
	}

	title := cartTitleFromText(items[0].Text)
	if title == "" {
		title = t(lang, "Mahsulot", "Товар")
	}
	text := t(lang,
		fmt.Sprintf("✅ Savatga qo'shildi: %s\n🛒 Savatdagi mahsulotlaringizni ko'rish uchun savatcha tugmasini bosing.", title),
		fmt.Sprintf("✅ Добавлено в корзину: %s\n🛒 Нажмите на корзину, чтобы увидеть товары.", title),
	)
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛒 Savatcha", "cart_open"),
		),
	)
	if msg != nil {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msg.MessageID, text, markup)
		if _, err := h.bot.Send(edit); err != nil {
			log.Printf("Cart back edit failed: %v", err)
		}
	} else {
		m := tgbotapi.NewMessage(chatID, text)
		m.ReplyMarkup = markup
		_, _ = h.sendAndLog(m)
	}
}

// Purchase flows
func isPurchaseIntent(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"sotib olaman",
		"sotib olishni xohlayman",
		"sotib olmoqchiman",
		"buyurtma", "zakaz",
		"rasmiylashtir", "rasmiylashtiring",
		"olmoqchiman", "olayman", "olaman", "olmoq", "olishim kerak",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (h *BotHandler) handlePurchaseIntentPrompt(ctx context.Context, userID int64, username string, chatID int64, userText string) {
	_ = ctx
	lang := h.getUserLang(userID)

	if h.isProcessing(userID) {
		h.sendMessage(chatID, t(lang, "⏳ Oldingi so'rov yakunlanmoqda, iltimos kuting.", "⏳ Предыдущий запрос обрабатывается, подождите."))
		return
	}

	suggestion := ""
	products := []string{}
	if last, ok := h.getLastSuggestion(userID); ok && strings.TrimSpace(last) != "" {
		suggestion = last
		products = extractProductNames(last)
	}

	if suggestion == "" {
		h.sendMessage(chatID, t(lang, "Qaysi mahsulotni sotib olmoqchisiz? Model yoki to'liq nomini yozing.", "Какой товар хотите купить? Напишите модель или полное название."))
		return
	}

	// Agar oxirgi taklif to'liq konfiguratsiya bo'lsa, user nimani sotib olmoqchi ekanini aniqlaymiz.
	if isConfigLikeResponse(suggestion) {
		if key := detectComponentKeyword(userText); key != "" {
			if line := componentLineFromConfig(suggestion, key); line != "" {
				h.startOrderSession(userID, pendingApproval{
					UserID:   userID,
					UserChat: chatID,
					Summary:  line,
					Config:   "",
					Username: username,
					SentAt:   time.Now(),
				})
				h.sendOrderForm(userID, "", nil)
				return
			}
		}
		h.sendMessage(chatID, t(lang,
			"Bu to'liq konfiguratsiya. Qaysi bitta komponent/mahsulotni olmoqchisiz? Modelini yoki CPU/GPU/RAM kabi nomini yozing.",
			"Это полная конфигурация. Какой конкретный компонент/товар хотите купить? Напишите модель или тип (CPU/GPU/RAM и т.д.)."))
		return
	}

	title := cartTitleFromText(suggestion)
	if len(title) > 80 {
		title = title[:80] + "..."
	}

	var prompt string
	if len(products) > 0 {
		prompt = t(lang,
			fmt.Sprintf("Quyidagi mahsulotni sotib olmoqchimisiz?\n• %s", strings.Join(products, "\n• ")),
			fmt.Sprintf("Хотите купить эти товары?\n• %s", strings.Join(products, "\n• ")),
		)
	} else {
		prompt = t(lang,
			fmt.Sprintf("Quyidagi taklifni sotib olmoqchimisiz?\n%s", suggestion),
			fmt.Sprintf("Хотите купить подборку?\n%s", suggestion),
		)
	}

	offerID := newUUID()
	if !isSingleProductSuggestion(suggestion) {
		h.sendMessage(chatID, prompt)
		return
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "✅ Ha", "✅ Да"), "purchase_yes|"+offerID),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "❌ Yo'q", "❌ Нет"), "purchase_no"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🛒 Savatga qo'shish", "🛒 Добавить в корзину"), "cart_add|"+offerID),
		),
	)

	h.saveFeedback(userID, feedbackInfo{
		OfferID:    offerID,
		Summary:    title,
		ConfigText: suggestion,
		Username:   username,
		Phone:      "",
		ChatID:     chatID,
	})

	msg := tgbotapi.NewMessage(chatID, prompt)
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err != nil {
		log.Printf("Purchase intent prompt send error: %v", err)
	} else {
		h.setPurchasePromptMessage(userID, chatID, sent.MessageID)
	}
}

// shouldSendPurchasePrompt - bir xil taklif uchun tugmalarni takrorlamaslik
func (h *BotHandler) shouldSendPurchasePrompt(userID int64, suggestion string) bool {
	trim := strings.TrimSpace(suggestion)
	if trim == "" {
		return false
	}
	title := cartTitleFromText(trim)
	h.purchaseMu.Lock()
	defer h.purchaseMu.Unlock()
	// Agar aynan shu matn yoki shu title uchun allaqachon tugma yuborilgan bo'lsa, takrorlamaymiz
	if prev, ok := h.purchasePrompt[userID]; ok && prev == trim {
		log.Printf("🔍 [shouldSendPurchasePrompt] UserID=%d, Skipping - same text as before", userID)
		return false
	}
	if prevTitle, ok := h.purchaseTitle[userID]; ok && prevTitle != "" && prevTitle == title {
		log.Printf("🔍 [shouldSendPurchasePrompt] UserID=%d, Skipping - same title: %q (prev: %q)", userID, title, prevTitle)
		return false
	}
	h.purchasePrompt[userID] = trim
	h.purchaseTitle[userID] = title
	return true
}

// sendPurchaseConfirmationButtons sotib olish tugmalarini yuborish
func (h *BotHandler) sendPurchaseConfirmationButtons(chatID, userID int64, suggestion, offerID string) {
	lang := h.getUserLang(userID)
	title := cartTitleFromText(suggestion)
	if title == "" {
		title = t(lang, "Taklif", "Подбор")
	}
	if isConfigLikeResponse(suggestion) {
		log.Printf("⚠️ [Purchase Buttons] Skipping - config-like response")
		return
	}
	if !isSingleProductSuggestion(suggestion) {
		log.Printf("⚠️ [Purchase Buttons] Skipping - not a single product suggestion")
		return
	}
	if !h.shouldSendPurchasePrompt(userID, suggestion) {
		log.Printf("⚠️ [Purchase Buttons] Skipping - shouldSendPurchasePrompt returned false")
		return
	}
	log.Printf("📤 [Purchase Buttons] Sending buttons to userID=%d", userID)
	text := t(lang,
		fmt.Sprintf("Sotib olamizmi? %s", title),
		fmt.Sprintf("Оформляем покупку? %s", title),
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "✅ Zakaz beraman", "✅ Оформить"), "purchase_yes|"+offerID),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🛒 Savatga qo'shish", "🛒 В корзину"), "cart_add|"+offerID),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "❌ Bekor qilish", "❌ Отмена"), "purchase_no"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if sent, err := h.sendAndLog(msg); err != nil {
		log.Printf("Purchase tugmalarini yuborishda xatolik: %v", err)
	} else {
		h.setPurchasePromptMessage(userID, chatID, sent.MessageID)
	}
}

// Purchase YES (order flow) + cart add
func (h *BotHandler) handlePurchaseYes(ctx context.Context, userID int64, username string, chatID int64, offerID string, cq *tgbotapi.CallbackQuery) {
	_ = ctx
	if h.hasOrderSession(userID) {
		lang := h.getUserLang(userID)
		h.sendMessage(chatID, t(lang, "🧾 Buyurtma jarayoni allaqachon boshlangan. Iltimos, avvalgi buyurtmani yakunlang.", "🧾 Оформление уже начато. Пожалуйста, завершите текущий заказ."))
		return
	}
	var editChatID int64
	var editMessageID int
	if cq != nil && cq.Message != nil && cq.Message.Chat != nil {
		editChatID = cq.Message.Chat.ID
		editMessageID = cq.Message.MessageID
	}
	if msg, ok := h.popPurchasePromptMessage(userID); ok {
		if editMessageID == 0 {
			editChatID = msg.chatID
			editMessageID = msg.messageID
		}
	}
	if editChatID != 0 && editMessageID != 0 {
		_ = h.clearInlineButtonsByMessage(editChatID, editMessageID, "")
	} else if cq != nil {
		_ = h.clearInlineButtons(cq)
	}
	// Config session'ni yopish (agar ochiq bo'lsa)
	// Chunki user "Zakaz beraman" bosganda order oqimiga o'tadi
	h.configMu.Lock()
	if _, exists := h.configSessions[userID]; exists {
		delete(h.configSessions, userID)
		log.Printf("[handlePurchaseYes] Config session yopildi: userID=%d", userID)
	}
	h.configMu.Unlock()

	// AVVAL cart itemlarni tekshiramiz (faqat "Savatga qo'shish" bosilgan itemlar)
	cartItems := h.listCart(userID)
	var orderText string
	var orderUsername string

	if len(cartItems) > 0 {
		// Cart mavjud - faqat cartdagi itemlarni ishlatamiz
		log.Printf("📦 Cart items topildi: userID=%d, count=%d", userID, len(cartItems))
		var items []string
		for _, item := range cartItems {
			// Title + to'liq text (narx va spetsifikatsiya bilan)
			items = append(items, item.Text)
		}
		orderText = strings.Join(items, "\n\n") // Har bir item yangi qatorda
		orderUsername = username
	} else {
		// Cart bo'sh - feedback/lastSuggestion dan olamiz (bitta item uchun)
		log.Printf("🔍 Cart bo'sh, feedback dan olinmoqda: userID=%d", userID)
		info, ok := h.getFeedbackByID(offerID)
		if !ok {
			if fb, okFb := h.getFeedback(userID); okFb {
				info = fb
				ok = true
			} else if last, okLast := h.getLastSuggestion(userID); okLast {
				info = feedbackInfo{
					ConfigText: last,
					Username:   username,
				}
				ok = true
			}
		}
		if !ok {
			h.sendMessage(chatID, "❌ Mahsulot ma'lumotlari topilmadi.")
			return
		}
		orderText = info.ConfigText
		orderUsername = info.Username
	}

	// Topic 6 approval o'chirildi - to'g'ridan Topic 8 (Active Orders) ga yuboriladi

	h.startOrderSession(userID, pendingApproval{
		UserID:   userID,
		UserChat: chatID,
		Summary:  orderText,
		Config:   "", // Savatchadan kelgan orderlar uchun Config bo'sh (faqat /configuratsiya flow uchun)
		Username: orderUsername,
		SentAt:   time.Now(),
	})
	if editChatID != 0 && editMessageID != 0 {
		h.setOrderMessageID(userID, editMessageID)
	}
	h.sendOrderForm(userID, "", nil)
	if editChatID != 0 && editMessageID != 0 {
		h.orderMu.RLock()
		sess, ok := h.orderSessions[userID]
		h.orderMu.RUnlock()
		if ok && sess != nil {
			switch sess.Stage {
			case orderStageNeedPhone:
				kb := h.phoneRequestKeyboard(chatID)
				h.showReplyKeyboard(chatID, kb)
			case orderStageNeedLocation:
				kb := h.locationRequestKeyboard(chatID)
				h.showReplyKeyboard(chatID, kb)
			}
		}
	}
}
