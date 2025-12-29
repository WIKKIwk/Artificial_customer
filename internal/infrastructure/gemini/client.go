package gemini

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/yourusername/telegram-ai-bot/internal/domain/constants"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
	"google.golang.org/api/option"
)

type geminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewGeminiClient yangi Gemini AI client yaratish
func NewGeminiClient(apiKey string) (repository.AIRepository, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	model := client.GenerativeModel(constants.GeminiModelName)

	// Model konfiguratsiyasi - aniq javoblar uchun
	model.SetTemperature(constants.AITemperature)
	model.SetTopK(constants.AITopK)
	model.SetTopP(constants.AITopP)
	// MaxOutputTokens ni o'chirish - limit yo'q

	// System instruction - do'stona, professional kompyuter mutaxassisi
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`Sen kompyuter do'konining JONLI xodimisan. Mijoz bilan SUHBATLASH, robot emas!

⛔ HECH QACHON BUNDAY YOZMA:
"Narxi: 1700$"
"Jami: 1700$"
"Ha, bizda bor: --"

✅ FAQAT SHUNDAY YOZ:

MISOL 1 - Mijoz: "manga ram kerak"
Sen javob: "Ajoyib! RAM uchun kerak. Qaysi maqsadda ishlatmoqchisiz - gaming yoki oddiy ish uchun? Va budjetingiz qancha? (masalan: 100$, 200$, 500$)"

	MISOL 2 - Mijoz: "gaming uchun, 200$"
	Sen javob: "200$ budjetga zo'r gaming RAM variantlar:

	1. CORSAIR VENGEANCE DDR4 16GB - 180$
	2. KINGSTON FURY DDR4 16GB - 150$
	3. CRUCIAL DDR4 16GB - 120$

	Qaysi birini tanlaysiz? Raqamini yozing (1, 2 yoki 3)."

MISOL 3 - Mijoz: "1"
Sen javob: "Ajoyib tanlov! CORSAIR VENGEANCE DDR4 16GB - 180$

Jami: 180$

Sotib oling tugmasini bosing yoki boshqa narsa kerakmi?"

⚠️ AGAR MIJOZ OLDINGI KONFIGURATSIYA HAQIDA SAVOL BERSA:
- Mijoz: "bu pc yaxshimi?" yoki "yuqoridagi yoqdimi?"
- HECH QACHON yangi variant ko'rsatma!
- FAQAT baholash va fikr ber
- Misol javob: "Ha, ajoyib tanlov! Bu konfiguratsiya gaming uchun juda yaxshi. RTX 3060 zamonaviy o'yinlarni yuqori sozlamada o'ynaydi. Ryzen 5 5600 ham kuchli. Narx-sifat jihatidan zo'r variant!"

MUHIM:
- Har doim SUHBAT qil
- Mahsulot qidirganda: Variantlarni raqam bilan yoz "1. Model - narx"
- Mijozga raqam yozishni ayting: "Raqamini yozing (1, 2 yoki 3)"
- "Jami:" faqat mijoz raqam yozib tanlaganda ko'rsat
	- Mahsulot qidirganda doim 3-5 variant taklif qil (imkon bo'lsa 5 ta)
	- Agar budjet bo'lsa, budjetga eng yaqin variantlarni yuqoriga qo'y
- Konfiguratsiya haqida savol bo'lsa - faqat baholash ber, yangi variant yo'q!
- Hech qachon "Ha, bizda bor: --" dema
- Emoji ishlatma (💚), faqat raqamlar: 1. 2. 3.

	🛒 SAVATCHA (BOT TOMONIDA)
	Botda alohida savatcha mavjud, shuning uchun:
	- Har safar faqat HOZIRGI so'ralgan/yangi tanlangan mahsulot haqida yoz.
	- Chat history'dagi oldingi tanlovlarni "Savatchadagi mahsulotlar:" deb qayta sanab chiqma.
	- Oldingi konfiguratsiya yoki boshqa mahsulotlarning narxini yangi mahsulotga qo'shib "Jami:" hisoblama.
	- Agar mijoz bir nechta mahsulot olmoqchi bo'lsa, "🛒 Savatga qo'shish" tugmasini bosib savatga qo'shishini eslat.
	
	MISOL:
	Mijoz avval chair tanlagan, keyin kovrik so'rasa:
		"SteelSeries QcK Heavy uchun 3-5 ta variant:
		1. SteelSeries QcK Heavy L - 40$
		2. SteelSeries QcK Edge XL - 55$
		3. Logitech G840 XL - 40$
		Qaysi birini tanlaysiz? Raqamini yozing (1, 2 yoki 3)."`),
		},
	}

	return &geminiClient{
		client: client,
		model:  model,
	}, nil
}

// GenerateResponse oddiy javob yaratish
func (g *geminiClient) GenerateResponse(ctx context.Context, message entity.Message, context []entity.Message) (string, error) {
	// Chat history ni tayyorlash
	var parts []genai.Part

	// Oldingi xabarlarni qo'shish (kontekst)
	for _, msg := range context {
		if msg.Text != "" {
			parts = append(parts, genai.Text(fmt.Sprintf("Foydalanuvchi: %s", msg.Text)))
		}
		if msg.Response != "" {
			parts = append(parts, genai.Text(fmt.Sprintf("Siz: %s", msg.Response)))
		}
	}

	// Hozirgi xabarni qo'shish
	parts = append(parts, genai.Text(message.Text))

	// Retry logic
	maxRetries := constants.MaxRetries
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("🔄 Gemini API ga so'rov yuborish (urinish %d/%d)...", attempt, maxRetries)

		resp, err := g.model.GenerateContent(ctx, parts...)
		if err != nil {
			lastErr = err
			log.Printf("❌ Urinish %d xato: %v", attempt, err)
			if attempt < maxRetries {
				waitDuration := constants.RetryDelay * time.Second
				log.Printf("⏳ %v kutib qayta urinish...", waitDuration)
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(waitDuration):
					continue
				}
			}
			continue
		}

		if len(resp.Candidates) == 0 {
			lastErr = fmt.Errorf("no response candidates")
			log.Printf("⚠️ Urinish %d: Javob kandidatlari yo'q", attempt)
			if attempt < maxRetries {
				waitDuration := constants.RetryDelay * time.Second
				log.Printf("⏳ %v kutib qayta urinish...", waitDuration)
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(waitDuration):
					continue
				}
			}
			continue
		}

		// Safety ratings tekshirish
		if len(resp.Candidates) > 0 && resp.Candidates[0].FinishReason != 0 {
			log.Printf("⚠️ Gemini FinishReason: %v", resp.Candidates[0].FinishReason)
			if resp.Candidates[0].FinishReason == 3 { // SAFETY
				log.Printf("🚫 Response blocked by safety filter!")
				return "Kechirasiz, javob berish imkoni bo'lmadi. Iltimos, boshqa so'rov bilan qaytadan urinib ko'ring.", nil
			}
		}

		responseText := extractText(resp)

		// Bo'sh javob tekshirish
		if strings.TrimSpace(responseText) == "" {
			log.Printf("⚠️ Urinish %d: Bo'sh javob qaytdi", attempt)
			if attempt < maxRetries {
				waitDuration := constants.RetryDelay * time.Second
				log.Printf("⏳ %v kutib qayta urinish...", waitDuration)
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(waitDuration):
					continue
				}
			}
			lastErr = fmt.Errorf("empty response after %d attempts", maxRetries)
			continue
		}

		// Muvaffaqiyatli javob!
		log.Printf("✅ Javob muvaffaqiyatli olindi (urinish %d)", attempt)
		return responseText, nil
	}

	// Barcha urinishlar muvaffaqiyatsiz
	log.Printf("❌ Barcha %d urinish muvaffaqiyatsiz tugadi", maxRetries)
	if lastErr != nil {
		return "", fmt.Errorf("AI javob berishda xatolik yuz berdi (%d urinishdan keyin): %w", maxRetries, lastErr)
	}
	return "", fmt.Errorf("AI dan javob olinmadi (%d urinishdan keyin)", maxRetries)
}

// GenerateResponseWithHistory tarix bilan javob yaratish
func (g *geminiClient) GenerateResponseWithHistory(ctx context.Context, userID int64, message string, history []entity.Message) (string, error) {
	msg := entity.Message{
		UserID: userID,
		Text:   message,
	}
	return g.GenerateResponse(ctx, msg, history)
}

// GenerateConfigResponse MAXSUS konfiguratsiya uchun javob yaratish
// Bu funksiya faqat /configuratsiya komandasi uchun ishlatiladi va PC yig'ishga ruxsat beradi
func (g *geminiClient) GenerateConfigResponse(ctx context.Context, userID int64, message string, history []entity.Message) (string, error) {
	// Maxsus konfiguratsiya uchun model yaratish
	configModel := g.client.GenerativeModel("gemini-2.5-flash")
	configModel.SetTemperature(0.3)
	configModel.SetTopK(20)
	configModel.SetTopP(0.9)

	// MAXSUS System Instruction - faqat konfiguratsiya uchun (PC yig'ishga ruxsat!)
	configModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`Sen kompyuter do'koni sotuvchisissan. O'zbek tilida oddiy va qisqa javob berasan.

🎯 MAXSUS REJIM: PC KONFIGURATSIYA YIG'ISH VA MASLAHAT

Sen /configuratsiya komandasi orqali chaqirilgansan. Ikki xil vazifa bajarasan:

📌 VAZIFA 1: YANGI KONFIGURATSIYA YIG'ISH
Agar mijoz yangi PC so'rasa (budjet, maqsad aytsa):
✅ TO'LIQ PC konfiguratsiyasi tuzish
✅ Barcha komponentlarni tanlash (CPU, RAM, GPU, SSD, PSU, va h.k.)
✅ Narxlarni CSV dan ANIQ olish
✅ Oxirida "Jami:" qatorida umumiy narxni ko'rsatish

📌 VAZIFA 2: SAVOL VA MASLAHAT (ENG MUHIM!)
Agar mijoz "yaxshimi?", "qanday?", "yuqoridagi...", "yig'ilgan..." deb so'rasa:
✅ CHAT HISTORY'NI KO'R! O'zingiz yozgan yoki boshqa AI yozgan konfiguratsiyani TOP!
✅ O'sha konfiguratsiyani tahlil qil va baholash ber
✅ Kamchilik/kuchli tomonlarini ayt
✅ Agar kerak bo'lsa, o'zgartirish tavsiyasi ber
✅ YANGI konfiguratsiya yig'ma, faqat fikr ayt!
❌ "PC yig'ish uchun /configuratsiya komandasini ishlating" DEMA!

🔍 CHAT HISTORY'DAN KONFIGURATSIYA TOPISH:
Agar mijoz "yuqoridagi", "yuqorida yig'ilgan", "bu pc" desa:
1. Chat history'ni tekshir
2. Oldingi xabarlarda "CPU:", "RAM:", "GPU:", "Jami:" ko'r
3. O'sha konfiguratsiyani tahlil qil
4. YANGI konfiguratsiya yig'ma!

Masalan:
━━━━━━━━━━━━━━━━━━━━━━━━━━
Chat history:
Siz: "🖥️ PC KONFIGURATSIYA
• CPU: INTEL CORE I5 13400F T - 170.00$
• RAM: KINGSTON DDR4 16GB - 47.00$
• GPU: PNY GEFORCE RTX 4060TI - 400.00$
Jami: 922.00$"

Mijoz: "yuqorida yig'ilgan pc yaxshimi?"

✅ TO'G'RI JAVOB:
"Ha, bu konfiguratsiya yaxshi! Intel Core i5 13400F va RTX 4060Ti birgalikda gaming uchun zo'r ishlaydi. Narxi ham 922$ - budjetga mos. Faqat RAM 16GB - agar professional ish qilsangiz, 32GB yaxshiroq bo'lardi. Lekin gaming uchun 16GB yetarli."

❌ XATO JAVOBLAR:
"PC yig'ish uchun /configuratsiya komandasini ishlating" (TAQIQLANGAN!)
[Yangi konfiguratsiya yig'ish] (TAQIQLANGAN!)
━━━━━━━━━━━━━━━━━━━━━━━━━━

Mijoz: "1k$ ga gaming pc kerak"
✅ TO'G'RI: [Yangi konfiguratsiya yig'ish - bu VAZIFA 1]

📋 KONFIGURATSIYA TUZISH QOIDALARI:

1️⃣ TO'LIQ KOMPONENTLAR RO'YXATI:
• CPU (Processor)
• RAM (Operativ xotira)
• GPU (Videokarta)
• SSD/HDD (Qattiq disk)
• Motherboard (Anakart)
• PSU (Quvvat bloki)
• Case (Korpus) - agar kerak bo'lsa

2️⃣ NARXLAR:
• CSV dan ANIQ narxlarni ol
• HECH QACHON narxni o'zgartirma (899$ → 899$, 1000$ EMAS!)
• Har bir komponent uchun narx ko'rsat

3️⃣ CPU-MOTHERBOARD-RAM INTELLIGENT MATCHING (KRITIK QOIDA!):
⚠️ HECH QACHON bundaylarga RUXSAT BERMA:
   ❌ Non-K CPU (13400F, 12400, 7500F) + Z790 MOTHERBOARD = JUDA QIMMAT VA NOTOG'RI!
   ❌ K-series CPU (13700K, 13900K) + B760 MOTHERBOARD = OVERCLOCKING IMKONI YO'Q!
   ❌ DDR4 RAM + Z790 MOTHERBOARD = INCOMPATIBLE!

✅ TO'G'RI KOMBINATSIYALAR:

A) BUDJET < 1000$ (Entry-level gaming):
   • CPU: i5-13400F (Non-K) - 170$
   • RAM: DDR4 32GB (KINGSTON FURY DDR4) - 85$
   • MOTHERBOARD: B760M-K WIFI D4 (DDR4 chipset) - 150$
   • Total CPU+RAM+MB: ~405$

B) BUDJET 1200-1500$ (Mid-range gaming):
   • CPU: i5-13400F (Non-K) - 170$ YOKI i5-12400F - 200$
   • RAM: DDR5 32GB (KINGSTON FURY DDR5 6000MHz) - 200$
   • MOTHERBOARD: ASUS ROG Z790-P DDR5 (DDR5 chipset) - 250$ YOKI GIGABYTE Z790 EAGLE AX - 250$
   • Total CPU+RAM+MB: ~620$
   ⚠️ QAT'IY QAIDA: Non-K CPU bilan Z790 FAQAT agar DDR5 RAM bo'lsa va budjet yetarli bo'lsa!

C) BUDJET 1500-2000$ (High-end gaming):
   • CPU: i7-13700K (K-series!) - 340$ YOKI AMD RYZEN 7 7700X - 300$
   • RAM: DDR5 32GB (KINGSTON FURY DDR5 6000MHz) - 200$
   • MOTHERBOARD: Z790 EAGLE AX DDR5 - 250$ YOKI ASUS ROG STRIX Z790 - 340$
   • Total CPU+RAM+MB: ~790$

D) BUDJET 2000$+ (Extreme gaming/creative):
   • CPU: i9-13900K (K-series!) - 420$ YOKI AMD RYZEN 9 7900X3D - 530$
   • RAM: DDR5 64GB (2x32GB KINGSTON FURY) - 400$
   • MOTHERBOARD: Z790 AORUS ELITE DDR5 - 300$ YOKI ASUS ROG STRIX Z790 - 340$
   • Total CPU+RAM+MB: ~1120$

AGAR QAYSHISIGA SHUBHA BO'LSA:
"Non-K CPU + non-overclocking motherboard + DDR4/DDR5" = TO'G'RI
"K-series CPU + Z790 motherboard + DDR5" = TO'G'RI

4️⃣ FORMAT (ANIQ VA TO'LIQ):

Assalomu alaykum! "[Config Name]" nomli, [budget] budjetga mos, [keydescription] konfiguratsiyasini tayyorladim. Monitor ham talabingizga binoan kiritildi.

🖥️ **PC KONFIGURATSIYA: [Config Name]**

• CPU: [To'liq nom] - [Narx]$
• RAM: [To'liq nom] - [Narx]$
• GPU: [To'liq nom] - [Narx]$
• SSD: [To'liq nom] - [Narx]$
• Motherboard: [To'liq nom] - [Narx]$
• Cooler: [To'liq nom] - [Narx]$
• PSU: [To'liq nom] - [Narx]$
• Case: [To'liq nom] - [Narx]$

-Case components
 Price: [summa]$

-Monitor (agar kerak bo'lsa)
• Monitor: [To'liq nom] - [Narx]$
 Price: [summa]$

-Peripherals
Price: [summa yoki 0$]

Overall price: [Jami Summa]$

❌ Hech qanday "TAVSIYALAR", "UPGRADE" yoki shunga o'xshash bo'lim yozma (mijoz so'ramasa).

MISOLDA (Professional Mid-High Gaming PC):
Assalomu alaykum! "Gaming Pro" nomli, 20 million so'm (taxminan 1800$) budjetga mos, Intel K-series CPU va RTX videokartali, zamonaviy DDR5 RAM bilan suyuq sovutishli gaming kompyuter konfiguratsiyasini tayyorladim. Monitor ham talabingizga binoan kiritildi.

🖥️ **PC KONFIGURATSIYA: Gaming Pro**

• CPU: INTEL CORE I7-13700K - 340.00$
• RAM: KINGSTON FURY DDR5 32GB(2X16GB)6000MHZ (WHITE) - 200.00$
• GPU: PNY GEFORCE RTX 4070 12GB - 550.00$
• SSD: SAMSUNG 970 EVO PLUS 1TB NVMe - 80.00$
• Motherboard: GIGABYTE Z790 EAGLE AX DDR5 WIFI LGA 1700 - 250.00$
• Cooler: DEEPCOOL LE300 MARRS 120MM AIO - 50.00$
• PSU: DEEPCOOL PK650D 650W 80 PLUS BRONZE - 60.00$
• Case: DEEPCOOL MACUBE 110 WH - 60.00$

-Case components
 Price: 1590.00$

-Monitor
• Monitor: DELL ALIENWARE AW2724HF 27" 240Hz IPS - 450.00$
 Price: 450.00$

-Peripherals
Price: 0$

Overall price: 2040.00$

4️⃣ BUDJET:
• Agar mijoz "1000$" desa, konfiguratsiya 1000$ dan OSHMASIN!
• Narxlarni to'g'ri hisoblang

5️⃣ MUHIM:
• Yangi konfiguratsiya uchun: "Jami:" qatorini ALBATTA yoz
• Savol uchun: Oldingi konfiguratsiyani baholash, yangi yig'ma!
• Barcha narxlarni CSV dan ol
• Komponentlar bir-biriga mos kelishini tekshir

QISQASI:
- Yangi PC so'rasa → To'liq konfiguratsiya yig'
- Savol bersa → Baholash ber, yangi yig'ma!`),
		},
	}

	// Chat history ni tayyorlash
	var parts []genai.Part
	for _, msg := range history {
		if msg.Text != "" {
			parts = append(parts, genai.Text(fmt.Sprintf("Foydalanuvchi: %s", msg.Text)))
		}
		if msg.Response != "" {
			parts = append(parts, genai.Text(fmt.Sprintf("Siz: %s", msg.Response)))
		}
	}
	parts = append(parts, genai.Text(message))

	// AI dan javob olish (retry logikasi bilan)
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("🔄 Gemini API (CONFIG MODE) ga so'rov yuborish (urinish %d/%d)...", attempt, maxRetries)

		resp, err := configModel.GenerateContent(ctx, parts...)
		if err != nil {
			lastErr = err
			log.Printf("❌ Urinish %d xato: %v", attempt, err)
			if attempt < maxRetries {
				waitDuration := constants.RetryDelay * time.Second
				log.Printf("⏳ %v kutib qayta urinish...", waitDuration)
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(waitDuration):
					continue
				}
			}
			continue
		}

		if len(resp.Candidates) == 0 {
			lastErr = fmt.Errorf("no response candidates")
			log.Printf("⚠️ Urinish %d: Javob kandidatlari yo'q", attempt)
			if attempt < maxRetries {
				waitDuration := constants.RetryDelay * time.Second
				log.Printf("⏳ %v kutib qayta urinish...", waitDuration)
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(waitDuration):
					continue
				}
			}
			continue
		}

		if len(resp.Candidates) > 0 && resp.Candidates[0].FinishReason != 0 {
			log.Printf("⚠️ Gemini FinishReason: %v", resp.Candidates[0].FinishReason)
			if resp.Candidates[0].FinishReason == 3 {
				log.Printf("🚫 Response blocked by safety filter!")
				return "Kechirasiz, javob berish imkoni bo'lmadi. Iltimos, boshqa so'rov bilan qaytadan urinib ko'ring.", nil
			}
		}

		responseText := extractText(resp)

		if strings.TrimSpace(responseText) == "" {
			log.Printf("⚠️ Urinish %d: Bo'sh javob qaytdi", attempt)
			if attempt < maxRetries {
				waitDuration := constants.RetryDelay * time.Second
				log.Printf("⏳ %v kutib qayta urinish...", waitDuration)
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(waitDuration):
					continue
				}
			}
			lastErr = fmt.Errorf("empty response after %d attempts", maxRetries)
			continue
		}

		log.Printf("✅ Konfiguratsiya javobi muvaffaqiyatli olindi (urinish %d)", attempt)
		return responseText, nil
	}

	log.Printf("❌ Barcha %d urinish muvaffaqiyatsiz tugadi", maxRetries)
	if lastErr != nil {
		return "", fmt.Errorf("AI konfiguratsiya javobi xatosi (%d urinishdan keyin): %w", maxRetries, lastErr)
	}
	return "", fmt.Errorf("AI dan konfiguratsiya javobi olinmadi (%d urinishdan keyin)", maxRetries)
}

// extractText javobdan textni ajratib olish
func extractText(resp *genai.GenerateContentResponse) string {
	var result strings.Builder
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				result.WriteString(fmt.Sprintf("%v", part))
			}
		}
	}
	return result.String()
}

// Close client ni yopish
func (g *geminiClient) Close() error {
	return g.client.Close()
}

// GetRawClient raw genai.Client qaytaradi (SmartRouter uchun)
func (g *geminiClient) GetRawClient() *genai.Client {
	return g.client
}
