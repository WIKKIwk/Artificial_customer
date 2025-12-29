package telegram

import "time"

type configStage int

const (
	configStageNeedName configStage = iota
	configStageNeedType
	configStageNeedBudget
	configStageNeedColor
	configStageNeedCPU
	configStageNeedCPUCooler
	configStageNeedStorage
	configStageNeedGPU
	configStageNeedMonitor
	configStageNeedMonitorHz
	configStageNeedMonitorDisplay
	configStageNeedPeripherals
)

type configSession struct {
	Stage           configStage
	Name            string
	PCType          string
	Budget          string
	Color           string
	CPUBrand        string
	CPUCooler       string
	Storage         string
	GPUBrand        string
	NeedMonitor     bool   // Monitor kerakmi?
	MonitorHz       string // 60Hz, 144Hz, 240Hz
	MonitorDisplay  string // IPS, VA, TN
	NeedPeripherals bool   // Peripherals kerakmi?
	Mouse           string // Sichqoncha
	Keyboard        string // Klaviatura
	Headphones      string // Quloqchin
	StartedAt       time.Time
	LastUpdate      time.Time
	Inline          bool
	MessageID       int
	ChatID          int64
}

type feedbackInfo struct {
	Summary    string
	ConfigText string
	Username   string
	Phone      string
	ChatID     int64
	Spec       configSpec
	OfferID    string
	OrderID    string
}

type groupThreadInfo struct {
	UserID    int64
	UserChat  int64
	Username  string
	Summary   string
	Config    string
	ChatID    int64
	ThreadID  int
	OrderID   string
	CreatedAt time.Time
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
	Name    string
	PCType  string
	Budget  string
	CPU     string
	RAM     string
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
	ChatID    int64
	MessageID int
	FromCart  bool // savatchadan rasmiylashtirilgan (checkout_all) order

	InventoryReserved bool
	ReservedItems     []string
	FormMessageIDs    []int
}

type orderFormCleanup struct {
	ChatID     int64
	MessageIDs []int
}

type waitingMessage struct {
	ChatID    int64
	MessageID int
}

type orderStatusInfo struct {
	UserID          int64
	UserChat        int64
	Username        string
	Phone           string
	Location        string
	Summary         string
	StatusSummary   string
	Config          string // PC konfiguratsiya teksti (to'liq)
	OrderID         string
	IsSingleItem    bool
	Delivery        string
	Total           string
	Status          string
	ActiveChatID    int64
	ActiveThreadID  int
	ActiveMessageID int
	ETAPromptChatID int64
	ETAPromptThread int
	ETAPromptMsgID  int
	CreatedAt       time.Time
}

type adminMenuMessage struct {
	chatID    int64
	messageID int
}

type userProfile struct {
	Name  string
	Phone string
}

type profileMeta struct {
	PromptMsgID int
}

type adminMessage struct {
	chatID int64
	msgID  int
}

type addProductStage int

const (
	addProductStageNeedSelect addProductStage = iota
	addProductStageNeedQty
)

type addProductState struct {
	ChatID      int64
	ProductID   string
	ProductName string
	Stage       addProductStage
	Delta       int
}

type cartItem struct {
	Title string
	Text  string
}

type reminderInputStage int

const (
	reminderStageNeedCount reminderInputStage = iota
	reminderStageNeedMessages
)

type reminderInputState struct {
	stage    reminderInputStage
	expected int
	messages []string
	chatID   int64
}

type purchasePromptMessage struct {
	chatID    int64
	messageID int
}

const (
	reminderMaxCount        = 20
	defaultReminderInterval = 5 * time.Minute
	minReminderInterval     = time.Minute
	maxReminderInterval     = 24 * time.Hour
)
