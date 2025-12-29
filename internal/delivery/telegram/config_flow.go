package telegram

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleConfigCommand konfiguratsiya sessiyasini boshlash
func (h *BotHandler) handleConfigCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	// Profile stage ni tozalash (config flow profile flowdan ustun bo'lishi kerak)
	h.setProfileStage(userID, "")

	h.startConfigWizard(userID, message.Chat.ID)
}

// startConfigSession yangi konfiguratsiya sessiyasini yaratish
func (h *BotHandler) startConfigSession(userID int64) {
	h.setConfigOrderLocked(userID, false)
	h.configMu.Lock()
	h.configSessions[userID] = &configSession{
		Stage:      configStageNeedName,
		StartedAt:  time.Now(),
		LastUpdate: time.Now(),
	}
	h.configMu.Unlock()
}

func (h *BotHandler) startConfigWizard(userID, chatID int64, msgID ...int) {
	h.startConfigSession(userID)
	mid := 0
	if len(msgID) > 0 {
		mid = msgID[0]
	}

	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		// Inline button mode - tez va qulay
		sess.Inline = true // Inline button rejimi
		sess.MessageID = mid
		sess.ChatID = chatID
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()

	// Birinchi savol: konfiguratsiyaga nom bering
	h.askConfigName(userID, chatID)
	// Eski CTA bekor qilinsin
	h.clearConfigCTA(chatID)
}

func (h *BotHandler) handleConfigFlow(ctx context.Context, userID int64, username, text string, chatID int64, msg *tgbotapi.Message) {
	input := strings.TrimSpace(text)
	lang := h.getUserLang(userID)

	h.configMu.Lock()
	session, ok := h.configSessions[userID]
	if ok {
		session.LastUpdate = time.Now()
		h.configSessions[userID] = session
	}
	h.configMu.Unlock()
	if !ok {
		h.sendMessage(chatID, t(lang, "Konfiguratsiya sessiyasi topilmadi. Boshlash uchun /configuratsiya ni bosing.", "Сессия конфигурации не найдена. Нажмите /configuratsiya чтобы начать."))
		return
	}

	log.Printf("[config] user=%d stage=%d inline=%v input=%q", userID, session.Stage, session.Inline, truncateForLog(input, 120))

	// Inline rejim
	if session.Inline {
		switch session.Stage {
		case configStageNeedName:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Konfiguratsiyaga nom bering (bo'sh bo'lmasin).", "Дайте имя конфигурации (не пустое)."))
				return
			}
			h.configMu.Lock()
			if sess, ok := h.configSessions[userID]; ok {
				sess.Name = input
				sess.Stage = configStageNeedType
				h.configSessions[userID] = sess
			}
			h.configMu.Unlock()
			h.askConfigType(userID, chatID)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedType:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, PC turi haqida yozing.", "Пожалуйста, укажите тип ПК."))
				return
			}
			h.setConfigTypeAndAskBudget(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedBudget:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, budjetni kiriting.", "Пожалуйста, укажите бюджет."))
				return
			}
			h.setConfigBudgetAndAskColor(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedColor:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, rangini kiriting.", "Пожалуйста, укажите цвет."))
				return
			}
			h.setConfigColorAndAskCPU(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedCPU:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, protsessor nomini kiriting.", "Пожалуйста, укажите название процессора."))
				return
			}
			h.setConfigCPUAndAskCPUCooler(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedCPUCooler:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, CPU sovutgichi nomini kiriting.", "Пожалуйста, укажите название процессорного охладителя."))
				return
			}
			h.setConfigCPUCoolerAndAskStorage(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedStorage:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, xotira turini kiriting.", "Пожалуйста, укажите тип накопителя."))
				return
			}
			h.setConfigStorageAndAskGPU(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedGPU:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, grafik karta nomini kiriting.", "Пожалуйста, укажите модель видеокарты."))
				return
			}
			h.deleteUserMessage(chatID, msg)
			h.setConfigGPUAndAskMonitor(userID, chatID, input)
			return
		case configStageNeedMonitorHz:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, monitor Hz qiymatini kiriting.", "Пожалуйста, укажите Hz монитора."))
				return
			}
			h.setConfigMonitorHzAndAskDisplay(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		case configStageNeedMonitorDisplay:
			if input == "" {
				h.sendMessage(chatID, t(lang, "Iltimos, display turini kiriting.", "Пожалуйста, укажите тип дисплея."))
				return
			}
			h.setConfigMonitorDisplayAndAskPeripherals(userID, chatID, input)
			h.deleteUserMessage(chatID, msg)
			return
		default:
			return
		}
	}

	switch session.Stage {
	case configStageNeedName:
		session.Name = strings.TrimSpace(input)
		if session.Name == "" {
			h.sendMessage(chatID, t(lang, "Konfiguratsiyaga nom bering (bo'sh bo'lmasin).", "Дайте имя конфигурации (не пустое)."))
			return
		}
		session.Stage = configStageNeedType
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.askConfigType(userID, chatID)
		h.deleteUserMessage(chatID, msg)
		return
	case configStageNeedType:
		session.PCType = input
		session.Stage = configStageNeedBudget
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.sendMessage(chatID, t(lang, "💰 Budjetni kiriting (masalan: 800$, 10 000 000 so'm). Aniq bo'lmasa, taxminiy yozing.", "💰 Укажите бюджет (например: 800$, 10 000 000 сум). Можно приблизительно."))
		return
	case configStageNeedBudget:
		session.Budget = input
		session.Stage = configStageNeedColor
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.clearConfigCTA(chatID)
		h.sendMessage(chatID, t(lang, "🎨 Rangni kiriting:", "🎨 Укажите цвет:"))
		return
	case configStageNeedColor:
		session.Color = input
		session.Stage = configStageNeedCPU
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.sendMessage(chatID, t(lang, "🧠 Qaysi protsessor turini xohlaysiz? Intel yoki AMD?", "🧠 Какой процессор предпочитаете? Intel или AMD?"))
		return
	case configStageNeedCPU:
		session.CPUBrand = input
		session.Stage = configStageNeedCPUCooler
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.sendMessage(chatID, t(lang, "❄️ CPU sovutgichini kiriting:", "❄️ Укажите процессорный охладитель:"))
		return
	case configStageNeedCPUCooler:
		session.CPUCooler = input
		session.Stage = configStageNeedStorage
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.sendMessage(chatID, t(lang, "💾 Xotira turi? HDD, SSD yoki NVMe?", "💾 Какой тип накопителя? HDD, SSD или NVMe?"))
		return
	case configStageNeedStorage:
		session.Storage = input
		session.Stage = configStageNeedGPU
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.sendMessage(chatID, t(lang, "🎮 Grafik karta: NVIDIA RTXmi yoki AMD Radeon?", "🎮 Видеокарта: NVIDIA RTX или AMD Radeon?"))
		return
	case configStageNeedGPU:
		session.GPUBrand = normalizeGPUSelection(input)
		session.Stage = configStageNeedMonitor
		h.configMu.Lock()
		h.configSessions[userID] = session
		h.configMu.Unlock()
		h.askConfigMonitor(userID, chatID)
		return
	default:
		h.configMu.Lock()
		delete(h.configSessions, userID)
		h.configMu.Unlock()
		h.sendMessage(chatID, t(lang, "Sessiya qayta ishga tushirildi. Yangi boshlash uchun /configuratsiya ni bosing.", "Сессия перезапущена. Для начала нажмите /configuratsiya."))
		return
	}
}

// finishConfigSession tanlangan parametrlar asosida AI javobi
func (h *BotHandler) finishConfigSession(ctx context.Context, userID int64, username string, chatID int64, session configSession) {
	// AI hisoblash vaqtida parallel so'rovlarni to'xtatish
	if !h.startProcessing(userID) {
		h.sendMessage(chatID, t(h.getUserLang(userID), "⏳ Oldingi so'rov yakunlanmoqda, iltimos kuting.", "⏳ Предыдущий запрос обрабатывается, подождите."))
		return
	}
	defer h.clearWaitingMessage(userID)
	defer h.endProcessing(userID)

	lang := h.getUserLang(userID)
	ko := t(lang, "ko'rsatilmagan", "не указано")

	var summary string
	monitorInfo := ""
	peripheralsInfo := ""
	gpuDisabled := isGPUDisabled(session.GPUBrand)
	gpuSummary := nonEmpty(session.GPUBrand, ko)
	if gpuDisabled {
		gpuSummary = t(lang, "Kerak emas", "Не нужно")
	}

	if session.NeedMonitor {
		if lang == "ru" {
			monitorInfo = fmt.Sprintf("\n• Монитор: Да (%s, %s)", session.MonitorHz, session.MonitorDisplay)
		} else {
			monitorInfo = fmt.Sprintf("\n• Monitor: Ha (%s, %s)", session.MonitorHz, session.MonitorDisplay)
		}
	} else {
		if lang == "ru" {
			monitorInfo = "\n• Монитор: Нет"
		} else {
			monitorInfo = "\n• Monitor: Yo'q"
		}
	}

	if session.NeedPeripherals {
		if lang == "ru" {
			peripheralsInfo = "\n• Периферия: Да"
		} else {
			peripheralsInfo = "\n• Peripherals: Ha"
		}
	} else {
		if lang == "ru" {
			peripheralsInfo = "\n• Периферия: Нет"
		} else {
			peripheralsInfo = "\n• Peripherals: Yo'q"
		}
	}

	if lang == "ru" {
		summary = fmt.Sprintf(`📝 Зафиксировал требования:
• Имя конфигурации: %s
• Назначение: %s
• Бюджет: %s
• Цвет: %s
• CPU: %s
• CPU Охладитель: %s
• Накопитель: %s
• GPU: %s%s%s

Сейчас подберу оптимальную конфигурацию...`,
			nonEmpty(session.Name, ko),
			nonEmpty(session.PCType, ko),
			nonEmpty(session.Budget, ko),
			nonEmpty(session.Color, ko),
			nonEmpty(session.CPUBrand, ko),
			nonEmpty(session.CPUCooler, ko),
			nonEmpty(session.Storage, ko),
			gpuSummary,
			monitorInfo,
			peripheralsInfo,
		)
	} else {
		summary = fmt.Sprintf(`📝 Talablaringizni yozib oldim:
• Konfiguratsiya nomi: %s
• Maqsad: %s
• Budjet: %s
• Rang: %s
• CPU: %s
• CPU Sovutgichi: %s
• Xotira: %s
• GPU: %s%s%s

Endi shu talablar bo'yicha optimal konfiguratsiyani tanlab beraman...`,
			nonEmpty(session.Name, ko),
			nonEmpty(session.PCType, ko),
			nonEmpty(session.Budget, ko),
			nonEmpty(session.Color, ko),
			nonEmpty(session.CPUBrand, ko),
			nonEmpty(session.CPUCooler, ko),
			nonEmpty(session.Storage, ko),
			gpuSummary,
			monitorInfo,
			peripheralsInfo,
		)
	}

	h.sendMessage(chatID, summary)
	waitMsg, err := h.sendMessageWithResp(chatID, t(lang, "⏳ Iltimos, javobni kuting.", "⏳ Пожалуйста, подождите."))
	if err == nil {
		h.setWaitingMessage(userID, chatID, waitMsg.MessageID)
	}

	// "typing" indikatori
	typingAction := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	h.bot.Send(typingAction)

	// AI ga yuboriladigan so'rov
	monitorPrompt := ""
	if session.NeedMonitor {
		monitorPrompt = fmt.Sprintf("Monitor: MAJBURIY KIRITILSIN! (%s, %s). Monitor modelni CSV dan ANIQ ol (masalan: DELL ALIENWARE, SAMSUNG ODYSSEY, LG UltraGear). ANIQ MODEL NOM BIZ LAZIM!\nQAT'IY FORMAT: • Monitor: [ANIQ MODEL NOM VA SPECS] - [Narx]$", session.MonitorHz, session.MonitorDisplay)
	} else {
		monitorPrompt = "Monitor: KERAK EMAS. Ro'yxatga umuman qo'shmang! Narxini hisoblamang."
	}

	peripheralsPrompt := ""
	if session.NeedPeripherals {
		peripheralsPrompt = "Peripherals: KERAK. MAJBURIY ravishda quyidagi 3 ta itemni aniq model va narx bilan ro'yxatga qo'sh:\n• Klaviatura (gaming/mekanik): [Model] - [Narx]$\n• Sichqoncha (gaming): [Model] - [Narx]$\n• Quloqchin (gaming headset): [Model] - [Narx]$"
	} else {
		peripheralsPrompt = "Peripherals: KERAK EMAS. Ro'yxatga umuman qo'shmang!"
	}

	budgetValue := parseBudgetUSD(session.Budget)
	minBudget := budgetValue - 100
	if minBudget < 0 {
		minBudget = 0
	}
	maxBudget := budgetValue + 100
	budgetPrompt := t(lang,
		"Jami narx budjetga maksimal yaqin bo'lsin. Budjetdan katta farq qoldirma (kerak bo'lsa +100$ gacha ruxsat).\n",
		"Итоговая цена должна быть максимально близка к бюджету. Большой остаток не оставляй (допускается +100$).\n",
	)
	if budgetValue > 0 {
		budgetPrompt = fmt.Sprintf(t(lang,
			"Jami narx %.0f$-%.0f$ oralig'ida bo'lsin. Budjetga maksimal yaqinlashtir (kerak bo'lsa +100$ gacha ruxsat).\n",
			"Итоговая цена должна быть в пределах %.0f$-%.0f$. Максимально приблизь к бюджету (допускается +100$).\n",
		), minBudget, maxBudget)
	}
	if gpuDisabled {
		budgetPrompt += t(lang,
			"GPU kerak emas bo'lsa, CPU/RAM/SSD/MB/PSU ni yuqori darajaga ko'tar va budjetni shu yerga sarfla.\n",
			"Если GPU не нужен, усили CPU/RAM/SSD/MB/PSU и расходуй бюджет на эти компоненты.\n",
		)
	}

	gpuPreference := nonEmpty(session.GPUBrand, "aniqlanmagan")
	gpuLine := "• GPU: [Model] - [Price]$\n"
	gpuPrompt := t(lang,
		"GPU: CSV dan mos modelni tanla. Agar brand ko'rsatilgan bo'lsa, shunga mos GPU tanla.\n",
		"GPU: Выбери модель из CSV. Если указан бренд, подбери соответствующую видеокарту.\n",
	)
	if gpuDisabled {
		gpuPreference = t(lang, "kerak emas", "не нужно")
		gpuLine = t(lang, "• GPU: Integrated Graphics (CPU) - 0$\n", "• GPU: Встроенная графика (CPU) - 0$\n")
		gpuPrompt = t(lang,
			"GPU: KERAK EMAS. Diskret GPU qo'shma. GPU qatorida faqat integrated graphics (CPU) va 0$ yoz.\n",
			"GPU: НЕ НУЖНО. Дискретную видеокарту не добавляй. В строке GPU укажи только встроенную графику (CPU) и 0$.\n",
		)
	}

	prompt := fmt.Sprintf(
		"Konfiguratsiya so'rovi: nom=%s, maqsad=%s, budjet=%s, rang=%s, CPU=%s, sovutgich=%s, xotira=%s, GPU=%s.\n\n"+
			"⚠️ ⚠️ KRITIK - BU TALABLARI BAJARISH SHART EMAS BU RESPONSE YAROQSIZ:\n\n"+
			"1️⃣ MONITOR MODELI:\n"+
			"   ❗ ALBATTA CSV dan monitor modeli tanlang\n"+
			"   ❗ Faqat narx emas, ANIQ MODEL kerak (masalan: DELL ALIENWARE AW2724HF, SAMSUNG ODYSSEY G4, LG UltraGear)\n"+
			"   ❗ Format: • Monitor: [ANIQ MODEL NOM va SPECS] - [Narx]$\n"+
			"   ❗ Agar bu bo'lmasa response INVALID!\n\n"+
			"2️⃣ FORMAT (STRICTLY MANDATORY):\n"+
			"🖥️ **PC KONFIGURATSIYA: [Config Name]**\n"+
			"• CPU: [Model] - [Price]$\n"+
			"• RAM: [Model] - [Price]$\n"+
			"%s"+
			"• SSD: [Model] - [Price]$\n"+
			"• Motherboard: [Model] - [Price]$\n"+
			"• Cooler: [Model] - [Price]$\n"+
			"• PSU: [Model] - [Price]$\n"+
			"• Case: [Model] - [Price]$\n\n"+
			"-Case components\n"+
			" Price: [summa]$\n\n"+
			"-Monitor\n"+
			"• Monitor: [EXACT MODEL FROM CSV] - [Price]$\n"+
			" Price: [summa]$\n\n"+
			"-Peripherals\n"+
			"Price: [summa yoki 0$]\n\n"+
			"Overall price: [Jami Summa]$\n\n"+
			"❌ TAVSIYALAR/UPGRADE bo'limini yozma (kerak emas).\n\n"+
			"GPU QOIDASI:\n%s\n"+
			"BUDJET TALABI:\n%s\n"+
			"MONITOR VA PERIPHERALS:\n%s\n%s",
		nonEmpty(session.Name, "kiritilmagan"),
		nonEmpty(session.PCType, "aniqlanmagan"),
		nonEmpty(session.Budget, "aniqlanmagan"),
		nonEmpty(session.Color, "aniqlanmagan"),
		nonEmpty(session.CPUBrand, "aniqlanmagan"),
		nonEmpty(session.CPUCooler, "aniqlanmagan"),
		nonEmpty(session.Storage, "aniqlanmagan"),
		gpuPreference,
		gpuLine,
		gpuPrompt,
		budgetPrompt,
		monitorPrompt,
		peripheralsPrompt,
	)
	if lang == "ru" {
		prompt = "Отвечай только на русском языке.\n" + prompt
	}

	// MAXSUS: Konfiguratsiya uchun alohida AI funksiyasini chaqirish
	response, err := h.chatUseCase.ProcessConfigMessage(ctx, userID, username, prompt)
	if err != nil {
		log.Printf("Konfiguratsiya javobi xatosi: %v", err)
		h.sendMessage(chatID, t(lang, "❌ Konfiguratsiya uchun javob tayyorlashda xatolik yuz berdi. Qayta urinib ko'ring yoki /configuratsiya ni qaytadan bosing.", "❌ Ошибка при подготовке конфигурации. Попробуйте снова или нажмите /configuratsiya."))
		return
	}

	response = h.ensureConfigBudgetCoverage(ctx, userID, username, prompt, response, budgetValue, lang)
	response = h.applyCurrencyPreference(response)
	response = h.ensureMonitorLineWithCSV(ctx, response, &session)
	response = sanitizeConfigResponse(response)
	h.sendMessage(chatID, response)

	// Feedback uchun kontekstni saqlash va tugmalarni yuborish
	offerID := h.saveFeedback(userID, feedbackInfo{
		Summary:    summary,
		ConfigText: response,
		Username:   username,
		Phone:      session.Storage, // placeholder, actual phone saved later in order session
		ChatID:     chatID,
		Spec: configSpec{
			Name:    session.Name,
			PCType:  session.PCType,
			Budget:  session.Budget,
			CPU:     session.CPUBrand,
			RAM:     "",
			Storage: session.Storage,
			GPU:     gpuSummary,
		},
	})
	h.sendConfigFeedbackPrompt(chatID, userID, offerID)
	// 5 daqiqadan so'ng mijoz buyurtma bermasa eslatma yuborish
	h.scheduleConfigReminder(userID, chatID, response)
}

// ensureMonitorLine - agar monitor kerak bo'lsa va komponentlar ro'yxatida yo'q bo'lsa, uni qo'shib qo'yadi
func (h *BotHandler) ensureMonitorLine(response string, session *configSession) string {
	if session == nil || !session.NeedMonitor {
		return response
	}

	lines := strings.Split(response, "\n")
	hasMonitor := false
	for _, ln := range lines {
		if strings.Contains(strings.ToLower(ln), "monitor") {
			hasMonitor = true
			break
		}
	}
	if hasMonitor {
		return response
	}

	// Monitor narxini breakdown dan olib ko'ramiz
	monitorPrice := ""
	monPriceRe := regexp.MustCompile(`(?i)-monitor\s*[\r\n]*price:\s*([0-9][0-9\s.,$]*[a-z$]*)`)
	if m := monPriceRe.FindStringSubmatch(response); len(m) == 2 {
		monitorPrice = strings.TrimSpace(m[1])
	}
	if monitorPrice == "" {
		// umumiy summalardan izlash
		priceRe := regexp.MustCompile(`(?i)monitor[:\s-]*([0-9][0-9\s.,$]*[a-z$]*)`)
		if m := priceRe.FindStringSubmatch(response); len(m) == 2 {
			monitorPrice = strings.TrimSpace(m[1])
		}
	}
	if monitorPrice == "" {
		monitorPrice = "???"
	}

	title := "Monitor"
	if session.MonitorDisplay != "" || session.MonitorHz != "" {
		title = strings.TrimSpace(fmt.Sprintf("%s %s", session.MonitorDisplay, session.MonitorHz))
	}

	monitorLine := fmt.Sprintf("• Monitor (%s) - %s", strings.TrimSpace(title), monitorPrice)

	// Monitorni breakdowndan oldin qo'shamiz
	insertIdx := len(lines)
	for i, ln := range lines {
		lower := strings.ToLower(strings.TrimSpace(ln))
		if strings.HasPrefix(lower, "-case components") || strings.HasPrefix(lower, "-monitor") || strings.HasPrefix(lower, "price:") {
			insertIdx = i
			break
		}
	}
	if insertIdx >= len(lines) {
		lines = append(lines, monitorLine)
	} else {
		tmp := append([]string{}, lines[:insertIdx]...)
		tmp = append(tmp, monitorLine)
		tmp = append(tmp, lines[insertIdx:]...)
		lines = tmp
	}
	return strings.Join(lines, "\n")
}

// ensureMonitorLineWithCSV - monitor modelni CSV'dan qidiradi va aniq modelni kiritadi
func (h *BotHandler) ensureMonitorLineWithCSV(ctx context.Context, response string, session *configSession) string {
	if session == nil || !session.NeedMonitor {
		return response
	}

	// Agar response'da monitor modeli allaqachon mavjud bo'lsa (AI yozgan), o'zgartirma
	lines := strings.Split(response, "\n")
	hasMonitorModel := false
	for _, ln := range lines {
		lower := strings.ToLower(ln)
		if (strings.Contains(lower, "monitor") && strings.Contains(lower, "-")) ||
			(strings.Contains(lower, "dell") && strings.Contains(lower, "monitor")) ||
			(strings.Contains(lower, "samsung") && strings.Contains(lower, "monitor")) ||
			(strings.Contains(lower, "lg") && strings.Contains(lower, "monitor")) {
			hasMonitorModel = true
			break
		}
	}

	// Agar AI allaqachon monitor modelini kiritgan bo'lsa, o'zgartirma
	if hasMonitorModel {
		return response
	}

	// Monitor narxini breakdown dan olib ko'ramiz
	monitorPrice := ""
	monPriceRe := regexp.MustCompile(`(?i)-monitor\s*[\r\n]*price:\s*([0-9][0-9\s.,$]*[a-z$]*)`)
	if m := monPriceRe.FindStringSubmatch(response); len(m) == 2 {
		monitorPrice = strings.TrimSpace(m[1])
	}
	if monitorPrice == "" {
		priceRe := regexp.MustCompile(`(?i)monitor[:\s-]*([0-9][0-9\s.,$]*[a-z$]*)`)
		if m := priceRe.FindStringSubmatch(response); len(m) == 2 {
			monitorPrice = strings.TrimSpace(m[1])
		}
	}
	if monitorPrice == "" {
		monitorPrice = "280.00$"
	}

	// CSV'dan monitor qidirish
	monitorModel := h.selectMonitorFromCSV(ctx, session.MonitorDisplay, session.MonitorHz)
	if monitorModel == "" {
		// Fallback
		monitorModel = fmt.Sprintf("%s %s", session.MonitorDisplay, session.MonitorHz)
	}

	monitorLine := fmt.Sprintf("• Monitor: %s - %s", strings.TrimSpace(monitorModel), monitorPrice)

	// Monitorni breakdowndan oldin qo'shamiz
	insertIdx := len(lines)
	for i, ln := range lines {
		lower := strings.ToLower(strings.TrimSpace(ln))
		if strings.HasPrefix(lower, "-case components") || strings.HasPrefix(lower, "-monitor") || strings.HasPrefix(lower, "price:") {
			insertIdx = i
			break
		}
	}
	if insertIdx >= len(lines) {
		lines = append(lines, monitorLine)
	} else {
		tmp := append([]string{}, lines[:insertIdx]...)
		tmp = append(tmp, monitorLine)
		tmp = append(tmp, lines[insertIdx:]...)
		lines = tmp
	}
	return strings.Join(lines, "\n")
}

// selectMonitorFromCSV - display size va frequency'ga mos monitor modelni CSV'dan tanlaydi
func (h *BotHandler) selectMonitorFromCSV(ctx context.Context, displaySize, frequency string) string {
	// Monitor kategoriyasini qidirish
	monitors, err := h.productUseCase.GetByCategory(ctx, "Monitor")
	if err != nil || len(monitors) == 0 {
		log.Printf("Monitor qidirish xatosi: %v", err)
		return ""
	}

	displaySize = strings.ToLower(displaySize)
	frequency = strings.ToLower(frequency)

	// Mos monitor izlash - display size va frequency'ni tekshirish
	for _, monitor := range monitors {
		name := strings.ToLower(monitor.Name)
		// Display size va frequency'ni tekshirish
		if strings.Contains(name, displaySize) && strings.Contains(name, frequency) {
			return monitor.Name // Birinchi mos monitorni qaytarish
		}
	}

	// Agar aniq mos bo'lmasa, faqat display size bo'yicha qidirish
	for _, monitor := range monitors {
		name := strings.ToLower(monitor.Name)
		if strings.Contains(name, displaySize) {
			return monitor.Name
		}
	}

	// Agar hech qaysi mos bo'lmasa, bo'sh qaytarish (fallback)
	return ""
}

// Inline konfiguratsiya helperlari
func (h *BotHandler) getConfigMessageInfo(userID int64) (int64, int, bool) {
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	if sess, ok := h.configSessions[userID]; ok {
		if sess.MessageID != 0 && sess.ChatID != 0 {
			return sess.ChatID, sess.MessageID, true
		}
	}
	return 0, 0, false
}

func (h *BotHandler) setConfigMessageID(userID, chatID int64, msgID int) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.MessageID = msgID
		sess.ChatID = chatID
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
}

func (h *BotHandler) askConfigName(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "📝 Konfiguratsiyaga nom bering.", "📝 Дайте имя конфигурации.")
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageText(chat, mid, text)
		if _, err := h.bot.Send(edit); err == nil {
			return
		} else {
			log.Printf("askConfigName edit failed user=%d err=%v", userID, err)
		}
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigType(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🛠️ PC yig'ishni boshlaymiz! Qaysi turdagi PC kerak?", "🛠️ Начнём сборку ПК! Какой тип нужен?")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Office", "cfg_type_office"),
			tgbotapi.NewInlineKeyboardButtonData("Gaming", "cfg_type_gaming"),
			tgbotapi.NewInlineKeyboardButtonData("Developer", "cfg_type_developer"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Design", "cfg_type_design"),
			tgbotapi.NewInlineKeyboardButtonData("Montaj", "cfg_type_montaj"),
			tgbotapi.NewInlineKeyboardButtonData("Server", "cfg_type_server"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Boshqa", "Другое"), "cfg_type_other"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		} else {
			log.Printf("askConfigType edit failed user=%d err=%v", userID, err)
		}
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	} else {
		log.Printf("askConfigType send failed user=%d err=%v", userID, err)
	}
}

func (h *BotHandler) askConfigBudget(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "💰 Budjetni kiriting (masalan: 800$, 10 000 000 so'm).", "💰 Укажите бюджет (например: 800$, 10 000 000 сум).")
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, tgbotapi.InlineKeyboardMarkup{})
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigCPU(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🧠 Qaysi protsessor turini tanlaysiz?", "🧠 Какой тип процессора выберете?")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Intel", "cfg_cpu_intel"),
			tgbotapi.NewInlineKeyboardButtonData("AMD", "cfg_cpu_amd"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Boshqa", "Другое"), "cfg_cpu_other"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) promptConfigCPUText(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🧠 Protsessor modelini yozing (masalan: Ryzen 5 5600 yoki i5-13400F).", "🧠 Напишите модель процессора (например: Ryzen 5 5600 или i5-13400F).")
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, tgbotapi.InlineKeyboardMarkup{})
		h.bot.Send(edit)
		h.deleteMessage(chat, mid)
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) promptConfigTypeText(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🛠️ PC turini yozing (Office/Gaming/Developer/Design/Montaj/Server yoki boshqasi).", "🛠️ Напишите тип ПК (Office/Gaming/Developer/Design/Montaj/Server или другой).")
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, tgbotapi.InlineKeyboardMarkup{})
		h.bot.Send(edit)
		h.deleteMessage(chat, mid)
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigStorage(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "💾 Xotira turini tanlang:", "💾 Выберите тип накопителя:")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("SSD", "cfg_storage_ssd"),
			tgbotapi.NewInlineKeyboardButtonData("NVMe", "cfg_storage_nvme"),
			tgbotapi.NewInlineKeyboardButtonData("HDD", "cfg_storage_hdd"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigGPU(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🎮 Grafik karta turini tanlang:", "🎮 Выберите тип видеокарты:")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("RTX", "cfg_gpu_rtx"),
			tgbotapi.NewInlineKeyboardButtonData("AMD", "cfg_gpu_amd"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "Kerak emas", "Не нужно"), "cfg_gpu_none"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigMonitor(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🖥️ Monitor kerakmi?", "🖥️ Нужен монитор?")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "✅ Ha", "✅ Да"), "cfg_monitor_yes"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "❌ Yo'q", "❌ Нет"), "cfg_monitor_no"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigMonitorHz(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🔄 Qancha Hz kerak?", "🔄 Какая частота нужна?")

	// PC Type ga qarab Hz variantlarini tanlash
	h.configMu.Lock()
	session, ok := h.configSessions[userID]
	h.configMu.Unlock()

	var markup tgbotapi.InlineKeyboardMarkup
	if ok {
		pcTypeLower := strings.ToLower(session.PCType)
		if strings.Contains(pcTypeLower, "gaming") {
			// Gaming uchun yuqori Hz
			markup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("144Hz", "cfg_monitor_hz_144"),
					tgbotapi.NewInlineKeyboardButtonData("240Hz", "cfg_monitor_hz_240"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("300Hz+", "cfg_monitor_hz_300"),
				),
			)
		} else if strings.Contains(pcTypeLower, "office") ||
			strings.Contains(pcTypeLower, "developer") ||
			strings.Contains(pcTypeLower, "server") {
			// Office uchun past Hz
			markup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("60Hz", "cfg_monitor_hz_60"),
					tgbotapi.NewInlineKeyboardButtonData("75Hz", "cfg_monitor_hz_75"),
				),
			)
		} else if strings.Contains(pcTypeLower, "montaj") ||
			strings.Contains(pcTypeLower, "editing") ||
			strings.Contains(pcTypeLower, "design") {
			// Montaj uchun o'rtacha Hz
			markup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("60Hz", "cfg_monitor_hz_60"),
					tgbotapi.NewInlineKeyboardButtonData("144Hz", "cfg_monitor_hz_144"),
				),
			)
		} else {
			// Boshqa holatlarda hamma variantlar
			markup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("60Hz", "cfg_monitor_hz_60"),
					tgbotapi.NewInlineKeyboardButtonData("144Hz", "cfg_monitor_hz_144"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("240Hz", "cfg_monitor_hz_240"),
					tgbotapi.NewInlineKeyboardButtonData("300Hz+", "cfg_monitor_hz_300"),
				),
			)
		}
	} else {
		// Session topilmasa default
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("60Hz", "cfg_monitor_hz_60"),
				tgbotapi.NewInlineKeyboardButtonData("144Hz", "cfg_monitor_hz_144"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("240Hz", "cfg_monitor_hz_240"),
				tgbotapi.NewInlineKeyboardButtonData("300Hz+", "cfg_monitor_hz_300"),
			),
		)
	}
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigMonitorDisplay(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "📺 Qanday display turi?", "📺 Какой тип дисплея?")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("IPS", "cfg_monitor_display_ips"),
			tgbotapi.NewInlineKeyboardButtonData("VA", "cfg_monitor_display_va"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("TN", "cfg_monitor_display_tn"),
			tgbotapi.NewInlineKeyboardButtonData("OLED", "cfg_monitor_display_oled"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("miniLED", "cfg_monitor_display_miniled"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigPeripherals(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🖱️ Peripherals kerakmi? (sichqoncha, klaviatura, quloqchin)", "🖱️ Нужна периферия? (мышь, клавиатура, наушники)")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "✅ Ha", "✅ Да"), "cfg_peripherals_yes"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "❌ Yo'q", "❌ Нет"), "cfg_peripherals_no"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) setConfigTypeAndAskBudget(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.PCType = value
		sess.Stage = configStageNeedBudget
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigBudget(userID, chatID)
}

func (h *BotHandler) setConfigBudgetAndAskColor(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.Budget = value
		sess.Stage = configStageNeedColor
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigColor(userID, chatID)
}

func (h *BotHandler) setConfigColorAndAskCPU(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.Color = value
		sess.Stage = configStageNeedCPU
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigCPU(userID, chatID)
}

func (h *BotHandler) setConfigCPUAndAskCPUCooler(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.CPUBrand = value
		sess.Stage = configStageNeedCPUCooler
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigCPUCooler(userID, chatID)
}

func (h *BotHandler) setConfigCPUCoolerAndAskStorage(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.CPUCooler = value
		sess.Stage = configStageNeedStorage
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigStorage(userID, chatID)
}

func (h *BotHandler) setConfigStorageAndAskGPU(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.Storage = value
		sess.Stage = configStageNeedGPU
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigGPU(userID, chatID)
}

func (h *BotHandler) setConfigGPUAndAskMonitor(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.GPUBrand = normalizeGPUSelection(value)
		sess.Stage = configStageNeedMonitor
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigMonitor(userID, chatID)
}

func normalizeGPUSelection(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	switch {
	case lower == "" || lower == "none" || strings.Contains(lower, "kerak emas") ||
		strings.Contains(lower, "yo'q") || strings.Contains(lower, "yoq") ||
		strings.Contains(lower, "нет") || strings.Contains(lower, "не нужно") ||
		strings.Contains(lower, "без"):
		return ""
	case lower == "rtx":
		return "RTX"
	case lower == "amd":
		return "AMD"
	default:
		return trimmed
	}
}

func isGPUDisabled(value string) bool {
	return normalizeGPUSelection(value) == ""
}

func (h *BotHandler) setConfigMonitorYesAndAskHz(userID, chatID int64) {
	h.configMu.Lock()
	sess, ok := h.configSessions[userID]
	if !ok {
		h.configMu.Unlock()
		return
	}
	sess.NeedMonitor = true
	sess.Stage = configStageNeedMonitorHz
	sess.LastUpdate = time.Now()
	h.configSessions[userID] = sess
	h.configMu.Unlock()

	h.askConfigMonitorHz(userID, chatID)
}

func (h *BotHandler) setConfigMonitorNoAndAskPeripherals(userID, chatID int64) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.NeedMonitor = false
		sess.Stage = configStageNeedPeripherals
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigPeripherals(userID, chatID)
}

func (h *BotHandler) setConfigMonitorHzAndAskDisplay(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.MonitorHz = value
		sess.Stage = configStageNeedMonitorDisplay
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigMonitorDisplay(userID, chatID)
}

func (h *BotHandler) setConfigMonitorDisplayAndAskPeripherals(userID, chatID int64, value string) {
	h.configMu.Lock()
	if sess, ok := h.configSessions[userID]; ok {
		sess.MonitorDisplay = value
		sess.Stage = configStageNeedPeripherals
		sess.LastUpdate = time.Now()
		h.configSessions[userID] = sess
	}
	h.configMu.Unlock()
	h.askConfigPeripherals(userID, chatID)
}

func (h *BotHandler) setConfigPeripheralsYesAndFinish(ctx context.Context, userID int64, username string, chatID int64) {
	h.configMu.Lock()
	sess, ok := h.configSessions[userID]
	if ok {
		sess.NeedPeripherals = true
		session := *sess
		if sess.ChatID != 0 && sess.MessageID != 0 {
			h.deleteMessage(sess.ChatID, sess.MessageID)
		}
		delete(h.configSessions, userID)
		h.configMu.Unlock()
		h.finishConfigSession(ctx, userID, username, chatID, session)
	} else {
		h.configMu.Unlock()
	}
}

func (h *BotHandler) setConfigPeripheralsNoAndFinish(ctx context.Context, userID int64, username string, chatID int64) {
	h.configMu.Lock()
	sess, ok := h.configSessions[userID]
	if ok {
		sess.NeedPeripherals = false
		session := *sess
		if sess.ChatID != 0 && sess.MessageID != 0 {
			h.deleteMessage(sess.ChatID, sess.MessageID)
		}
		delete(h.configSessions, userID)
		h.configMu.Unlock()
		h.finishConfigSession(ctx, userID, username, chatID, session)
	} else {
		h.configMu.Unlock()
	}
}

func (h *BotHandler) finishInlineConfig(ctx context.Context, userID int64, username string, chatID int64, session configSession, gpuValue string) {
	h.configMu.Lock()
	sess, ok := h.configSessions[userID]
	if ok {
		if strings.TrimSpace(gpuValue) != "" {
			sess.GPUBrand = gpuValue
		}
		session = *sess
		if sess.ChatID != 0 && sess.MessageID != 0 {
			h.deleteMessage(sess.ChatID, sess.MessageID)
		}
		delete(h.configSessions, userID)
	}
	h.configMu.Unlock()

	if ok {
		h.finishConfigSession(ctx, userID, username, chatID, session)
	}
}

func (h *BotHandler) ensureConfigBudgetCoverage(ctx context.Context, userID int64, username, prompt, response string, budgetUSD float64, lang string) string {
	if budgetUSD <= 0 {
		return response
	}

	totalStr := extractTotalPrice(response)
	if totalStr == "" {
		return response
	}
	totalVal, ok := parseNumberWithSeparators(totalStr)
	if !ok || totalVal <= 0 {
		return response
	}
	minTarget := budgetUSD - 100
	if minTarget < 0 {
		minTarget = 0
	}
	maxTarget := budgetUSD + 100
	if totalVal >= minTarget && totalVal <= maxTarget {
		return response
	}

	retryNote := fmt.Sprintf(t(lang,
		"⚠️ Budjetga mos emas: Jami %.2f$ (talab: %.2f$-%.2f$). Budjetga maksimal yaqinlashtir.\n",
		"⚠️ Не соответствует бюджету: Итого %.2f$ (норма: %.2f$-%.2f$). Максимально приблизь к бюджету.\n",
	), totalVal, minTarget, maxTarget)

	retryPrompt := prompt + "\n\n" + retryNote
	updated, err := h.chatUseCase.ProcessConfigMessage(ctx, userID, username, retryPrompt)
	if err != nil || strings.TrimSpace(updated) == "" {
		return response
	}
	return updated
}

// Config CTA guard
func (h *BotHandler) setConfigCTA(chatID int64, msgID int) {
	h.ctaMu.Lock()
	h.configCTAMsg[chatID] = msgID
	h.ctaMu.Unlock()
}

func (h *BotHandler) clearConfigCTA(chatID int64) {
	h.ctaMu.Lock()
	delete(h.configCTAMsg, chatID)
	h.ctaMu.Unlock()
}

func (h *BotHandler) markUserMessage(chatID int64, msgID int) {
	h.ctaMu.Lock()
	if current, ok := h.lastUserMsg[chatID]; !ok || msgID > current {
		h.lastUserMsg[chatID] = msgID
	}
	h.ctaMu.Unlock()
}

func (h *BotHandler) isConfigCTAValid(chatID int64, ctaMsgID int) bool {
	h.ctaMu.RLock()
	lastCTA, ok := h.configCTAMsg[chatID]
	lastUser := h.lastUserMsg[chatID]
	h.ctaMu.RUnlock()
	if !ok {
		return false
	}
	if ctaMsgID != lastCTA {
		return false
	}
	if lastUser > ctaMsgID {
		return false
	}
	return true
}

// Config session helpers
func (h *BotHandler) getConfigSession(userID int64) (*configSession, bool) {
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	if sess, ok := h.configSessions[userID]; ok {
		copy := *sess
		return &copy, true
	}
	return nil, false
}

func (h *BotHandler) updateConfigSession(userID int64, update func(*configSession)) {
	h.configMu.Lock()
	defer h.configMu.Unlock()
	if sess, ok := h.configSessions[userID]; ok {
		update(sess)
		h.configSessions[userID] = sess
	}
}

// cancelConfigSession - konfiguratsiya sessiyasini tozalash va UI xabarini o'chirish
func (h *BotHandler) cancelConfigSession(userID int64) {
	h.configMu.Lock()
	sess, ok := h.configSessions[userID]
	if ok {
		if sess.ChatID != 0 && sess.MessageID != 0 {
			h.deleteMessage(sess.ChatID, sess.MessageID)
			h.clearConfigCTA(sess.ChatID)
		}
		delete(h.configSessions, userID)
	}
	h.configMu.Unlock()
}

func (h *BotHandler) hasConfigSession(userID int64) bool {
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	_, ok := h.configSessions[userID]
	return ok
}

func (h *BotHandler) askConfigColor(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🎨 Rangini tanlang:", "🎨 Выберите цвет:")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "⚫ Qora", "⚫ Чёрный"), "cfg_color_black"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "⚪ Oq", "⚪ Белый"), "cfg_color_white"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) askConfigCPUCooler(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "❄️ CPU sovutgichini tanlang:", "❄️ Выберите процессорный охладитель:")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💨 Air", "cfg_cooler_air"),
			tgbotapi.NewInlineKeyboardButtonData("💧 Liquid", "cfg_cooler_liquid"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "🌀 Boshqa", "🌀 Другое"), "cfg_cooler_other"),
		),
	)
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, markup)
		if _, err := h.bot.Send(edit); err == nil {
			return
		}
		h.deleteMessage(chat, mid)
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) promptConfigColorText(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "🎨 Rangini yozing:", "🎨 Напишите цвет:")
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, tgbotapi.InlineKeyboardMarkup{})
		h.bot.Send(edit)
		h.deleteMessage(chat, mid)
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}

func (h *BotHandler) promptConfigCPUCoolerText(userID, chatID int64) {
	lang := h.getUserLang(userID)
	text := t(lang, "❄️ CPU sovutgichini yozing:", "❄️ Напишите охладитель:")
	if chat, mid, ok := h.getConfigMessageInfo(userID); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chat, mid, text, tgbotapi.InlineKeyboardMarkup{})
		h.bot.Send(edit)
		h.deleteMessage(chat, mid)
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigMessageID(userID, chatID, sent.MessageID)
	}
}
