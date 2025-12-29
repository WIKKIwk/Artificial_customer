package telegram

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/usecase"
)

// handleAnalyzePCCallback "Analyze PC" button bosilganda
func (h *BotHandler) handleAnalyzePCCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	lang := h.getUserLang(userID)

	// Callback javob (button endi yuklanayotganini ko'rsatish)
	h.bot.Request(tgbotapi.NewCallback(callback.ID, t(lang, "Tahlil qilinmoqda...", "Анализируется...")))

	// PC build ni olish (bu yerda siz qanday qilib build ni saqlab qo'yganingizga qarab)
	// Misol uchun, oxirgi konfiguratsiyani chat history'dan olish mumkin
	build, err := h.extractPCBuildFromHistory(ctx, userID)
	if err != nil {
		h.sendMessage(callback.Message.Chat.ID, t(lang,
			"❌ Konfiguratsiya topilmadi. Avval /configuratsiya orqali PC yig'ing!",
			"❌ Конфигурация не найдена. Сначала соберите ПК через /configuratsiya!"))
		return
	}

	// Progress message
	progressMsg, _ := h.sendMessageWithResp(callback.Message.Chat.ID, t(lang,
		"⏳ PC tahlil qilinmoqda...\n\n🔍 FPS hisoblanmoqda...\n🌡️ Temperatura aniqlanmoqda...\n⚖️ Bottleneck tekshirilmoqda...\n⚡ Quvvat hisoblanmoqda...",
		"⏳ Анализ ПК...\n\n🔍 Расчет FPS...\n🌡️ Определение температуры...\n⚖️ Проверка узких мест...\n⚡ Расчет мощности..."))

	// PC Analyzer yaratish
	analyzer := usecase.NewPCAnalyzer(h.chatUseCase)

	// Tahlil qilish
	analytics, err := analyzer.AnalyzePC(ctx, build, lang)
	if err != nil {
		log.Printf("❌ PC tahlil xatosi: %v", err)
		h.sendMessage(callback.Message.Chat.ID, t(lang,
			"❌ Tahlil qilishda xatolik yuz berdi. Iltimos, qayta urinib ko'ring.",
			"❌ Ошибка при анализе. Попробуйте еще раз."))
		return
	}

	// Progress message'ni o'chirish
	if progressMsg != nil {
		h.deleteMessage(callback.Message.Chat.ID, progressMsg.MessageID)
	}

	// Analytics ni formatlash
	formattedAnalytics := FormatPCAnalytics(build, analytics, lang)

	// Yuborish
	h.sendMessage(callback.Message.Chat.ID, formattedAnalytics)

	if h.isConfigOrderLocked(userID) {
		return
	}

	// Qo'shimcha button'lar
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				t(lang, "📊 PDF Hisobot", "📊 PDF Отчет"),
				"download_pdf_report",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				t(lang, "🛒 Sotib olish", "🛒 Купить"),
				"purchase_config",
			),
			tgbotapi.NewInlineKeyboardButtonData(
				t(lang, "🔄 Boshqa variant", "🔄 Другой вариант"),
				"new_config",
			),
		),
	)

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, t(lang,
		"📊 Tahlil tugadi! Yuqoridagi ma'lumotlarni ko'rib chiqing.\n\nSavol bo'lsa menga yozishingiz mumkin, agar zarur bo'lsa @Ingame_support ga murojaat qiling! 😊",
		"📊 Анализ завершен! Просмотрите информацию выше.\n\nЕсли есть вопросы, можете написать мне, если нужно - обращайтесь к @Ingame_support! 😊"))
	msg.ReplyMarkup = keyboard
	_, _ = h.sendAndLog(msg)
}

// extractPCBuildFromHistory chat history'dan PC build ni extract qilish
func (h *BotHandler) extractPCBuildFromHistory(ctx context.Context, userID int64) (*entity.PCBuild, error) {
	// Chat history'ni olish (oxirgi 20 ta xabar)
	history, err := h.chatUseCase.GetHistory(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return nil, err
	}

	// Oxirgi PC konfiguratsiyani topish (AI response'larni ko'r)
	var configText string
	for i := len(history) - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(history[i].Response), "cpu:") &&
			strings.Contains(strings.ToLower(history[i].Response), "gpu:") {
			configText = history[i].Response
			break
		}
	}

	if configText == "" {
		return nil, err
	}

	// Build entity'ni yaratish
	build := &entity.PCBuild{
		UserID:      userID,
		Purpose:     normalizePurposeLabel(extractField(configText, "maqsad|purpose|назначение", "")),
		ColorScheme: extractField(configText, "rang|color|цвет", "RGB"),
	}
	if strings.TrimSpace(build.Purpose) == "" {
		build.Purpose = "Gaming"
	}
	if strings.TrimSpace(build.ColorScheme) == "" {
		build.ColorScheme = "RGB"
	}

	// Komponentlarni parse qilish
	if cpu := parseComponent(configText, "cpu|processor", "CPU"); cpu != nil {
		build.CPU = *cpu
	}
	if ram := parseComponent(configText, "ram|operativ", "RAM"); ram != nil {
		build.RAM = *ram
	}
	if gpu := parseComponent(configText, "gpu|videokarta|video", "GPU"); gpu != nil {
		build.GPU = *gpu
	}
	if ssd := parseComponent(configText, "ssd|nvme|hdd", "SSD"); ssd != nil {
		build.SSD = *ssd
	}
	if mb := parseComponent(configText, "motherboard|anakart|mobo", "Motherboard"); mb != nil {
		build.Motherboard = *mb
	}
	if psu := parseComponent(configText, "psu|power supply|quvvat", "PSU"); psu != nil {
		build.PSU = *psu
	}
	// Pointer fields uchun
	build.Case = parseComponent(configText, "case|korpus|корпус", "Case")
	build.Cooler = parseComponent(configText, "cooler|cooling|sovutgich|охлажде", "Cooler")

	return build, nil
}

// parseComponent konfiguratsiyadan komponent parse qilish
func parseComponent(configText, pattern, componentType string) *entity.Product {
	// Pattern bilan qatorni topish
	re := regexp.MustCompile(`(?i)•\s*` + pattern + `[^•]*?-\s*([\d,.]+)\$`)
	matches := re.FindStringSubmatch(configText)

	if len(matches) < 2 {
		return nil
	}

	// Komponent nomi olish (qator boshiga qarab)
	lineRe := regexp.MustCompile(`(?i)•\s*` + pattern + `[^:]*?:\s*([^-]+)-`)
	lineMatches := lineRe.FindStringSubmatch(configText)
	var name string
	if len(lineMatches) > 1 {
		name = strings.TrimSpace(lineMatches[1])
	} else {
		name = componentType
	}

	// Narx olish
	priceStr := strings.TrimSpace(matches[1])
	priceStr = strings.ReplaceAll(priceStr, ",", ".")
	price := 0.0
	if parsed, err := strconv.ParseFloat(priceStr, 64); err == nil {
		price = parsed
	}

	return &entity.Product{
		Name:  name,
		Price: price,
	}
}

// extractField konfiguratsiyadan maydan qiymatini extract qilish
func extractField(text, pattern, defaultValue string) string {
	re := regexp.MustCompile(`(?i)(` + pattern + `)[:\s]+([^\n•-]+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 2 {
		return strings.TrimSpace(matches[2])
	}
	return defaultValue
}

func normalizePurposeLabel(purpose string) string {
	lower := strings.ToLower(strings.TrimSpace(purpose))
	if lower == "" {
		return ""
	}
	switch {
	case strings.Contains(lower, "gaming") || strings.Contains(lower, "o'yin") || strings.Contains(lower, "oʻyin") ||
		strings.Contains(lower, "o‘yin") || strings.Contains(lower, "игр"):
		return "Gaming"
	case strings.Contains(lower, "developer") || strings.Contains(lower, "dev") || strings.Contains(lower, "dasturchi") ||
		strings.Contains(lower, "program") || strings.Contains(lower, "coding") || strings.Contains(lower, "backend"):
		return "Developer"
	case strings.Contains(lower, "design") || strings.Contains(lower, "designer") || strings.Contains(lower, "dizayn") ||
		strings.Contains(lower, "montaj") || strings.Contains(lower, "editing") || strings.Contains(lower, "render"):
		return "Design"
	case strings.Contains(lower, "server") || strings.Contains(lower, "сервер") || strings.Contains(lower, "hosting") ||
		strings.Contains(lower, "vps"):
		return "Server"
	case strings.Contains(lower, "office") || strings.Contains(lower, "офис") || strings.Contains(lower, "work"):
		return "Office"
	default:
		return strings.Title(lower)
	}
}

// handlePCAnalysisRequest PC tahlil so'rovini qayta ishlash (cfg_analyze_pc tugmasidan)
func (h *BotHandler) handlePCAnalysisRequest(ctx context.Context, userID int64, username string, chatID int64, configText string, purposeHint string) {
	_ = username
	lang := h.getUserLang(userID)

	// Progress message
	progressMsg, _ := h.sendMessageWithResp(chatID, t(lang,
		"⏳ PC tahlil qilinmoqda...\n\n🔍 FPS hisoblanmoqda...\n🌡️ Temperatura aniqlanmoqda...\n⚖️ Bottleneck tekshirilmoqda...\n⚡ Quvvat hisoblanmoqda...",
		"⏳ Анализ ПК...\n\n🔍 Расчет FPS...\n🌡️ Определение температуры...\n⚖️ Проверка узких мест...\n⚡ Расчет мощности..."))

	// PC build'ni config text'dan extract qilish
	build := h.extractPCBuildFromText(userID, configText, purposeHint)
	if build == nil {
		if progressMsg != nil {
			h.deleteMessage(chatID, progressMsg.MessageID)
		}
		h.sendMessage(chatID, t(lang,
			"❌ Konfiguratsiya tahlillanmadi. Iltimos, qayta urinib ko'ring.",
			"❌ Не удалось проанализировать конфигурацию. Пожалуйста, попробуйте еще раз."))
		return
	}

	// PC Analyzer yaratish va tahlil qilish
	analyzer := usecase.NewPCAnalyzer(h.chatUseCase)
	analytics, err := analyzer.AnalyzePC(ctx, build, lang)
	if err != nil {
		log.Printf("PC tahlil xatosi: %v", err)
		if progressMsg != nil {
			h.deleteMessage(chatID, progressMsg.MessageID)
		}
		h.sendMessage(chatID, t(lang,
			"❌ Tahlil qilishda xatolik yuz berdi. Iltimos, qayta urinib ko'ring.",
			"❌ Ошибка при анализе. Попробуйте еще раз."))
		return
	}

	// Progress message'ni o'chirish
	if progressMsg != nil {
		h.deleteMessage(chatID, progressMsg.MessageID)
	}

	// Analytics ni formatlash
	formattedAnalytics := FormatPCAnalytics(build, analytics, lang)

	// Yuborish
	h.sendMessage(chatID, formattedAnalytics)

	if h.isConfigOrderLocked(userID) {
		return
	}

	// Qo'shimcha tugmalar
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				t(lang, "✅ Zakaz beraman", "✅ Заказать"),
				"cfg_fb_yes|",
			),
			tgbotapi.NewInlineKeyboardButtonData(
				t(lang, "🔄 O'zgartirish", "🔄 Изменить"),
				"cfg_fb_change|",
			),
		),
	)

	msg := tgbotapi.NewMessage(chatID, t(lang,
		"📊 Tahlil tugadi! Yuqoridagi ma'lumotlarni ko'rib chiqing.\n\nSavol bo'lsa menga yozishingiz mumkin, agar zarur bo'lsa @Ingame_support ga murojaat qiling! 😊",
		"📊 Анализ завершен! Просмотрите информацию выше.\n\nЕсли есть вопросы, можете написать мне, если нужно - обращайтесь к @Ingame_support! 😊"))
	msg.ReplyMarkup = keyboard
	_, _ = h.sendAndLog(msg)
}

// extractPCBuildFromText config text'dan PC build ni extract qilish
// Format: "• CPU: INTEL CORE I5 13600KF - 250.00$"
func (h *BotHandler) extractPCBuildFromText(userID int64, configText string, purposeHint string) *entity.PCBuild {
	build := &entity.PCBuild{
		UserID:      userID,
		Purpose:     normalizePurposeLabel(extractField(configText, "maqsad|purpose|назначение", "")),
		ColorScheme: extractField(configText, "rang|color|цвет", "RGB"),
	}

	lines := strings.Split(configText, "\n")
	totalPrice := 0.0

	// Price regex
	priceRegex := regexp.MustCompile(`(\d+(?:\.\d{2})?)\$`)
	anyNumberRegex := regexp.MustCompile(`\d+(?:\.\d+)?`)

	parseLinePrice := func(line string, matches []string) (float64, bool) {
		if len(matches) > 1 {
			if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
				return p, true
			}
		}
		if raw := anyNumberRegex.FindString(line); raw != "" {
			if p, err := strconv.ParseFloat(raw, 64); err == nil {
				return p, true
			}
		}
		return 0, false
	}

	isTotalLine := func(lower string) bool {
		return strings.Contains(lower, "overall price") ||
			strings.Contains(lower, "total price") ||
			strings.Contains(lower, "umumiy") ||
			strings.Contains(lower, "jami") ||
			strings.Contains(lower, "итого") ||
			strings.Contains(lower, "общая") ||
			strings.Contains(lower, "summa")
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)

		// Extract price
		var price float64 = 0
		priceMatches := priceRegex.FindStringSubmatch(line)
		if len(priceMatches) > 0 {
			if p, err := strconv.ParseFloat(priceMatches[1], 64); err == nil {
				price = p
			}
		}

		if isTotalLine(lower) {
			if p, ok := parseLinePrice(line, priceMatches); ok {
				totalPrice = p
			}
			continue
		}

		// Parse format: "• CPU: NAME - PRICE$"
		// Extract component label and value
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			// Try Jami:
			if strings.Contains(lower, "jami:") {
				if p, ok := parseLinePrice(line, priceMatches); ok {
					totalPrice = p
				}
			}
			continue
		}

		// Label is everything before ":"
		label := strings.TrimSpace(line[:colonIdx])
		label = strings.TrimPrefix(label, "•")
		label = strings.TrimPrefix(label, "*")
		label = strings.TrimSpace(label)
		labelLower := strings.ToLower(label)

		// Value is everything after ":" and before price
		valueStart := colonIdx + 1
		valueEnd := strings.LastIndex(line, "-")
		if valueEnd == -1 || valueEnd <= valueStart {
			continue
		}

		name := strings.TrimSpace(line[valueStart:valueEnd])
		if name == "" {
			continue
		}

		// Match component by LABEL, not by name keywords
		switch {
		case strings.Contains(labelLower, "cpu") && !strings.Contains(labelLower, "cooler"):
			build.CPU = entity.Product{Name: name, Price: price}
			log.Printf("🔧 [Parse] CPU: %s - %.2f$", name, price)

		case strings.Contains(labelLower, "cooler"):
			if build.Cooler == nil {
				build.Cooler = &entity.Product{}
			}
			build.Cooler.Name = name
			build.Cooler.Price = price
			log.Printf("🔧 [Parse] CPU Cooler: %s - %.2f$", name, price)

		case strings.Contains(labelLower, "ram"):
			build.RAM = entity.Product{Name: name, Price: price}
			log.Printf("🔧 [Parse] RAM: %s - %.2f$", name, price)

		case strings.Contains(labelLower, "gpu"):
			build.GPU = entity.Product{Name: name, Price: price}
			log.Printf("🔧 [Parse] GPU: %s - %.2f$", name, price)

		case strings.Contains(labelLower, "ssd"):
			build.SSD = entity.Product{Name: name, Price: price}
			log.Printf("🔧 [Parse] SSD: %s - %.2f$", name, price)

		case strings.Contains(labelLower, "motherboard"):
			build.Motherboard = entity.Product{Name: name, Price: price}
			log.Printf("🔧 [Parse] Motherboard: %s - %.2f$", name, price)

		case strings.Contains(labelLower, "psu"):
			build.PSU = entity.Product{Name: name, Price: price}
			log.Printf("🔧 [Parse] PSU: %s - %.2f$", name, price)

		case strings.Contains(labelLower, "case"):
			if build.Case == nil {
				build.Case = &entity.Product{}
			}
			build.Case.Name = name
			build.Case.Price = price
			log.Printf("🔧 [Parse] Case: %s - %.2f$", name, price)
		}
	}

	// If total not found, calculate it
	if totalPrice == 0 {
		totalPrice = build.CPU.Price + build.GPU.Price + build.RAM.Price +
			build.SSD.Price + build.Motherboard.Price + build.PSU.Price
		if build.Case != nil {
			totalPrice += build.Case.Price
		}
		if build.Cooler != nil && build.Cooler.Price > 0 {
			totalPrice += build.Cooler.Price
		}
	}

	build.Budget = totalPrice
	if strings.TrimSpace(build.Purpose) == "" && strings.TrimSpace(purposeHint) != "" {
		build.Purpose = normalizePurposeLabel(purposeHint)
	}
	if strings.TrimSpace(build.Purpose) == "" {
		build.Purpose = "Gaming"
	}
	if strings.TrimSpace(build.ColorScheme) == "" {
		build.ColorScheme = "RGB"
	}

	log.Printf("✅ [Parse Complete] Total: %.2f$, CPU: %s, GPU: %s", totalPrice, build.CPU.Name, build.GPU.Name)

	return build
}
