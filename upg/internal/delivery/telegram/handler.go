package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/usecase"
)

type configStage int

const (
	configStageNeedType configStage = iota
	configStageNeedBudget
	configStageNeedCPU
	configStageNeedStorage
	configStageNeedGPU
)

type configSession struct {
	Stage      configStage
	PCType     string
	Budget     string
	CPUBrand   string
	Storage    string
	GPUBrand   string
	StartedAt  time.Time
	LastUpdate time.Time
}

type feedbackInfo struct {
	Summary    string
	ConfigText string
	Username   string
	ChatID     int64
	Spec       configSpec
}

type groupThreadInfo struct {
	UserID     int64
	UserChat   int64
	Username   string
	Summary    string
	Config     string
	CreatedAt  time.Time
	AllowOrder bool
}

type pendingApproval struct {
	UserID   int64
	UserChat int64
	Summary  string
	SentAt   time.Time
	Config   string
	Username string
}

type configSpec struct {
	PCType  string
	Budget  string
	CPU     string
	Storage string
	GPU     string
}

type changeRequest struct {
	Component string
	Spec      configSpec
}

type orderStage int

const (
	orderStageNeedName orderStage = iota
	orderStageNeedPhone
	orderStageNeedLocation
	orderStageNeedDeliveryChoice
	orderStageNeedDeliveryConfirm
)

type orderSession struct {
	Stage     orderStage
	Name      string
	Phone     string
	Location  string
	Delivery  string
	Summary   string
	ConfigTxt string
	Username  string
}

type adminApprovalRequest struct {
	Target   groupThreadInfo
	AdminMsg string
}

// BotHandler Telegram bot handler
type BotHandler struct {
	bot              *tgbotapi.BotAPI
	group1ChatID     int64
	group2ChatID     int64
	chatUseCase      usecase.ChatUseCase
	adminUseCase     usecase.AdminUseCase
	productUseCase   usecase.ProductUseCase
	configMu         sync.RWMutex
	configSessions   map[int64]*configSession
	feedbackMu       sync.RWMutex
	feedbacks        map[int64]feedbackInfo
	groupMu          sync.RWMutex
	groupThreads     map[int]groupThreadInfo
	approvalMu       sync.RWMutex
	pendingApprove   map[int64]pendingApproval
	adminApprovalMu  sync.RWMutex
	adminApprovals   map[int]adminApprovalRequest
	reminderMu       sync.RWMutex
	configReminded   map[int64]bool
	changeMu         sync.RWMutex
	pendingChange    map[int64]changeRequest
	orderMu          sync.RWMutex
	orderSessions    map[int64]*orderSession
	userMsgMu        sync.RWMutex
	awaitingAdminMsg map[int64]bool
	shopMu           sync.RWMutex
	shopMode         map[int64]bool

	// Admin login kutilayotgan userlar
	awaitingPassword map[int64]bool
	mu               sync.RWMutex
}

const orderDoneStickerID = "CAACAgIAAxkBAAEBzPVpIV1OxMMb6EKyrMB5V4ffAZs3wwACpFoAAmmL2UsgcfjSRzVqDDYE"

// NewBotHandler yangi bot handler yaratish
func NewBotHandler(
	token string,
	group1ChatID int64,
	group2ChatID int64,
	chatUseCase usecase.ChatUseCase,
	adminUseCase usecase.AdminUseCase,
	productUseCase usecase.ProductUseCase,
) (*BotHandler, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &BotHandler{
		bot:              bot,
		group1ChatID:     group1ChatID,
		group2ChatID:     group2ChatID,
		chatUseCase:      chatUseCase,
		adminUseCase:     adminUseCase,
		productUseCase:   productUseCase,
		configSessions:   make(map[int64]*configSession),
		feedbacks:        make(map[int64]feedbackInfo),
		groupThreads:     make(map[int]groupThreadInfo),
		pendingApprove:   make(map[int64]pendingApproval),
		adminApprovals:   make(map[int]adminApprovalRequest),
		configReminded:   make(map[int64]bool),
		pendingChange:    make(map[int64]changeRequest),
		orderSessions:    make(map[int64]*orderSession),
		awaitingAdminMsg: make(map[int64]bool),
		shopMode:         make(map[int64]bool),
		awaitingPassword: make(map[int64]bool),
	}, nil
}

// Start botni ishga tushirish
func (h *BotHandler) Start(ctx context.Context) error {
	log.Printf("Bot @%s ishga tushdi!", h.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := h.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			log.Println("Bot to'xtatilmoqda...")
			return ctx.Err()
		case update := <-updates:
			if update.CallbackQuery != nil {
				go h.handleCallback(ctx, update.CallbackQuery)
				continue
			}

			if update.Message == nil {
				continue
			}

			go h.handleMessage(ctx, update.Message)
		}
	}
}

// handleMessage xabarni qayta ishlash
func (h *BotHandler) handleMessage(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	username := message.From.UserName
	if username == "" {
		username = message.From.FirstName
	}

	// Group feedback chatda AI javobini to'xtatamiz, faqat reply logic
	if message.Chat != nil && h.group1ChatID != 0 && message.Chat.ID == h.group1ChatID {
		h.handleGroupMessage(ctx, message)
		return
	}

	// Fayl yuborilgan bo'lsa
	if message.Document != nil {
		h.handleDocumentMessage(ctx, message)
		return
	}

	// Parol kutilayotgan bo'lsa
	if h.isAwaitingPassword(userID) {
		h.handlePasswordInput(ctx, message)
		return
	}

	// Komandalarni qayta ishlash
	if message.IsCommand() {
		h.handleCommand(ctx, message)
		return
	}

	// Oddiy xabarlarni AI ga yuborish
	if message.Text != "" || message.Contact != nil || message.Location != nil {
		h.handleTextMessage(ctx, userID, username, message.Text, message.Chat.ID, message)
	}
}

// handleCommand komandalarni qayta ishlash
func (h *BotHandler) handleCommand(ctx context.Context, message *tgbotapi.Message) {
	switch message.Command() {
	case "start":
		h.sendMessage(message.Chat.ID, h.getWelcomeMessage())
	case "help":
		h.sendMessage(message.Chat.ID, h.getHelpMessage())
	case "clear":
		h.handleClearCommand(ctx, message)
	case "history":
		h.handleHistoryCommand(ctx, message)
	case "admin":
		h.handleAdminCommand(ctx, message)
	case "logout":
		h.handleLogoutCommand(ctx, message)
	case "catalog":
		h.handleCatalogCommand(ctx, message)
	case "products":
		h.handleProductsCommand(ctx, message)
	case "configuratsiya":
		h.handleConfigCommand(ctx, message)
	case "clean":
		h.handleCleanCommand(ctx, message)
	case "shop":
		h.handleShopCommand(ctx, message)
	default:
		h.sendMessage(message.Chat.ID, "Noma'lum komanda. /help yordam uchun.")
	}
}

// handleAdminCommand admin login boshlash
func (h *BotHandler) handleAdminCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	// Allaqachon admin bo'lsa
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if isAdmin {
		h.sendMessage(message.Chat.ID, "Siz allaqachon admin sifatida tizimga kirgansiz!")
		return
	}

	// Parol kutish rejimini yoqish
	h.setAwaitingPassword(userID, true)
	h.sendMessage(message.Chat.ID, "🔐 Admin parolini kiriting:")
}

// handlePasswordInput parol kiritilganini qayta ishlash
func (h *BotHandler) handlePasswordInput(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID
	password := message.Text

	// Parol kutish rejimini o'chirish
	h.setAwaitingPassword(userID, false)

	// Xabarni o'chirish (xavfsizlik uchun)
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	h.bot.Send(deleteMsg)

	// Login urinishi
	success, err := h.adminUseCase.Login(ctx, userID, password)
	if err != nil {
		log.Printf("Login error: %v", err)
		h.sendMessage(message.Chat.ID, "❌ Login xatosi yuz berdi.")
		return
	}

	if !success {
		h.sendMessage(message.Chat.ID, "❌ Noto'g'ri parol!")
		return
	}

	// Muvaffaqiyatli login
	welcomeMsg := `✅ Admin panelga xush kelibsiz!

🔧 Admin imkoniyatlari:
• Excel fayl yuklash orqali mahsulot katalogini yangilash
• Mahsulotlar ro'yxatini ko'rish
• Katalog statistikasi

📤 Mahsulot katalogini yuklash uchun:
Excel faylni (maksimal 5MB) botga yuboring. Fayl quyidagi ustunlarni o'z ichiga olishi kerak:
- Nomi / Name
- Kategoriya / Category
- Narx / Price
- Tavsif / Description (ixtiyoriy)
- Soni / Stock (ixtiyoriy)

/catalog - Hozirgi katalog haqida ma'lumot
/products - Barcha mahsulotlar ro'yxati
/logout - Admin paneldan chiqish`

	btns := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗂 Foydalanuvchi yozishmalari", "admin_msgs"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, welcomeMsg)
	msg.ReplyMarkup = btns
	h.bot.Send(msg)
}

// Shop so'rovlarini qayta ishlash
func (h *BotHandler) handleShopMessage(ctx context.Context, userID int64, username, text string, chatID int64) bool {
	query := strings.TrimSpace(text)
	if query == "" {
		h.sendMessage(chatID, "Mahsulot nomi yoki talabi aniq emas. Masalan: \"RTX 3060\", \"1TB SSD\".")
		return true
	}

	products, err := h.productUseCase.Search(ctx, query)
	if err != nil {
		h.sendMessage(chatID, "❌ Mahsulotlarni qidirishda xatolik. Boshqa so'rov kiriting.")
		return true
	}

	if len(products) == 0 {
		h.sendMessage(chatID, "Mahsulot topilmadi. Boshqa nom yoki modelni yozib ko'ring.")
		return true
	}

	preview := buildProductPreview(products, 6)
	h.setShopMode(userID, false)

	h.savePendingApproval(userID, pendingApproval{
		UserID:   userID,
		UserChat: chatID,
		Summary:  fmt.Sprintf("Shop so'rov: %s", query),
		Config:   preview,
		Username: username,
		SentAt:   time.Now(),
	})

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ha ✅", "shop_yes"),
			tgbotapi.NewInlineKeyboardButtonData("Yo'q ❌", "shop_no"),
			tgbotapi.NewInlineKeyboardButtonData("Variant ko'raman 🔄", "shop_more"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Topdim!\n\n%s\nRasmiylashtiramizmi?", preview))
	msg.ReplyMarkup = markup
	h.bot.Send(msg)
	return true
}

// Bevosita katalogdan mahsulot qidirish (AI ga bormasdan)
func (h *BotHandler) handleDirectProductSearch(ctx context.Context, userID int64, username, text string, chatID int64) bool {
	if !isProductSearchQuery(text) {
		return false
	}

	// Foydalanuvchi so'rovidan muhim kalit so'zlarni ajratib olamiz
	searchQuery := extractProductKeywords(text)
	if searchQuery == "" {
		return false // Agar hech qanday muhim so'z topilmasa, AI ga yuboramiz
	}

	products, err := h.productUseCase.Search(ctx, searchQuery)
	if err != nil {
		log.Printf("Mahsulot qidirish xatosi: %v", err)
		return false
	}

	if len(products) == 0 {
		return h.handleAIProductSearch(ctx, userID, username, text, chatID)
	}

	preview := buildProductPreview(products, 6)
	h.savePendingApproval(userID, pendingApproval{
		UserID:   userID,
		UserChat: chatID,
		Summary:  fmt.Sprintf("Mahsulot so'rovi: %s", text),
		Config:   preview,
		Username: username,
		SentAt:   time.Now(),
	})

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ha ✅", "buy_yes"),
			tgbotapi.NewInlineKeyboardButtonData("Yo'q ❌", "buy_no"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Ha, topdim! %s\n\nRasmiylashtiramizmi?\n\n%s", text, preview))
	msg.ReplyMarkup = markup
	h.bot.Send(msg)
	return true
}

// AI javobidan keyin sotib olish taklifini ko'rsatish
func (h *BotHandler) maybeAskToBuy(chatID, userID int64, username, userText, response string) {
	if !shouldOfferPurchase(userText, response) {
		return
	}

	h.savePendingApproval(userID, pendingApproval{
		UserID:   userID,
		UserChat: chatID,
		Summary:  fmt.Sprintf("Mahsulot so'rovi: %s", userText),
		Config:   response,
		Username: username,
		SentAt:   time.Now(),
	})

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ha ✅", "buy_yes"),
			tgbotapi.NewInlineKeyboardButtonData("Yo'q ❌", "buy_no"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Ha, bizda bor! Sotib olishni istaysizmi?")
	msg.ReplyMarkup = markup
	h.bot.Send(msg)
}

// handleLogoutCommand admin logout
func (h *BotHandler) handleLogoutCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "Siz admin emassiz.")
		return
	}

	err := h.adminUseCase.Logout(ctx, userID)
	if err != nil {
		h.sendMessage(message.Chat.ID, "Logout xatosi.")
		return
	}

	h.sendMessage(message.Chat.ID, "✅ Admin paneldan chiqdingiz.")
}

// handleCleanCommand barcha ma'lumotlarni tozalash (admin)
func (h *BotHandler) handleCleanCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	if err := h.adminUseCase.CleanAll(ctx, userID); err != nil {
		log.Printf("Clean error: %v", err)
		h.sendMessage(message.Chat.ID, "❌ Tozalashda xatolik yuz berdi.")
		return
	}

	h.sendMessage(message.Chat.ID, "🧹 Barcha mahsulotlar va chat tarixlari tozalandi. Bot yangilanib boshlandi.")
}

// handleCatalogCommand katalog haqida ma'lumot
func (h *BotHandler) handleCatalogCommand(ctx context.Context, message *tgbotapi.Message) {
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, message.From.ID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Bu komanda faqat adminlar uchun.")
		return
	}

	info, err := h.adminUseCase.GetCatalogInfo(ctx)
	if err != nil {
		h.sendMessage(message.Chat.ID, "❌ Katalog topilmadi. Excel fayl yuklang.")
		return
	}

	h.sendMessage(message.Chat.ID, info)
}

// handleProductsCommand mahsulotlar ro'yxati
func (h *BotHandler) handleProductsCommand(ctx context.Context, message *tgbotapi.Message) {
	products, err := h.productUseCase.GetAll(ctx)
	if err != nil || len(products) == 0 {
		h.sendMessage(message.Chat.ID, "❌ Mahsulotlar topilmadi.")
		return
	}

	productsText, err := h.productUseCase.GetProductsAsText(ctx)
	if err != nil {
		h.sendMessage(message.Chat.ID, "❌ Mahsulotlarni yuklashda xatolik.")
		return
	}

	// Uzun bo'lsa, qismlarga bo'lib yuborish
	if len(productsText) > 4000 {
		h.sendMessage(message.Chat.ID, fmt.Sprintf("📦 Jami %d ta mahsulot mavjud. Katalog juda katta, AI bilan savdo qilishingiz mumkin.", len(products)))
	} else {
		h.sendMessage(message.Chat.ID, productsText)
	}
}

// handleConfigCommand konfiguratsiya sessiyasini boshlash
func (h *BotHandler) handleConfigCommand(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	h.startConfigSession(userID)

	firstStep := "🛠️ PC yig'ishni boshlaymiz! Qaysi turdagi PC kerak? (Office / Gaming / Montaj / Server / Boshqa)"
	h.sendMessage(message.Chat.ID, firstStep)
}

// handleConfigFlow konfiguratsiya savollarini bosqichma-bosqich ko'rsatish
func (h *BotHandler) handleConfigFlow(ctx context.Context, userID int64, username, text string, chatID int64) {
	input := strings.TrimSpace(text)

	h.configMu.Lock()
	session, ok := h.configSessions[userID]
	if !ok {
		h.configMu.Unlock()
		h.sendMessage(chatID, "Konfiguratsiya sessiyasi topilmadi. Boshlash uchun /configuratsiya ni bosing.")
		return
	}

	session.LastUpdate = time.Now()

	switch session.Stage {
	case configStageNeedType:
		session.PCType = input
		session.Stage = configStageNeedBudget
		h.configMu.Unlock()
		h.sendMessage(chatID, "💰 Budjetni kiriting (masalan: 800$, 10 000 000 so'm). Aniq bo'lmasa, taxminiy yozing.")
		return
	case configStageNeedBudget:
		session.Budget = input
		session.Stage = configStageNeedCPU
		h.configMu.Unlock()
		h.sendMessage(chatID, "🧠 Qaysi protsessor turini xohlaysiz? Intel yoki AMD?")
		return
	case configStageNeedCPU:
		session.CPUBrand = input
		session.Stage = configStageNeedStorage
		h.configMu.Unlock()
		h.sendMessage(chatID, "💾 Xotira turi? HDD, SSD yoki NVMe?")
		return
	case configStageNeedStorage:
		session.Storage = input
		session.Stage = configStageNeedGPU
		h.configMu.Unlock()
		h.sendMessage(chatID, "🎮 Grafik karta: NVIDIA RTXmi yoki AMD Radeon?")
		return
	case configStageNeedGPU:
		session.GPUBrand = input
		completed := *session
		delete(h.configSessions, userID)
		h.configMu.Unlock()
		h.finishConfigSession(ctx, userID, username, chatID, completed)
		return
	default:
		delete(h.configSessions, userID)
		h.configMu.Unlock()
		h.sendMessage(chatID, "Sessiya qayta ishga tushirildi. Yangi boshlash uchun /configuratsiya ni bosing.")
		return
	}
}

// finishConfigSession tanlangan parametrlar asosida AI javobi
func (h *BotHandler) finishConfigSession(ctx context.Context, userID int64, username string, chatID int64, session configSession) {
	summary := fmt.Sprintf(`📝 Talablaringizni yozib oldim:
• Maqsad: %s
• Budjet: %s
• CPU: %s
• Xotira: %s
• GPU: %s

Endi shu talablar bo'yicha optimal konfiguratsiyani tanlab beraman...`,
		nonEmpty(session.PCType, "ko'rsatilmagan"),
		nonEmpty(session.Budget, "ko'rsatilmagan"),
		nonEmpty(session.CPUBrand, "ko'rsatilmagan"),
		nonEmpty(session.Storage, "ko'rsatilmagan"),
		nonEmpty(session.GPUBrand, "ko'rsatilmagan"),
	)

	h.sendMessage(chatID, summary)

	// "typing" indikatori
	typingAction := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	h.bot.Send(typingAction)

	// AI ga yuboriladigan so'rov
	prompt := fmt.Sprintf(
		"Konfiguratsiya so'rovi: maqsad=%s, budjet=%s, CPU=%s, xotira=%s, GPU=%s. Shu talablarga mos PC konfiguratsiyasi tuzing va narxlarni ro'yxatdan foydalanib ko'rsating.",
		nonEmpty(session.PCType, "aniqlanmagan"),
		nonEmpty(session.Budget, "aniqlanmagan"),
		nonEmpty(session.CPUBrand, "aniqlanmagan"),
		nonEmpty(session.Storage, "aniqlanmagan"),
		nonEmpty(session.GPUBrand, "aniqlanmagan"),
	)

	response, err := h.chatUseCase.ProcessMessage(ctx, userID, username, prompt)
	if err != nil {
		log.Printf("Konfiguratsiya javobi xatosi: %v", err)
		h.sendMessage(chatID, "❌ Konfiguratsiya uchun javob tayyorlashda xatolik yuz berdi. Qayta urinib ko'ring yoki /configuratsiya ni qaytadan bosing.")
		return
	}

	h.sendMessage(chatID, response)

	// Feedback uchun kontekstni saqlash va tugmalarni yuborish
	h.saveFeedback(userID, feedbackInfo{
		Summary:    summary,
		ConfigText: response,
		Username:   username,
		ChatID:     chatID,
		Spec: configSpec{
			PCType:  session.PCType,
			Budget:  session.Budget,
			CPU:     session.CPUBrand,
			Storage: session.Storage,
			GPU:     session.GPUBrand,
		},
	})
	h.sendConfigFeedbackPrompt(chatID)
}

// handleDocumentMessage fayl yuborilganda
func (h *BotHandler) handleDocumentMessage(ctx context.Context, message *tgbotapi.Message) {
	userID := message.From.ID

	// Admin tekshirish
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.sendMessage(message.Chat.ID, "❌ Fayllarni faqat adminlar yuklashi mumkin. /admin komandasi bilan admin bo'ling.")
		return
	}

	doc := message.Document

	// Fayl hajmini tekshirish (5MB)
	if doc.FileSize > 5*1024*1024 {
		h.sendMessage(message.Chat.ID, "❌ Fayl hajmi 5MB dan oshmasligi kerak!")
		return
	}

	// Fayl turini tekshirish
	if !strings.HasSuffix(doc.FileName, ".xlsx") && !strings.HasSuffix(doc.FileName, ".xls") {
		h.sendMessage(message.Chat.ID, "❌ Faqat Excel fayllari (.xlsx, .xls) qabul qilinadi!")
		return
	}

	h.sendMessage(message.Chat.ID, "⏳ Fayl yuklanmoqda va qayta ishlanmoqda...")

	// Faylni yuklash
	fileBytes, err := h.downloadFile(doc.FileID)
	if err != nil {
		log.Printf("File download error: %v", err)
		h.sendMessage(message.Chat.ID, "❌ Faylni yuklashda xatolik yuz berdi.")
		return
	}

	// Katalogni yangilash
	count, err := h.adminUseCase.UploadCatalog(ctx, userID, fileBytes, doc.FileName)
	if err != nil {
		log.Printf("Upload catalog error: %v", err)
		h.sendMessage(message.Chat.ID, fmt.Sprintf("❌ Katalogni yangilashda xatolik: %v", err))
		return
	}

	successMsg := fmt.Sprintf(`✅ Katalog muvaffaqiyatli yangilandi!

📦 Yuklangan mahsulotlar: %d ta
📄 Fayl: %s

Endi men ushbu mahsulotlar bilan mijozlarga xizmat ko'rsataman!

/catalog - Katalog haqida ma'lumot
/products - Barcha mahsulotlar`, count, doc.FileName)

	h.sendMessage(message.Chat.ID, successMsg)
}

// downloadFile Telegram dan faylni yuklash
func (h *BotHandler) downloadFile(fileID string) ([]byte, error) {
	file, err := h.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, err
	}

	fileURL := file.Link(h.bot.Token)
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// handleTextMessage text xabarlarni qayta ishlash
func (h *BotHandler) handleTextMessage(ctx context.Context, userID int64, username, text string, chatID int64, msg *tgbotapi.Message) {
	// Agar foydalanuvchi adminga yozishni boshlagan bo'lsa
	if h.isAwaitingAdminMessage(userID) {
		h.handleUserMessageForAdmin(ctx, userID, username, text, chatID)
		return
	}

	// Agar konfiguratsiya sessiyasi davom etayotgan bo'lsa, shu oqimni davom ettiramiz
	if h.hasConfigSession(userID) {
		h.handleConfigFlow(ctx, userID, username, text, chatID)
		return
	}

	// /shop rejimidagi so'rovlar
	if h.isInShopMode(userID) {
		if handled := h.handleShopMessage(ctx, userID, username, text, chatID); handled {
			return
		}
	}

	// BEVOSITA QIDIRUVNI O'CHIRIB QOLDIK!
	// AI barcha mahsulotlarni ko'radi va foydalanuvchi so'ragan narsani o'zi topadi
	// Bu ancha ishonchli va aniq!

	// Agar buyurtma rasmiylashtirish jarayoni bo'lsa
	if h.hasOrderSession(userID) {
		h.handleOrderFlow(ctx, userID, username, text, chatID, msg)
		return
	}

	// Agar komponent almashtirish jarayoni bo'lsa
	if h.hasPendingChange(userID) {
		h.handleChangeRequest(ctx, userID, username, text, chatID)
		return
	}

	isCfgReq := isConfigRequest(text)

	// PC yig'ish so'rovi bo'lsa, foydalanuvchini /configuratsiya komandasi tomon yo'naltiramiz
	if isCfgReq {
		if !h.wasConfigReminded(chatID) {
			h.markConfigReminded(chatID)
			h.sendMessage(chatID, "⚙️ PC yig'ishda yordam berish uchun /configuratsiya komandasi orqali bosqichma-bosqich ma'lumot kiriting. Boshlash uchun /configuratsiya ni bosing.")
			return
		}
		// Esalatildi: endi AI ga odatdagi javob berish uchun davom etamiz
	}

	// "typing" indikatori
	typingAction := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	h.bot.Send(typingAction)

	response, err := h.chatUseCase.ProcessMessage(ctx, userID, username, text)
	if err != nil {
		log.Printf("Xatolik: %v", err)
		if isQuotaError(err) {
			h.sendMessage(chatID, "AI xizmatida vaqtincha cheklov. Iltimos, 30 soniyadan so'ng qayta urinib ko'ring.")
		} else {
			h.sendMessage(chatID, "Kechirasiz, xatolik yuz berdi. Iltimos, qayta urinib ko'ring.")
		}
		return
	}

	h.sendMessage(chatID, response)
	h.maybeAskToBuy(chatID, userID, username, text, response)

	// Agar bu konfiguratsiya so'rovi bo'lsa (hatto /configuratsiya emas), feedback tugmalarini yuboramiz
	if isCfgReq || isLikelyConfigResponse(response) {
		h.saveFeedback(userID, feedbackInfo{
			Summary:    fmt.Sprintf("So'rov: %s", text),
			ConfigText: response,
			Username:   username,
			ChatID:     chatID,
		})
		h.sendConfigFeedbackPrompt(chatID)
	}
}

// handleClearCommand tarixni tozalash
func (h *BotHandler) handleClearCommand(ctx context.Context, message *tgbotapi.Message) {
	err := h.chatUseCase.ClearHistory(ctx, message.From.ID)
	if err != nil {
		h.sendMessage(message.Chat.ID, "Tarixni tozalashda xatolik.")
		return
	}
	h.sendMessage(message.Chat.ID, "✅ Chat tarixi tozalandi! Yangi suhbat boshlashingiz mumkin.")
}

// handleHistoryCommand tarixni ko'rsatish
func (h *BotHandler) handleHistoryCommand(ctx context.Context, message *tgbotapi.Message) {
	history, err := h.chatUseCase.GetHistory(ctx, message.From.ID)
	if err != nil {
		h.sendMessage(message.Chat.ID, "Tarixni olishda xatolik.")
		return
	}

	if len(history) == 0 {
		h.sendMessage(message.Chat.ID, "Sizning chat tarixingiz bo'sh.")
		return
	}

	var sb strings.Builder
	sb.WriteString("📜 *Chat tarixi:*\n\n")
	for i, msg := range history {
		sb.WriteString(fmt.Sprintf("*%d.* %s\n", i+1, msg.Text))
		if msg.Response != "" {
			sb.WriteString(fmt.Sprintf("↳ %s\n\n", msg.Response))
		}
	}

	h.sendMessageMarkdown(message.Chat.ID, sb.String())
}

// isAwaitingPassword parol kutilayotganini tekshirish
func (h *BotHandler) isAwaitingPassword(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.awaitingPassword[userID]
}

// setAwaitingPassword parol kutish rejimini o'rnatish
func (h *BotHandler) setAwaitingPassword(userID int64, awaiting bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if awaiting {
		h.awaitingPassword[userID] = true
	} else {
		delete(h.awaitingPassword, userID)
	}
}

// isAwaitingAdminMessage adminga xabar kutilayotganini tekshirish
func (h *BotHandler) isAwaitingAdminMessage(userID int64) bool {
	h.userMsgMu.RLock()
	defer h.userMsgMu.RUnlock()
	return h.awaitingAdminMsg[userID]
}

// setAwaitingAdminMessage adminga xabar kutilishini boshqarish
func (h *BotHandler) setAwaitingAdminMessage(userID int64, awaiting bool) {
	h.userMsgMu.Lock()
	defer h.userMsgMu.Unlock()
	if awaiting {
		h.awaitingAdminMsg[userID] = true
	} else {
		delete(h.awaitingAdminMsg, userID)
	}
}

// Shop mode helpers
func (h *BotHandler) setShopMode(userID int64, on bool) {
	h.shopMu.Lock()
	defer h.shopMu.Unlock()
	if on {
		h.shopMode[userID] = true
	} else {
		delete(h.shopMode, userID)
	}
}

func (h *BotHandler) isInShopMode(userID int64) bool {
	h.shopMu.RLock()
	defer h.shopMu.RUnlock()
	return h.shopMode[userID]
}

// sendMessage oddiy xabar yuborish
func (h *BotHandler) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Xabar yuborishda xatolik: %v", err)
	}
}

// Admin uchun foydalanuvchilar ro'yxati
func (h *BotHandler) sendAdminUserList(chatID int64) {
	msgs, err := h.chatUseCase.GetAllMessages(context.Background(), 200)
	if err != nil {
		h.sendMessage(chatID, "❌ Yozishmalarni yuklab bo'lmadi.")
		return
	}
	if len(msgs) == 0 {
		h.sendMessage(chatID, "Hali yozishmalar yo'q.")
		return
	}

	users := collectUsersFromMessages(msgs)
	if len(users) == 0 {
		h.sendMessage(chatID, "Hali yozishmalar yo'q.")
		return
	}

	markup := buildUserButtons(users)
	text := fmt.Sprintf("🗂 %d ta foydalanuvchi yozishmalari bor. Bittasini tanlang:", len(users))

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	h.bot.Send(msg)
}

// sendMessageWithResp yuborilgan xabarni qaytarish
func (h *BotHandler) sendMessageWithResp(chatID int64, text string) (*tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	sent, err := h.bot.Send(msg)
	if err != nil {
		log.Printf("Xabar yuborishda xatolik: %v", err)
		return nil, err
	}
	return &sent, nil
}

// /shop komandasi
func (h *BotHandler) handleShopCommand(ctx context.Context, message *tgbotapi.Message) {
	h.setShopMode(message.From.ID, true)
	h.sendMessage(message.Chat.ID, "🛒 Shop rejimi: mahsulot nomini yoki talabni yozing. Masalan: \"RTX 3060\", \"144Hz monitor\", \"1TB SSD\". Topib bersam rasmiylashtirish tugmalarini yuboraman.")
}

// AI yordamida qidiruv (fallback)
func (h *BotHandler) handleAIProductSearch(ctx context.Context, userID int64, username, text string, chatID int64) bool {
	productsText, err := h.productUseCase.GetProductsAsText(ctx)
	if err != nil || productsText == "" {
		return false
	}

	prompt := fmt.Sprintf(`Foydalanuvchi so'rovi: "%s"
Quyida do'kon katalogi (nomlari va narxlar). Faqat shu ro'yxatdan eng mos 6 ta mahsulotni tanla.
- Har bir satr: "Nom - $narx"
- Faqat aniq katalog nomini va narxini yoz (o'zgartirma)
- So'rovga mos kelmaydigan kategoriyalarni yozma
- Oxirida jami summa yoki izoh yozma

%s

Eng mos mahsulotlarni raqamlab yoz (1) ... 6) tarzda).`, text, productsText)

	response, err := h.chatUseCase.ProcessMessage(ctx, userID, username, prompt)
	if err != nil {
		log.Printf("AI qidiruv xatosi: %v", err)
		return false
	}

	h.savePendingApproval(userID, pendingApproval{
		UserID:   userID,
		UserChat: chatID,
		Summary:  fmt.Sprintf("AI qidiruv: %s", text),
		Config:   response,
		Username: username,
		SentAt:   time.Now(),
	})

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ha ✅", "buy_yes"),
			tgbotapi.NewInlineKeyboardButtonData("Yo'q ❌", "buy_no"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Topdim (AI qidiruv):\n\n%s\n\nSotib olamizmi?", response))
	msg.ReplyMarkup = markup
	h.bot.Send(msg)
	return true
}

// sendMessageMarkdown markdown formatda xabar yuborish
func (h *BotHandler) sendMessageMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Xabar yuborishda xatolik: %v", err)
	}
}

// Inline feedback tugmalari
func (h *BotHandler) sendConfigFeedbackPrompt(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Konfiguratsiya yoqdimi?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👍 Ha", "cfg_fb_yes"),
			tgbotapi.NewInlineKeyboardButtonData("👎 Yo'q", "cfg_fb_no"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Komponentni almashtirish", "cfg_fb_change"),
		),
	)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Feedback tugmalarini yuborishda xatolik: %v", err)
	}
}

// Feedback ma'lumotini saqlash
func (h *BotHandler) saveFeedback(userID int64, info feedbackInfo) {
	h.feedbackMu.Lock()
	defer h.feedbackMu.Unlock()
	h.feedbacks[userID] = info
}

// Feedback ma'lumotini olish va o'chirish
func (h *BotHandler) popFeedback(userID int64) (feedbackInfo, bool) {
	h.feedbackMu.Lock()
	defer h.feedbackMu.Unlock()
	info, ok := h.feedbacks[userID]
	if ok {
		delete(h.feedbacks, userID)
	}
	return info, ok
}

// getFeedback mavjud feedbackni o'chirmasdan olish
func (h *BotHandler) getFeedback(userID int64) (feedbackInfo, bool) {
	h.feedbackMu.RLock()
	defer h.feedbackMu.RUnlock()
	info, ok := h.feedbacks[userID]
	return info, ok
}

// Callback query larini qayta ishlash
func (h *BotHandler) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	userID := cq.From.ID
	data := cq.Data
	chatID := cq.Message.Chat.ID

	// Callback ga javob (spinnerni to'xtatish)
	callback := tgbotapi.NewCallback(cq.ID, "")
	if _, err := h.bot.Request(callback); err != nil {
		log.Printf("Callback javobida xatolik: %v", err)
	}

	if strings.HasPrefix(data, "adm_approve_yes:") || strings.HasPrefix(data, "adm_approve_no:") {
		allowOrder := strings.HasPrefix(data, "adm_approve_yes:")
		var prefix string
		if allowOrder {
			prefix = "adm_approve_yes:"
		} else {
			prefix = "adm_approve_no:"
		}

		idStr := strings.TrimPrefix(data, prefix)
		reqID, err := strconv.Atoi(idStr)
		if err != nil {
			h.sendMessage(chatID, "❌ Tasdiq ma'lumotini o'qib bo'lmadi.")
			return
		}

		if ok := h.handleAdminApprovalCallback(reqID, allowOrder, chatID, cq.Message.MessageID); !ok {
			h.sendMessage(chatID, "❌ Tasdiqlash topilmadi yoki eskirgan.")
		}
		return
	}

	// Admin foydalanuvchi yozishmalari callbacki
	if strings.HasPrefix(data, "admin_msgs_user:") {
		isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
		if !isAdmin {
			h.sendMessage(chatID, "❌ Bu bo'lim faqat adminlar uchun.")
			return
		}
		h.handleAdminUserMessages(chatID, data)
		return
	}

	switch data {
	case "cfg_fb_yes":
		h.sendMessage(chatID, "👍 Rahmat! Talabni qabul qildik, tez orada admin javob beradi.")
		if info, ok := h.popFeedback(userID); ok {
			if h.group1ChatID != 0 {
				groupMsg := fmt.Sprintf("👍 Konfiguratsiya yoqdi\nUser: @%s (%d)\n%s\n\n📋 AI konfiguratsiya:\n%s", nonEmpty(info.Username, "nomalum"), userID, info.Summary, info.ConfigText)
				if sent, err := h.sendMessageWithResp(h.group1ChatID, groupMsg); err == nil {
					h.saveGroupThread(sent.MessageID, groupThreadInfo{
						UserID:     userID,
						UserChat:   info.ChatID,
						Username:   info.Username,
						Summary:    info.Summary,
						Config:     info.ConfigText,
						CreatedAt:  time.Now(),
						AllowOrder: true,
					})
				}
			}
		}
	case "cfg_fb_no":
		h.sendMessage(chatID, "😔 Uzr, bu konfiguratsiya yoqmadi. Yana bir bor harakat qilib ko'ramizmi? /configuratsiya ni qayta bosishingiz mumkin.")
		h.popFeedback(userID) // tozalash
	case "cfg_fb_change":
		h.sendChangeComponentPrompt(chatID)
	case "cfg_change_cpu":
		if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "cpu", Spec: info.Spec})
		}
		h.sendMessage(chatID, "CPU o'rnida qanday modelni xohlaysiz? To'liq nomini yozib yuboring.")
	case "cfg_change_gpu":
		if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "gpu", Spec: info.Spec})
		}
		h.sendMessage(chatID, "GPU o'rnida qanday modelni xohlaysiz? To'liq nomini yozib yuboring.")
	case "cfg_change_ram":
		if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "ram", Spec: info.Spec})
		}
		h.sendMessage(chatID, "RAM uchun qanday hajm/tezlikni xohlaysiz? Masalan: 32GB DDR5 6000MHz.")
	case "cfg_change_ssd":
		if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "ssd", Spec: info.Spec})
		}
		h.sendMessage(chatID, "Qancha hajmdagi SSD xohlaysiz? Masalan: 1TB NVMe Gen4.")
	case "cfg_change_other":
		if info, ok := h.getFeedback(userID); ok {
			h.setPendingChange(userID, changeRequest{Component: "other", Spec: info.Spec})
		}
		h.sendMessage(chatID, "Qaysi komponentni o'zgartirish kerakligini yozib yuboring.")
	case "admin_msgs":
		isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
		if !isAdmin {
			h.sendMessage(chatID, "❌ Bu bo'lim faqat adminlar uchun.")
			return
		}
		h.sendAdminUserList(chatID)
	case "order_yes":
		info, ok := h.popPendingApproval(userID)
		if !ok {
			h.sendMessage(chatID, "❌ Buyurtma ma'lumotlari topilmadi. Iltimos qaytadan urinib ko'ring.")
			return
		}
		h.startOrderSession(userID, info)
		h.sendMessage(chatID, "📝 Iltimos, to'liq ismingizni yozing.")
	case "order_no":
		h.sendMessage(chatID, "😔 Afsusdamiz. Ketkazgan vaqtingiz uchun uzr so'raymiz.")
		h.popPendingApproval(userID)
	case "delivery_pickup":
		h.handleDeliveryChoice(ctx, userID, "pickup", chatID)
	case "delivery_courier":
		h.handleDeliveryChoice(ctx, userID, "courier", chatID)
	case "delivery_confirm_yes":
		h.handleDeliveryConfirm(ctx, userID, true, chatID)
	case "delivery_confirm_no":
		h.handleDeliveryConfirm(ctx, userID, false, chatID)
	case "msg_admin_start":
		h.setAwaitingAdminMessage(userID, true)
		h.sendMessage(chatID, "✉️ Adminga qoldirmoqchi bo'lgan xabaringizni yozib yuboring.")
	case "buy_yes":
		info, ok := h.popPendingApproval(userID)
		if !ok {
			h.sendMessage(chatID, "❌ Ma'lumot topilmadi. Mahsulot so'rovini qayta yuboring.")
			return
		}
		h.startOrderSession(userID, info)
		h.sendMessage(chatID, "📝 Iltimos, to'liq ismingizni yozing.")
		if h.group1ChatID != 0 {
			h.sendMessage(h.group1ChatID, fmt.Sprintf("🛍 Mahsulot so'rovi: @%s (%d)\n%s\n\n%s", nonEmpty(info.Username, "nomalum"), userID, info.Summary, info.Config))
		}
	case "buy_no":
		h.popPendingApproval(userID)
		h.sendMessage(chatID, "Yana savollar bo'lsa, bemalol yozing.")
	case "shop_yes":
		info, ok := h.popPendingApproval(userID)
		if !ok {
			h.sendMessage(chatID, "❌ Buyurtma ma'lumotlari topilmadi. Qayta qidirib ko'ring.")
			return
		}
		h.startOrderSession(userID, info)
		h.sendMessage(chatID, "📝 Iltimos, to'liq ismingizni yozing.")
		if h.group1ChatID != 0 {
			h.sendMessage(h.group1ChatID, fmt.Sprintf("🛒 Shop so'rovi: @%s (%d)\n%s\n\n%s", nonEmpty(info.Username, "nomalum"), userID, info.Summary, info.Config))
		}
	case "shop_no":
		h.popPendingApproval(userID)
		h.sendMessage(chatID, "Yana biror mahsulot kerak bo'lsa, nomini yozing.")
	case "shop_more":
		h.setShopMode(userID, true)
		h.sendMessage(chatID, "Qaysi variantni qidiramiz? Yangi model yoki kategoriya yozing.")
	default:
		// boshqa callback lar uchun hech narsa qilmaymiz
	}
}

// startConfigSession yangi konfiguratsiya sessiyasini yaratish
func (h *BotHandler) startConfigSession(userID int64) {
	h.configMu.Lock()
	h.configSessions[userID] = &configSession{
		Stage:      configStageNeedType,
		StartedAt:  time.Now(),
		LastUpdate: time.Now(),
	}
	h.configMu.Unlock()
}

// hasConfigSession sessiya mavjudligini tekshirish
func (h *BotHandler) hasConfigSession(userID int64) bool {
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	_, ok := h.configSessions[userID]
	return ok
}

// nonEmpty bo'sh bo'lmagan qiymatni qaytarish
func nonEmpty(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

// Pending change helpers
func (h *BotHandler) setPendingChange(userID int64, cr changeRequest) {
	h.changeMu.Lock()
	defer h.changeMu.Unlock()
	h.pendingChange[userID] = cr
}

func (h *BotHandler) popPendingChange(userID int64) (changeRequest, bool) {
	h.changeMu.Lock()
	defer h.changeMu.Unlock()
	cr, ok := h.pendingChange[userID]
	if ok {
		delete(h.pendingChange, userID)
	}
	return cr, ok
}

func (h *BotHandler) hasPendingChange(userID int64) bool {
	h.changeMu.RLock()
	defer h.changeMu.RUnlock()
	_, ok := h.pendingChange[userID]
	return ok
}

// Komponentni almashtirish uchun yuborilgan matnni qayta ishlash
func (h *BotHandler) handleChangeRequest(ctx context.Context, userID int64, username, text string, chatID int64) {
	cr, ok := h.popPendingChange(userID)
	if !ok {
		return
	}

	newValue := strings.TrimSpace(text)
	if newValue == "" {
		h.sendMessage(chatID, "Iltimos, o'zgartirmoqchi bo'lgan komponent nomini yozing.")
		return
	}

	spec := cr.Spec
	switch cr.Component {
	case "cpu":
		spec.CPU = newValue
	case "gpu":
		spec.GPU = newValue
	case "ram":
		spec.CPU = spec.CPU // no-op, just explicit
		spec.Budget = spec.Budget
		spec.Storage = spec.Storage
		spec.GPU = spec.GPU
		spec.PCType = spec.PCType
		spec.CPU = spec.CPU
		spec.Storage = spec.Storage
		spec.Budget = spec.Budget
	case "ssd":
		spec.Storage = newValue
	case "other":
		// Ustiga qo'shimcha talablarga yozamiz
		spec.Storage = spec.Storage
	}

	prompt := fmt.Sprintf("Mijoz konfiguratsiyani o'zgartirmoqchi. Talablar: maqsad=%s, budjet=%s, CPU=%s, xotira=%s, GPU=%s. Mijozning qo'shimcha talabi: %s. Shu asosida yangi konfiguratsiya tuzib, narxlarni ko'rsating.",
		nonEmpty(spec.PCType, "aniqlanmagan"),
		nonEmpty(spec.Budget, "aniqlanmagan"),
		nonEmpty(spec.CPU, "aniqlanmagan"),
		nonEmpty(spec.Storage, "aniqlanmagan"),
		nonEmpty(spec.GPU, "aniqlanmagan"),
		newValue,
	)

	response, err := h.chatUseCase.ProcessMessage(ctx, userID, username, prompt)
	if err != nil {
		log.Printf("Komponent almashtirish javobi xatosi: %v", err)
		h.sendMessage(chatID, "❌ Yangi konfiguratsiyani hisoblashda xatolik yuz berdi. Qayta urinib ko'ring.")
		return
	}

	h.sendMessage(chatID, response)

	// Feedbackni yangilab qo'yamiz
	h.saveFeedback(userID, feedbackInfo{
		Summary:    fmt.Sprintf("Maqsad: %s, Budjet: %s, CPU: %s, Xotira: %s, GPU: %s", spec.PCType, spec.Budget, spec.CPU, spec.Storage, spec.GPU),
		ConfigText: response,
		Username:   username,
		ChatID:     chatID,
		Spec:       spec,
	})
	h.sendConfigFeedbackPrompt(chatID)
}

// Foydalanuvchidan adminga xabar yuborish oqimi
func (h *BotHandler) handleUserMessageForAdmin(ctx context.Context, userID int64, username, text string, chatID int64) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		h.sendMessage(chatID, "✉️ Adminga yuborish uchun xabar matnini yozing.")
		return
	}

	h.setAwaitingAdminMessage(userID, false)

	if h.group1ChatID == 0 {
		h.sendMessage(chatID, "❌ Hozircha admin bilan bog'lanish ishlamayapti.")
		return
	}

	groupText := fmt.Sprintf("✉️ Foydalanuvchidan xabar\nUser: @%s (%d)\n\n%s",
		nonEmpty(username, "nomalum"),
		userID,
		trimmed,
	)

	sent, err := h.sendMessageWithResp(h.group1ChatID, groupText)
	if err != nil {
		h.sendMessage(chatID, "❌ Xabarni adminga yuborib bo'lmadi. Birozdan so'ng yana urinib ko'ring.")
		return
	}

	// Admin javobi foydalanuvchiga qaytishi uchun threadni saqlaymiz
	h.saveGroupThread(sent.MessageID, groupThreadInfo{
		UserID:     userID,
		UserChat:   chatID,
		Username:   username,
		Summary:    "Foydalanuvchi xabari",
		Config:     trimmed,
		CreatedAt:  time.Now(),
		AllowOrder: false,
	})

	h.sendMessage(chatID, "✅ Xabaringiz adminga yuborildi. Javob shu yerga keladi.")
}

// Order session helpers
func (h *BotHandler) startOrderSession(userID int64, info pendingApproval) {
	h.orderMu.Lock()
	defer h.orderMu.Unlock()
	h.orderSessions[userID] = &orderSession{
		Stage:     orderStageNeedName,
		Summary:   info.Summary,
		ConfigTxt: info.Config,
		Username:  info.Username,
	}
}

func (h *BotHandler) clearOrderSession(userID int64) {
	h.orderMu.Lock()
	defer h.orderMu.Unlock()
	delete(h.orderSessions, userID)
}

func (h *BotHandler) hasOrderSession(userID int64) bool {
	h.orderMu.RLock()
	defer h.orderMu.RUnlock()
	_, ok := h.orderSessions[userID]
	return ok
}

// Order flow
func (h *BotHandler) handleOrderFlow(ctx context.Context, userID int64, username, text string, chatID int64, msg *tgbotapi.Message) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	h.orderMu.Unlock()
	if !ok {
		return
	}

	switch session.Stage {
	case orderStageNeedName:
		session.Name = strings.TrimSpace(text)
		if session.Name == "" {
			h.sendMessage(chatID, "Iltimos, to'liq ismingizni yozing.")
			return
		}
		session.Stage = orderStageNeedPhone
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		h.sendPhoneRequest(chatID)
		return
	case orderStageNeedPhone:
		phone := ""
		if msg != nil && msg.Contact != nil && msg.Contact.PhoneNumber != "" {
			phone = msg.Contact.PhoneNumber
		} else {
			phone = strings.TrimSpace(text)
		}
		if phone == "" {
			h.sendPhoneRequest(chatID)
			return
		}
		session.Phone = phone
		session.Stage = orderStageNeedLocation
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		h.sendLocationRequest(chatID)
		return
	case orderStageNeedLocation:
		locText := strings.TrimSpace(text)
		if msg != nil && msg.Location != nil {
			loc := msg.Location
			locText = fmt.Sprintf("Lat: %.5f, Lon: %.5f", loc.Latitude, loc.Longitude)
		}
		if locText == "" {
			h.sendLocationRequest(chatID)
			return
		}
		session.Location = locText
		session.Stage = orderStageNeedDeliveryChoice
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		h.sendDeliveryChoice(chatID)
		return
	case orderStageNeedDeliveryChoice:
		// Kutamiz (callbacklar bilan)
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		return
	case orderStageNeedDeliveryConfirm:
		// Kutamiz (callbacklar bilan)
		h.orderMu.Lock()
		h.orderSessions[userID] = session
		h.orderMu.Unlock()
		return
	}
}

// Delivery bosqichlari callbacklari
func (h *BotHandler) handleDeliveryChoice(ctx context.Context, userID int64, choice string, chatID int64) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	if ok {
		if choice == "pickup" {
			session.Delivery = "pickup"
			h.orderSessions[userID] = session
		} else if choice == "courier" {
			session.Delivery = "courier"
			session.Stage = orderStageNeedDeliveryConfirm
			h.orderSessions[userID] = session
		}
	}
	h.orderMu.Unlock()

	if !ok {
		h.sendMessage(chatID, "Buyurtma ma'lumotlari topilmadi. /configuratsiya ni qayta bosing.")
		return
	}

	if choice == "pickup" {
		h.sendMessage(chatID, "✅ Rahmat! Buyurtmangiz 24 soat ichida tayyor bo'ladi, ertaga olib ketishingiz mumkin.")
		h.sendOrderToGroup2(userID, session, "Olib ketish", "")
		if err := h.sendSticker(chatID, orderDoneStickerID); err != nil {
			h.sendMessage(chatID, "⚠️ Stiker yuborishda xatolik yuz berdi, lekin buyurtma qabul qilindi.")
		}
		h.clearOrderSession(userID)
	} else {
		h.sendDeliveryConfirm(chatID)
	}
}

func (h *BotHandler) handleDeliveryConfirm(ctx context.Context, userID int64, agree bool, chatID int64) {
	h.orderMu.Lock()
	session, ok := h.orderSessions[userID]
	h.orderMu.Unlock()
	if !ok {
		h.sendMessage(chatID, "Buyurtma ma'lumotlari topilmadi. /configuratsiya ni qayta bosing.")
		return
	}

	if !agree {
		h.sendMessage(chatID, "Unda buyurtmani olib ketish punktidan olib keting.")
		h.sendOrderToGroup2(userID, session, "Olib ketish", "Dostavka narxiga rozilik bermadi")
		if err := h.sendSticker(chatID, orderDoneStickerID); err != nil {
			h.sendMessage(chatID, "⚠️ Stiker yuborishda xatolik yuz berdi, lekin buyurtma qabul qilindi.")
		}
		h.clearOrderSession(userID)
		return
	}

	// Ha bo'lsa
	h.sendMessage(chatID, "✅ Qabul qilindi. Buyurtmangiz rasmiylashtirilmoqda.")

	if h.group2ChatID != 0 {
		h.sendOrderToGroup2(userID, session, "Dostavka (100k)", "Rozilik berildi")
	}
	if err := h.sendSticker(chatID, orderDoneStickerID); err != nil {
		h.sendMessage(chatID, "⚠️ Stiker yuborishda xatolik yuz berdi, lekin buyurtma qabul qilindi.")
	}
	h.clearOrderSession(userID)
}

// UI helpers for order flow
func (h *BotHandler) sendPhoneRequest(chatID int64) {
	btn := tgbotapi.NewKeyboardButtonContact("📞 Telefon raqamni jo'natish")
	kb := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(btn))
	kb.OneTimeKeyboard = true
	msg := tgbotapi.NewMessage(chatID, "📞 Telefon raqamingizni yuboring (yoki button bosib jo'nating).")
	msg.ReplyMarkup = kb
	h.bot.Send(msg)
}

func (h *BotHandler) sendLocationRequest(chatID int64) {
	locBtn := tgbotapi.NewKeyboardButtonLocation("📍 Lokatsiyani yuborish")
	kb := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(locBtn))
	kb.OneTimeKeyboard = true
	msg := tgbotapi.NewMessage(chatID, "📍 Lokatsiyangizni yuboring yoki manzilni matn ko'rinishida yozing.")
	msg.ReplyMarkup = kb
	h.bot.Send(msg)
}

func (h *BotHandler) sendDeliveryChoice(chatID int64) {
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏬 Olib ketaman", "delivery_pickup"),
			tgbotapi.NewInlineKeyboardButtonData("🚚 Dostavka", "delivery_courier"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Mahsulotni qanday olasiz?")
	msg.ReplyMarkup = markup
	h.bot.Send(msg)
}

func (h *BotHandler) sendDeliveryConfirm(chatID int64) {
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ha ✅", "delivery_confirm_yes"),
			tgbotapi.NewInlineKeyboardButtonData("Yo'q ❌", "delivery_confirm_no"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Yetkazib berish narxi manzilingizga qarab kelishiladi rozimisiz")
	msg.ReplyMarkup = markup
	h.bot.Send(msg)
}

// Send order summary to group 2
func (h *BotHandler) sendOrderToGroup2(userID int64, session *orderSession, delivery string, note string) {
	if h.group2ChatID == 0 {
		return
	}
	summary := nonEmpty(session.ConfigTxt, session.Summary)
	orderText := fmt.Sprintf(
		"🧾 Yangi buyurtma\nUser ID: %d (@%s)\nIsm: %s\nTelefon: %s\nLokatsiya: %s\nYetkazish: %s\nIzoh: %s\n\n%s",
		userID,
		nonEmpty(session.Username, "nomalum"),
		nonEmpty(session.Name, "ko'rsatilmagan"),
		nonEmpty(session.Phone, "ko'rsatilmagan"),
		nonEmpty(session.Location, "ko'rsatilmagan"),
		delivery,
		nonEmpty(note, "-"),
		summary,
	)
	h.sendMessage(h.group2ChatID, orderText)
}

// Sticker helper
func (h *BotHandler) sendSticker(chatID int64, stickerID string) error {
	st := tgbotapi.NewSticker(chatID, tgbotapi.FileID(stickerID))
	_, err := h.bot.Send(st)
	if err != nil {
		log.Printf("Stiker yuborishda xatolik: %v", err)
	}
	return err
}

// Reminder helpers
func (h *BotHandler) wasConfigReminded(chatID int64) bool {
	h.reminderMu.RLock()
	defer h.reminderMu.RUnlock()
	return h.configReminded[chatID]
}

func (h *BotHandler) markConfigReminded(chatID int64) {
	h.reminderMu.Lock()
	defer h.reminderMu.Unlock()
	h.configReminded[chatID] = true
}

// Komponent almashtirish tugmalari
func (h *BotHandler) sendChangeComponentPrompt(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Qaysi komponentni almashtiraylik?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("CPU", "cfg_change_cpu"),
			tgbotapi.NewInlineKeyboardButtonData("GPU", "cfg_change_gpu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("RAM", "cfg_change_ram"),
			tgbotapi.NewInlineKeyboardButtonData("SSD", "cfg_change_ssd"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Boshqa", "cfg_change_other"),
		),
	)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Komponent tugmalarini yuborishda xatolik: %v", err)
	}
}

// isConfigRequest foydalanuvchi xabari konfiguratsiya so'rovi ekanini aniqlash
func isConfigRequest(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"pc yig", "kompyuter yig", "pc build", "pc konfigur", "konfiguratsiya",
		"sborka", "sbor", "pc sbor", "pc sborka", "pc topla", "kompyuter topla",
		"pc kerak", "pc kera", "kompyuter kerak", "kompyuter kera",
		"gaming pc", "office pc", "montaj pc", "pc config", "pc setup",
		"kompyuter sborka", "kompyuter sbor", "pc yig'ib", "pc yigib", "pc yiqib",
		"$1k", "1k$", "1000$", "budjet", "byudjet", "budget",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	// Kombinatsion tekshiruv: "pc"/"kompyuter" + "yig"/"topla"/"config"
	hasPC := strings.Contains(lower, "pc") || strings.Contains(lower, "kompyuter") || strings.Contains(lower, "komp")
	hasBuild := strings.Contains(lower, "yig") || strings.Contains(lower, "topla") || strings.Contains(lower, "config") || strings.Contains(lower, "setup") || strings.Contains(lower, "sbor")
	hasNeed := strings.Contains(lower, "kerak") || strings.Contains(lower, "kera")
	hasBudget := strings.Contains(lower, "$") || strings.Contains(lower, "usd") || strings.Contains(lower, "so'm") || strings.Contains(lower, "soum") || strings.Contains(lower, "sum") || strings.Contains(lower, "uzs")
	return hasPC && (hasBuild || hasNeed || hasBudget)
}

// Hech bo'lmaganda asosiy komponent kalit so'zlari bor javob konfiguratsiya ekanini anglatadi
func isLikelyConfigResponse(text string) bool {
	lower := strings.ToLower(text)
	keys := []string{"protsessor", "ona plata", "motherboard", "gpu", "video karta", "ram", "ssd", "hdd", "jami", "konfiguratsiya"}
	match := 0
	for _, k := range keys {
		if strings.Contains(lower, k) {
			match++
		}
	}
	return match >= 2
}

// Group thread mapping
func (h *BotHandler) saveGroupThread(messageID int, info groupThreadInfo) {
	h.groupMu.Lock()
	defer h.groupMu.Unlock()
	h.groupThreads[messageID] = info
}

func (h *BotHandler) getGroupThread(messageID int) (groupThreadInfo, bool) {
	h.groupMu.RLock()
	defer h.groupMu.RUnlock()
	info, ok := h.groupThreads[messageID]
	return info, ok
}

// Pending approval mapping
func (h *BotHandler) savePendingApproval(userID int64, info pendingApproval) {
	h.approvalMu.Lock()
	defer h.approvalMu.Unlock()
	h.pendingApprove[userID] = info
}

func (h *BotHandler) popPendingApproval(userID int64) (pendingApproval, bool) {
	h.approvalMu.Lock()
	defer h.approvalMu.Unlock()
	info, ok := h.pendingApprove[userID]
	if ok {
		delete(h.pendingApprove, userID)
	}
	return info, ok
}

// Admin tasdig'i so'rovlari mapping
func (h *BotHandler) saveAdminApproval(messageID int, req adminApprovalRequest) {
	h.adminApprovalMu.Lock()
	defer h.adminApprovalMu.Unlock()
	h.adminApprovals[messageID] = req
}

func (h *BotHandler) popAdminApproval(messageID int) (adminApprovalRequest, bool) {
	h.adminApprovalMu.Lock()
	defer h.adminApprovalMu.Unlock()
	req, ok := h.adminApprovals[messageID]
	if ok {
		delete(h.adminApprovals, messageID)
	}
	return req, ok
}

// Groupdagi xabarlarni qayta ishlash (AI yo'q)
func (h *BotHandler) handleGroupMessage(ctx context.Context, message *tgbotapi.Message) {
	// Faqat bot yuborgan xabarga reply qilingan bo'lsa ishlaymiz
	if message.ReplyToMessage == nil {
		return
	}
	targetInfo, ok := h.getGroupThread(message.ReplyToMessage.MessageID)
	if !ok {
		return
	}

	adminText := strings.TrimSpace(message.Text)
	if adminText == "" && message.Caption != "" {
		adminText = strings.TrimSpace(message.Caption)
	}
	if adminText == "" {
		return
	}

	req := adminApprovalRequest{
		Target:   targetInfo,
		AdminMsg: adminText,
	}
	h.saveAdminApproval(message.MessageID, req)

	prompt := tgbotapi.NewMessage(message.Chat.ID, "Foydalanuvchiga rasmiylashtirishni so'rashga ruxsat berasizmi?")
	prompt.ReplyToMessageID = message.MessageID
	prompt.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ha ✅", fmt.Sprintf("adm_approve_yes:%d", message.MessageID)),
			tgbotapi.NewInlineKeyboardButtonData("Yo'q ❌", fmt.Sprintf("adm_approve_no:%d", message.MessageID)),
		),
	)

	if _, err := h.bot.Send(prompt); err != nil {
		log.Printf("Admin tasdiqi xabarini yuborishda xatolik: %v", err)
	}
}

func (h *BotHandler) sendAdminReplyToUser(target groupThreadInfo, adminMsg string, askOrder bool) {
	replyText := fmt.Sprintf("🔔 Admindan javob:\n%s", nonEmpty(adminMsg, "javob matni yo'q"))

	var markup tgbotapi.InlineKeyboardMarkup

	if askOrder {
		replyText += "\n\nRasmiylashtiramizmi?"
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Ha ✅", "order_yes"),
				tgbotapi.NewInlineKeyboardButtonData("Yo'q ❌", "order_no"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✉️ Adminga yozish", "msg_admin_start"),
			),
		)

		h.savePendingApproval(target.UserID, pendingApproval{
			UserID:   target.UserID,
			UserChat: target.UserChat,
			Summary:  target.Summary,
			Config:   target.Config,
			Username: target.Username,
			SentAt:   time.Now(),
		})
	} else {
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✉️ Adminga yozish", "msg_admin_start"),
			),
		)
	}

	msg := tgbotapi.NewMessage(target.UserChat, replyText)
	msg.ReplyMarkup = markup

	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Foydalanuvchiga admin javobi yuborilmadi: %v", err)
	}
}

func (h *BotHandler) completeAdminApprovalPrompt(chatID int64, messageID int, text string) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{}

	if _, err := h.bot.Send(edit); err != nil {
		log.Printf("Admin tasdig'i xabarini yangilashda xatolik: %v", err)
	}
}

func (h *BotHandler) handleAdminApprovalCallback(reqID int, allowOrder bool, chatID int64, promptMessageID int) bool {
	req, ok := h.popAdminApproval(reqID)
	if !ok {
		return false
	}

	h.sendAdminReplyToUser(req.Target, req.AdminMsg, allowOrder)

	status := "ℹ️ Faqat admin javobi yuborildi."
	if allowOrder {
		status = "✅ Foydalanuvchiga yuborildi va rasmiylashtirish so'raldi."
	}
	h.completeAdminApprovalPrompt(chatID, promptMessageID, status)

	return true
}

func buildAdminMessagesDigest(msgs []entity.Message, maxLen int) string {
	var sb strings.Builder
	sb.WriteString("🗂 Oxirgi yozishmalar:\n\n")

	userSeen := make(map[int64]int)
	for _, m := range msgs {
		if userSeen[m.UserID] >= 3 {
			continue
		}
		userSeen[m.UserID]++

		entry := fmt.Sprintf("@%s (%d)\n🕒 %s\n👤: %s\n🤖: %s\n\n",
			nonEmpty(m.Username, "nomalum"),
			m.UserID,
			m.Timestamp.Format("02 Jan 15:04"),
			truncateString(m.Text, 180),
			truncateString(m.Response, 180),
		)

		if maxLen > 0 && sb.Len()+len(entry) > maxLen {
			sb.WriteString("…")
			break
		}
		sb.WriteString(entry)
	}

	if len(userSeen) == 0 {
		return "Yozishmalar topilmadi."
	}

	return sb.String()
}

func truncateString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

type adminUserSummary struct {
	UserID   int64
	Username string
	LastAt   time.Time
}

func collectUsersFromMessages(msgs []entity.Message) []adminUserSummary {
	m := make(map[int64]adminUserSummary)
	for _, msg := range msgs {
		cur, ok := m[msg.UserID]
		if !ok || msg.Timestamp.After(cur.LastAt) {
			m[msg.UserID] = adminUserSummary{
				UserID:   msg.UserID,
				Username: msg.Username,
				LastAt:   msg.Timestamp,
			}
		}
	}

	var list []adminUserSummary
	for _, v := range m {
		list = append(list, v)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].LastAt.After(list[j].LastAt)
	})
	return list
}

func buildUserButtons(users []adminUserSummary) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{}
	row := []tgbotapi.InlineKeyboardButton{}

	for _, u := range users {
		label := fmt.Sprintf("@%s (%d)", nonEmpty(u.Username, "nomalum"), u.UserID)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("admin_msgs_user:%d", u.UserID))
		row = append(row, btn)
		if len(row) == 2 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (h *BotHandler) handleAdminUserMessages(chatID int64, data string) {
	idStr := strings.TrimPrefix(data, "admin_msgs_user:")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.sendMessage(chatID, "❌ Foydalanuvchi aniqlanmadi.")
		return
	}

	msgs, err := h.chatUseCase.GetHistory(context.Background(), userID)
	if err != nil {
		h.sendMessage(chatID, "❌ Yozishmalarni yuklab bo'lmadi.")
		return
	}
	if len(msgs) == 0 {
		h.sendMessage(chatID, "Bu foydalanuvchi uchun yozishmalar topilmadi.")
		return
	}

	text := buildUserConversationDigest(msgs, 3900)
	h.sendMessage(chatID, text)
}

func buildUserConversationDigest(msgs []entity.Message, maxLen int) string {
	if len(msgs) == 0 {
		return "Yozishmalar topilmadi."
	}
	var sb strings.Builder
	user := msgs[0]
	sb.WriteString(fmt.Sprintf("👤 @%s (%d)\n\n", nonEmpty(user.Username, "nomalum"), user.UserID))

	for _, m := range msgs {
		entry := fmt.Sprintf("🕒 %s\n👤: %s\n🤖: %s\n\n",
			m.Timestamp.Format("02 Jan 15:04"),
			truncateString(m.Text, 280),
			truncateString(m.Response, 280),
		)
		if maxLen > 0 && sb.Len()+len(entry) > maxLen {
			sb.WriteString("…")
			break
		}
		sb.WriteString(entry)
	}
	return sb.String()
}

func buildProductPreview(products []entity.Product, limit int) string {
	if limit > 0 && len(products) > limit {
		products = products[:limit]
	}
	var sb strings.Builder
	for i, p := range products {
		sb.WriteString(fmt.Sprintf("%d) %s - $%.2f", i+1, p.Name, p.Price))
		if p.Stock > 0 {
			sb.WriteString(fmt.Sprintf(" (Ombor: %d)", p.Stock))
		}
		if p.Description != "" {
			sb.WriteString(fmt.Sprintf("\n   %s", truncateString(p.Description, 120)))
		}
		if len(p.Specs) > 0 {
			sb.WriteString("\n   Specs: ")
			specs := []string{}
			for k, v := range p.Specs {
				specs = append(specs, fmt.Sprintf("%s=%s", k, v))
			}
			sort.Strings(specs)
			sb.WriteString(truncateString(strings.Join(specs, ", "), 160))
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func shouldOfferPurchase(userText, response string) bool {
	lt := strings.ToLower(userText)
	lr := strings.ToLower(response)

	queryIntent := strings.Contains(lt, "bormi") ||
		strings.Contains(lt, "bormi?") ||
		strings.Contains(lt, "bor mi") ||
		strings.Contains(lt, "olmoq") ||
		strings.Contains(lt, "sotib") ||
		strings.Contains(lt, "narx") ||
		strings.Contains(lt, "qancha")

	responseSignals := strings.Contains(lr, "bizda bor") ||
		strings.Contains(lr, "mavjud") ||
		strings.Contains(lr, "$") ||
		strings.Contains(lr, "sotib ol") ||
		strings.Contains(lr, "xarid")

	negative := strings.Contains(lr, "mavjud emas") ||
		strings.Contains(lr, "emas") ||
		strings.Contains(lr, "yo'q") ||
		strings.Contains(lr, "yoq") ||
		strings.Contains(lr, "afsus") ||
		strings.Contains(lr, "topilmadi") ||
		strings.Contains(lr, "mavjud emas")

	return queryIntent && responseSignals && !negative
}

func isProductSearchQuery(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"rtx", "gtx", "rx", "gpu", "video karta", "videokarta", "grafik",
		"ssd", "hdd", "nvme",
		"ram", "ddr4", "ddr5",
		"intel", "core i3", "i5", "i7", "i9",
		"ryzen", "r3", "r5", "r7", "r9",
		"monitor", "ekran",
		"cpu", "processor", "protsessor",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	// Model raqamga qarash (4 raqam ketma-ketligi)
	digitStreak := 0
	for _, ch := range lower {
		if ch >= '0' && ch <= '9' {
			digitStreak++
			if digitStreak >= 4 {
				return true
			}
		} else {
			digitStreak = 0
		}
	}
	return false
}

// extractProductKeywords foydalanuvchi matnidan faqat mahsulot kalit so'zlarini ajratib oladi
func extractProductKeywords(text string) string {
	lower := strings.ToLower(text)

	// Mahsulot nomlari va kalit so'zlar
	productKeywords := []string{
		// GPU
		"rtx 5090", "rtx 4090", "rtx 4080", "rtx 4070", "rtx 4060", "rtx 3090", "rtx 3080", "rtx 3070", "rtx 3060",
		"gtx 1660", "gtx 1650", "rx 7900", "rx 7800", "rx 6800", "rx 6700",
		// CPU
		"core i9", "core i7", "core i5", "core i3",
		"ryzen 9", "ryzen 7", "ryzen 5", "ryzen 3",
		"i9-14900", "i9-13900", "i7-14700", "i7-13700", "i5-14600", "i5-13600",
		// RAM
		"ddr5 32gb", "ddr5 16gb", "ddr4 32gb", "ddr4 16gb", "ddr4 8gb",
		// Storage
		"nvme 2tb", "nvme 1tb", "nvme 512gb", "ssd 1tb", "ssd 512gb", "hdd 2tb", "hdd 1tb",
		// Monitor
		"144hz", "165hz", "240hz", "27\"", "32\"",
	}

	// Uzun kalit so'zlardan boshlaymiz (masalan "core i5" ni "i5" dan oldin)
	for _, kw := range productKeywords {
		if strings.Contains(lower, kw) {
			return kw
		}
	}

	// Qisqa kalit so'zlar (agar uzun topilmasa)
	shortKeywords := []string{
		"rtx", "gtx", "rx",
		"i9", "i7", "i5", "i3",
		"ryzen 9", "ryzen 7", "ryzen 5", "ryzen 3",
		"ddr5", "ddr4",
		"nvme", "ssd", "hdd",
		"monitor", "gpu", "cpu", "processor", "protsessor",
		"video karta", "videokarta",
	}

	for _, kw := range shortKeywords {
		if strings.Contains(lower, kw) {
			// Atrofidagi so'zlarni ham qo'shamiz (kontekst uchun)
			parts := strings.Fields(lower)
			var result []string
			for i, part := range parts {
				if strings.Contains(part, kw) || (i > 0 && strings.Contains(parts[i-1], kw)) {
					// Raqamlar va harflarni saqlaymiz
					cleaned := strings.Trim(part, "?,!.;:")
					if len(cleaned) > 0 {
						result = append(result, cleaned)
					}
				}
			}
			if len(result) > 0 {
				return strings.Join(result, " ")
			}
			return kw
		}
	}

	// Raqamli model (4+ raqam ketma-ketligi)
	words := strings.Fields(lower)
	for _, word := range words {
		digitCount := 0
		for _, ch := range word {
			if ch >= '0' && ch <= '9' {
				digitCount++
			}
		}
		if digitCount >= 4 {
			return word
		}
	}

	return ""
}

func isQuotaError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "quota") || strings.Contains(msg, "retry in") || strings.Contains(msg, "rate limit")
}

// getWelcomeMessage salom xabari
func (h *BotHandler) getWelcomeMessage() string {
	return `Assalomu alaykum! 👋

Men kompyuter texnikasi bo'yicha professional AI do'konchiman.

Sizga quyidagilar bo'yicha yordam berishim mumkin:
• Kompyuter va noutbuklar tanlash
• CPU, GPU, RAM, SSD va boshqa komponentlar haqida ma'lumot
• Texnik spetsifikatsiyalar va taqqoslashlar
• Narxlar va tavsiyalar
• /configuratsiya komandasi orqali bosqichma-bosqich PC yig'ish

Menga savolingizni yozing va men sizga yordam beraman! 💻`
}

// getHelpMessage yordam xabari
func (h *BotHandler) getHelpMessage() string {
	return `🤖 *Bot komandlari:*

📱 Asosiy:
/start - Botni qayta ishga tushirish
/help - Yordam va komandalar ro'yxati
/clear - Chat tarixini tozalash
/history - Chat tarixini ko'rish
/configuratsiya - PC yig'ish uchun bosqichma-bosqich sozlash

🔐 Admin:
/admin - Admin panelga kirish
/logout - Admin paneldan chiqish
/catalog - Katalog haqida ma'lumot (admin)
/products - Barcha mahsulotlar

*Qanday foydalanish:*
Menga oddiy xabar yuboring va men sizga javob beraman. Masalan:
• "Gaming uchun kompyuter tavsiya qiling"
• "RTX 4070 haqida ma'lumot bering"
• "16GB RAM yetadimi?"

Men sizning savollaringizni saqlayman, shuning uchun kontekstni eslab qolaman! 💡`
}

// GetBotUsername bot username ni olish
func (h *BotHandler) GetBotUsername() string {
	return h.bot.Self.UserName
}
