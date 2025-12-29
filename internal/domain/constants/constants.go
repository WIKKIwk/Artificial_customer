package constants

// Chat va Context konstantalari
const (
	// DefaultMaxContextSize chat tarixida saqlanadigan max xabarlar soni
	DefaultMaxContextSize = 60

	// DefaultMaxHistoryMessages tarixda ko'rsatiladigan max xabarlar
	DefaultMaxHistoryMessages = 10
)

// Admin konstantalari
const (
	// DefaultSessionTimeout admin session timeout (soatlarda)
	DefaultSessionTimeout = 24

	// MaxFileUploadSize maksimal fayl hajmi (bayt)
	MaxFileUploadSize = 5 * 1024 * 1024 // 5MB
)

// AI Model konstantalari
const (
	// GeminiModelName Gemini AI model nomi
	GeminiModelName = "gemini-2.5-flash"

	// AITemperature AI javob aniqlik darajasi (0.0-1.0)
	AITemperature = 0.3

	// AITopK Top-K sampling parametri
	AITopK = 20

	// AITopP Top-P sampling parametri
	AITopP = 0.9

	// MaxRetries AI ga so'rov yuborish uchun max urinishlar
	MaxRetries = 3

	// RetryDelay har bir urinish o'rtasidagi kutish vaqti (soniya)
	RetryDelay = 10
)

// Xabar konstantalari
const (
	// AdminContactPhone admin telefon raqami
	AdminContactPhone = "+998888170131"
)
