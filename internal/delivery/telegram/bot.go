package telegram

import (
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/generative-ai-go/genai"
	"github.com/yourusername/telegram-ai-bot/internal/usecase"
)

// BotHandler Telegram bot handler
type BotHandler struct {
	bot                  *tgbotapi.BotAPI
	group1ChatID         int64
	group1ThreadID       int
	group2ChatID         int64
	group2ThreadID       int
	group3ChatID         int64
	group3ThreadID       int
	group4ChatID         int64
	group4ThreadID       int
	group5ChatID         int64
	group5ThreadID       int
	activeOrdersChatID   int64
	activeOrdersThreadID int
	profileMu            sync.RWMutex
	profiles             map[int64]userProfile
	profileStage         map[int64]string
	profileMeta          map[int64]profileMeta
	chatUseCase          usecase.ChatUseCase
	adminUseCase         usecase.AdminUseCase
	productUseCase       usecase.ProductUseCase
	configBuilder        *ConfigurationBuilder
	configMu             sync.RWMutex
	configSessions       map[int64]*configSession
	configOrderMu        sync.RWMutex
	configOrderLocked    map[int64]bool
	feedbackMu           sync.RWMutex
	feedbacks            map[int64]feedbackInfo // legacy (latest per user)
	feedbackByID         map[string]feedbackInfo
	feedbackLatest       map[int64]string
	groupMu              sync.RWMutex
	groupThreads         map[int]groupThreadInfo
	approvalMu           sync.RWMutex
	pendingApprove       map[int64]pendingApproval
	reminderMu           sync.RWMutex
	configReminded       map[int64]bool
	reminderTemplates    []string
	reminderInput        map[int64]*reminderInputState
	reminderInterval     time.Duration
	reminderEnabled      bool

	changeMu         sync.RWMutex
	pendingChange    map[int64]changeRequest
	orderMu          sync.RWMutex
	orderSessions    map[int64]*orderSession
	orderCleanup     map[int64]orderFormCleanup
	orderStatusMu    sync.RWMutex
	orderStatuses    map[string]orderStatusInfo // legacy
	pendingETAMu     sync.RWMutex
	pendingETAs      map[int64]string
	pendingETAChat   map[string]string
	processingMu     sync.RWMutex
	processing       map[int64]bool
	warnMu           sync.RWMutex
	processingWarn   map[int64]int
	warnMsgs         map[int64][]waitingMessage
	waitingMu        sync.RWMutex
	waitingMsgs      map[int64]waitingMessage
	suggestionMu     sync.RWMutex
	lastSuggestion   map[int64]string
	adminMenuMu      sync.RWMutex
	adminMenuMsgs    map[int64]adminMenuMessage
	adminMsgMu       sync.RWMutex
	adminMessages    map[int64][]adminMessage
	adminActive      map[int64]bool
	adminAuthMu      sync.RWMutex
	adminAuthorized  map[int64]bool
	searchMu         sync.RWMutex
	searchAwait      map[int64]bool
	userHistoryMu    sync.RWMutex
	userHistoryAwait map[int64]bool
	userHistoryMsgMu sync.RWMutex
	userHistoryMsgs  map[string][]adminMessage
	addProductMu     sync.RWMutex
	addProductState  map[int64]*addProductState
	ctaMu            sync.RWMutex
	configCTAMsg     map[int64]int
	lastUserMsg      map[int64]int
	cartMu           sync.RWMutex
	cartItems        map[int64][]cartItem
	currencyMu       sync.RWMutex
	currencyMode     string
	currencyRate     float64
	currencyAwait    map[int64]bool
	purchaseMu       sync.RWMutex
	purchasePrompt   map[int64]string
	purchaseTitle    map[int64]string
	purchaseMsgMu    sync.RWMutex
	purchaseMsg      map[int64]purchasePromptMessage

	orderCounterMu sync.Mutex
	orderCounter   map[string]int

	sheetMasterMu  sync.RWMutex
	sheetMasterCfg *sheetMasterConfig

	stickerMu    sync.RWMutex
	stickerCfg   *stickerConfig
	stickerAwait map[int64]stickerSlot

	group1PendingMu       sync.RWMutex
	group1PendingApprovals map[int]struct{}

	sheetMasterSetupMu sync.RWMutex
	sheetMasterSetup   map[int64]*sheetMasterSetupState

	sheetMasterSyncMu sync.Mutex
	aboutUserSyncMu   sync.Mutex
	aboutUserSyncTimer *time.Timer
	aboutUserSyncPending bool
	aboutUserSyncLast time.Time

	importAutoMu            sync.RWMutex
	importAutoEnabled       bool
	importAutoInterval      time.Duration
	importAutoStop          chan struct{}
	importAutoRunAsUserID   int64
	importAutoLastFileID    uint
	importAutoLastUpdatedAt time.Time
	importAutoLastRunAt     time.Time
	importAutoLastCount     int
	importAutoLastErr       string
	importAutoInput         map[int64]*importAutoInputState

	lastSeenMu   sync.RWMutex
	lastSeen     map[int64]time.Time
	nameMu       sync.RWMutex
	lastName     map[int64]string
	liveFeedMu   sync.RWMutex
	liveFeedSent map[int64]time.Time

	// Admin login kutilayotgan userlar
	awaitingPassword map[int64]bool
	mu               sync.RWMutex
	welcomeMu        sync.RWMutex
	welcomeMsgs      map[int64][]int

	// Performance optimizations
	workerPool *workerPool
	cache      *responseCache

	// AI client for SmartRouter
	geminiClient *genai.Client

	// User language preferences
	langMu   sync.RWMutex
	userLang map[int64]string

	// Bot start timestamp (used for chat history filtering)
	botStartedAt time.Time

	orderStore OrderStore
	chatStore  ChatStore

	// Konfiguratsiya yakunidan keyingi avtomatik eslatmalar
	configReminder map[int64]*time.Timer
}

// NewBotHandler yangi bot handler yaratish
func NewBotHandler(
	token string,
	group1ChatID int64,
	group1ThreadID int,
	group2ChatID int64,
	group2ThreadID int,
	group3ChatID int64,
	group3ThreadID int,
	group4ChatID int64,
	group4ThreadID int,
	group5ChatID int64,
	group5ThreadID int,
	chatUseCase usecase.ChatUseCase,
	adminUseCase usecase.AdminUseCase,
	productUseCase usecase.ProductUseCase,
	geminiClient *genai.Client,
) (*BotHandler, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	orderStore, err := newOrderStoreFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to init order store: %w", err)
	}
	chatStore, err := newChatStoreFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to init chat store: %w", err)
	}

	handler := &BotHandler{
		bot:                bot,
		group1ChatID:       group1ChatID,
		group1ThreadID:     group1ThreadID,
		group2ChatID:       group2ChatID,
		group2ThreadID:     group2ThreadID,
		group3ChatID:       group3ChatID,
		group3ThreadID:     group3ThreadID,
		group4ChatID:       group4ChatID,
		group4ThreadID:     group4ThreadID,
		group5ChatID:       group5ChatID,
		group5ThreadID:     group5ThreadID,
		chatUseCase:        chatUseCase,
		adminUseCase:       adminUseCase,
		productUseCase:     productUseCase,
		configBuilder:      NewConfigurationBuilder(productUseCase),
		geminiClient:       geminiClient,
		configSessions:     make(map[int64]*configSession),
		configOrderLocked:  make(map[int64]bool),
		feedbacks:          make(map[int64]feedbackInfo),
		feedbackByID:       make(map[string]feedbackInfo),
		feedbackLatest:     make(map[int64]string),
		groupThreads:       make(map[int]groupThreadInfo),
		pendingApprove:     make(map[int64]pendingApproval),
		configReminded:     make(map[int64]bool),
		pendingChange:      make(map[int64]changeRequest),
		orderSessions:      make(map[int64]*orderSession),
		orderCleanup:       make(map[int64]orderFormCleanup),
		orderStatuses:      make(map[string]orderStatusInfo),
		pendingETAs:        make(map[int64]string),
		pendingETAChat:     make(map[string]string),
		processing:         make(map[int64]bool),
		processingWarn:     make(map[int64]int),
		warnMsgs:           make(map[int64][]waitingMessage),
		waitingMsgs:        make(map[int64]waitingMessage),
		lastSuggestion:     make(map[int64]string),
		adminMenuMsgs:      make(map[int64]adminMenuMessage),
		adminMessages:      make(map[int64][]adminMessage),
		adminActive:        make(map[int64]bool),
		adminAuthorized:    make(map[int64]bool),
		searchAwait:        make(map[int64]bool),
		userHistoryAwait:   make(map[int64]bool),
		userHistoryMsgs:    make(map[string][]adminMessage),
		addProductState:    make(map[int64]*addProductState),
		configCTAMsg:       make(map[int64]int),
		lastUserMsg:        make(map[int64]int),
		cartItems:          make(map[int64][]cartItem),
		currencyMode:       "usd",
		currencyRate:       0,
		currencyAwait:      make(map[int64]bool),
		purchasePrompt:     make(map[int64]string),
		purchaseTitle:      make(map[int64]string),
		purchaseMsg:        make(map[int64]purchasePromptMessage),
		stickerAwait:       make(map[int64]stickerSlot),
		group1PendingApprovals: make(map[int]struct{}),
		sheetMasterSetup:   make(map[int64]*sheetMasterSetupState),
		lastSeen:           make(map[int64]time.Time),
		lastName:           make(map[int64]string),
		liveFeedSent:       make(map[int64]time.Time),
		awaitingPassword:   make(map[int64]bool),
		welcomeMsgs:        make(map[int64][]int),
		cache:              newResponseCache(defaultCacheTTL, defaultMaxCacheSize),
		userLang:           make(map[int64]string),
		botStartedAt:       time.Now(),
		orderStore:         orderStore,
		chatStore:          chatStore,
		configReminder:     make(map[int64]*time.Timer),
		reminderInput:      make(map[int64]*reminderInputState),
		reminderInterval:   defaultReminderInterval,
		reminderEnabled:    true,
		orderCounter:       make(map[string]int),
		profiles:           make(map[int64]userProfile),
		profileStage:       make(map[int64]string),
		profileMeta:        make(map[int64]profileMeta),
		importAutoInput:    make(map[int64]*importAutoInputState),
		importAutoInterval: defaultImportAutoInterval,
	}

	// Active orders chat: ustuvor group4, aks holda group2
	handler.activeOrdersChatID = handler.group2ChatID
	handler.activeOrdersThreadID = handler.group2ThreadID
	if handler.group4ChatID != 0 {
		handler.activeOrdersChatID = handler.group4ChatID
		handler.activeOrdersThreadID = handler.group4ThreadID
	}

	// Initialize worker pool
	handler.workerPool = newWorkerPool(handler, defaultWorkerCount)

	// Load SheetMaster config from disk (optional)
	handler.loadSheetMasterConfigFromDisk()
	handler.loadStickerConfigFromDisk()

	return handler, nil
}

// GetBotUsername returns the bot's username from Telegram API state.
func (h *BotHandler) GetBotUsername() string {
	return h.bot.Self.UserName
}
