package telegram

import (
	"context"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

// SmartRouter - aqlli router foydalanuvchi xabarini tahlil qiladi va to'g'ri oqimga yo'naltiradi
type SmartRouter struct {
	aiClient *genai.Client
	chatRepo repository.ChatRepository
}

// MessageIntent foydalanuvchi niyatini aniqlaydi
type MessageIntent int

const (
	// IntentProductSearch - mahsulot qidiruv (gpu bormi?, ram kerak)
	IntentProductSearch MessageIntent = iota
	// IntentPCBuildRequest - yangi PC yig'ish so'rovi
	IntentPCBuildRequest
	// IntentQuestion - savol (bu yaxshimi?, qanday?, farqi?)
	IntentQuestion
	// IntentHistoryReference - oldingi xabarga murojaat (yuqoridagi, shu)
	IntentHistoryReference
	// IntentNormalChat - oddiy suhbat
	IntentNormalChat
)

// DetectIntent foydalanuvchi xabaridan niyatni aniqlaydi
func (sr *SmartRouter) DetectIntent(userID int64, text string) MessageIntent {
	lower := strings.ToLower(text)

	// Avval AI bilan intent aniqlash (chat history bilan)
	if sr.aiClient != nil && sr.chatRepo != nil {
		if intent := sr.detectIntentWithAI(context.Background(), userID, text); intent != IntentNormalChat {
			return intent
		}
	}

	// AI ishlamasa yoki aniqlay olmasa, keyword-based fallback
	// 1. OLDINGI XABARGA MUROJAAT
	if sr.isHistoryReference(lower) {
		return IntentHistoryReference
	}

	// 2. MAHSULOT QIDIRUV
	if sr.isProductSearch(lower) {
		return IntentProductSearch
	}

	// 3. PC YIG'ISH SO'ROVI
	if sr.isPCBuildRequest(lower) {
		return IntentPCBuildRequest
	}

	// 4. SAVOL (oldingi konfiguratsiya haqida)
	if sr.isQuestion(lower) {
		return IntentQuestion
	}

	// 5. ODDIY SUHBAT
	return IntentNormalChat
}

// detectIntentWithAI AI yordamida intent aniqlaydi (chat history bilan)
func (sr *SmartRouter) detectIntentWithAI(ctx context.Context, userID int64, text string) MessageIntent {
	model := sr.aiClient.GenerativeModel("gemini-2.0-flash-exp")
	model.SetTemperature(0.1) // Juda past temperature - aniq javob kerak

	// Oxirgi 10ta xabarni olish
	history, err := sr.chatRepo.GetHistory(ctx, userID, 10)
	if err != nil {
		// History yo'q bo'lsa, faqat joriy xabar bilan ishlash
		history = nil
	}

	// Chat history ni formatlash
	var historyText string
	if len(history) > 0 {
		historyText = "\n\nOxirgi suhbat:\n"
		for _, msg := range history {
			if msg.Text != "" {
				historyText += "User: " + msg.Text + "\n"
			}
			if msg.Response != "" {
				historyText += "Assistant: " + msg.Response + "\n"
			}
		}
	}

	prompt := `Foydalanuvchi xabarini tahlil qil va niyatini aniql. Chat history ni inobatga olib, foydalanuvchi nimani so'rayotganini aniqlang.

FAQAT ushbu javoblardan birini ber (boshqa hech narsa yozma):
- "PC_BUILD" - agar foydalanuvchi TO'LIQ kompyuter yoki PC yig'ib/tuzib berishni so'rasa
- "PRODUCT_SEARCH" - agar foydalanuvchi bitta komponent (gpu, ram, cpu, monitor, klaviatura, mishka) qidirsa
- "OTHER" - qolgan hamma holatlar

MUHIM:
- Agar chat history'da foydalanuvchi allaqachon bitta mahsulot (monitor, klaviatura) so'ragan bo'lsa va keyin "gaming uchun" yoki "250$" kabi qo'shimcha ma'lumot bersa, bu PRODUCT_SEARCH hisoblanadi (PC_BUILD EMAS!)
- Faqat "kompyuter kerak", "PC tuzib ber", "PC yig'ib ber" kabi aniq so'rovlarda PC_BUILD javob ber

Misollar:
"manga kompyuter kerak" → PC_BUILD
"gaming uchun pc tuzib ber" → PC_BUILD
"1500$ budjetda PC kerak" → PC_BUILD
"ram kerak" → PRODUCT_SEARCH
"rtx 4060 bormi" → PRODUCT_SEARCH
"monitor kerak" → PRODUCT_SEARCH
(avval "monitor kerak" degan, keyin) "250$ gaming uchun" → PRODUCT_SEARCH (NOT PC_BUILD!)
"salom" → OTHER` + historyText + `

Hozirgi user xabari: "` + text + `"

Javob (faqat PC_BUILD, PRODUCT_SEARCH yoki OTHER):`

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil || resp == nil || len(resp.Candidates) == 0 {
		return IntentNormalChat // Xatolikda fallback
	}

	textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return IntentNormalChat
	}
	answer := strings.TrimSpace(strings.ToUpper(string(textPart)))

	if strings.Contains(answer, "PC_BUILD") {
		return IntentPCBuildRequest
	}
	if strings.Contains(answer, "PRODUCT_SEARCH") {
		return IntentProductSearch
	}

	return IntentNormalChat
}

// isHistoryReference oldingi xabarga murojaat bormi?
func (sr *SmartRouter) isHistoryReference(text string) bool {
	historyKeywords := []string{
		"yuqoridagi", "yuqorida", "tepada", "tepagi",
		"shu", "o'sha", "usha", "bu",
		"yig'ilgan", "yigʻilgan", "yig'gan",
		"aytgan", "ko'rsatgan", "bergan",
		"этот", "этим", "данн", "выше", "верх",
		"the above", "previous", "earlier",
	}

	for _, kw := range historyKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	return false
}

// isPCBuildRequest PC yig'ish so'rovi bormi?
func (sr *SmartRouter) isPCBuildRequest(text string) bool {
	buildKeywords := []string{
		// O'zbek
		"pc yig'", "pc yigʻ", "pc tuz", "pc qur",
		"kompyuter yig'", "kompyuter tuz", "kompyuter qur",
		"sborka", "sbor", "yig'ib ber", "tuzib ber",
		"to'liq sistema", "tuliq sistema", "to'liq pc",
		"konfiguratsiya kerak", "config kerak",
		"pc kerak", "kompyuter kerak",
		"gaming pc", "gaming kompyuter",
		"pc tayyorla", "pc tayyor",

		// Rus
		"собр", "сбор", "соста", "конфиг",
		"полн систем", "полн пк", "полн компьютер",
		"нужен компьютер", "нужен пк", "нужна сборка",
		"хочу компьютер", "хочу пк",

		// Ingliz
		"build pc", "build me", "build a",
		"make pc", "create pc", "assemble",
		"full pc", "complete pc", "gaming rig",
		"i need pc", "i want pc",
	}

	for _, kw := range buildKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	return false
}

// isQuestion savol bormi?
func (sr *SmartRouter) isQuestion(text string) bool {
	// Savol belgisi
	if strings.Contains(text, "?") {
		return true
	}

	questionKeywords := []string{
		// O'zbek
		"yaxshimi", "yoqdimi", "mos keladimi",
		"qanday", "qandey", "nimasi", "farqi",
		"yetadimi", "yetarlimi", "yetarmi",
		"afsuslanmayman", "afsuslanaman",
		"ishlaydimi", "ishlaydi mi",
		"olsam bo'ladimi", "olsam boladimi",
		"arzonmi", "qimmatmi",

		// Rus
		"хорош", "норм", "подойд",
		"как", "какой", "какая", "разниц",
		"достаточ", "хватит",
		"пожале", "сожале",
		"работа", "потян",
		"можно", "стоит",

		// Ingliz
		"good", "bad", "better", "worse",
		"how", "what", "which", "difference",
		"enough", "sufficient",
		"regret", "worth",
		"can it", "will it", "does it",
	}

	for _, kw := range questionKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	return false
}

// isProductSearch mahsulot qidiruv so'rovi bormi?
func (sr *SmartRouter) isProductSearch(text string) bool {
	productKeywords := []string{
		// Komponentlar
		"gpu", "cpu", "ram", "ssd", "hdd",
		"videokarta", "protsessor", "processor",
		"motherboard", "mat plata", "anakart",
		"blok pitaniya", "psu", "quvvat",
		"korpus", "case", "korp",
		"monitor", "klaviatura", "keyboard",
		"mouse", "sichqoncha", "mishka",

		// So'rovlar
		"bormi", "bor mi", "mavjudmi", "mavjud mi",
		"kerak", "need", "нужен", "нужна",
		"qidiraman", "qidiryapman", "izlayapman",
		"ko'rsating", "korsating", "покажи", "show",
		"tavsiya", "maslahat", "sovet", "recommend",
	}

	for _, kw := range productKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	return false
}

// GetRouteDescription intent uchun tavsif
func (sr *SmartRouter) GetRouteDescription(intent MessageIntent) string {
	switch intent {
	case IntentProductSearch:
		return "Mahsulot qidiruv (savol-javob rejimi)"
	case IntentPCBuildRequest:
		return "PC yig'ish so'rovi (to'liq konfiguratsiya)"
	case IntentQuestion:
		return "Savol (oldingi konfiguratsiya haqida)"
	case IntentHistoryReference:
		return "Oldingi xabarga murojaat"
	case IntentNormalChat:
		return "Oddiy suhbat"
	default:
		return "Noma'lum"
	}
}
