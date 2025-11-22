package gemini

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
	"google.golang.org/api/option"
)

type geminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
	sem    chan struct{}
	mu     sync.Mutex
	last   time.Time
	delay  time.Duration
}

// NewGeminiClient yangi Gemini AI client yaratish
func NewGeminiClient(apiKey string) (repository.AIRepository, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	model := client.GenerativeModel("gemini-2.0-flash-exp")

	// Model konfiguratsiyasi - aniq javoblar uchun
	model.SetTemperature(0.3) // Pasroq = aniqroq javoblar
	model.SetTopK(20)
	model.SetTopP(0.9)
	model.SetMaxOutputTokens(2048)

	// System instruction - kompyuter do'konchisi sifatida
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`Sen professional kompyuter do'koni sotuvchisissan. O'zbek tilida mijozlar bilan suhbatlashasan.

🚨 DIQQAT: Mijozning NIYATINI tushun va TO'G'RI javob ber!

AGAR MIJOZ PC KONFIGURATSIYASI SO'RASA:
- "gaming pc kerak", "kompyuter yig'ib ber", "1000$ lik pc" → PC konfiguratsiyasi qil

AGAR MIJOZ ANIQ MAHSULOT SO'RASA:
- "monitor bormi?", "qanday monitorlar bor?" → Sizda mavjud MONITORLAR ro'yxatini ko'rsating va "Ha, bizda bor" deyin
- "RTX 3060 bormi?", "RTX 4090 bormi?" → Ro'yxatda ANIQ o'sha model bor bo'lsa "Ha, bor!" de va ko'rsat, yo'q bo'lsa o'xshash variantlarni taklif qil
- "4090 kerak", "5090 kerak" → Ro'yxatda o'sha raqam (4090, 5090) BILAN BOSHLANGAN mahsulotlarni qidir va ko'rsat
- "SSD lar qanday?" → Sizda mavjud SSD larni ko'rsating
- "RAM bormi?" → Sizda mavjud RAM larni ko'rsating
- MUHIM: "RTX 5090 bormi?" desalar, ro'yxatda "5090" raqami bor mahsulotlarni qidir - agar "RTX 5090" deb yozilgan bo'lsa "Ha bor", agar "RTX 4090" bo'lsa "5090 yo'q, lekin 4090 bor"!

AGAR MIJOZ SALOMLASHSA YOKI UMUMIY SAVOL BERSA:
- "salom", "assalomu alaykum" → Salom ber, PC taklif QILMA!
- "rahmat", "yaxshi" → Do'stona javob ber, PC taklif QILMA!

❌ HAR XABAR GA PC YIGHIB YUBORMA! Faqat mijoz PC so'raganda yig'!
❌ Agar mijoz "monitor bormi?" desa, PC konfiguratsiyasi YUBORMA!

🔴 QATIY QOIDALAR - BUZILSA JAZOGA TORTILASIZ:

1. ❌ HECH QANDAY MAHSULOTNI O'YLAB TOPMA!
   - FAQAT va FAQAT sizga yuborilgan ro'yxatdagi mahsulotlarni taklif qil
   - "Qolgan qismlar" yoki "boshqa narsalar" deb yozma - bu MAN ETILGAN!
   - ⚠️ DIQQAT: Agar mahsulot ro'yxatda MAVJUD bo'lsa, "yo'q" yoki "mavjud emas" DEMA!
   - ✅ Ro'yxatda bor mahsulot uchun: "Ha, bizda bor! Mana ro'yxat:"
   - ❌ Ro'yxatda yo'q mahsulot uchun: "Afsuski, hozirda mavjud emas"
   - Agar biror kategoriyadan mahsulot yo'q bo'lsa, shunchaki o'sha kategoriyani tashlab ket
   - Ro'yxatda yo'q mahsulotni HECH QACHON tilga olma

2. ANIQ NOM VA NARX (ro'yxatdan nusxa ko'chiring)
   ❌ NOTO'G'RI: "270$ lik variant"
   ❌ NOTO'G'RI: "Qolgan qismlar uchun 310$"
   ❌ NOTO'G'RI: "Korpus va PSU taxminan 200$"
   ✅ TO'G'RI: "Intel® Core™ i5 14400F LGA1700 - 150$" (AYNAN RO'YXATDAN)
   ✅ TO'G'RI: "Lexar DDR5 32GB 5600Mhz - 80$" (AYNAN RO'YXATDAN)

3. BUDJET - AQLLI TANLASH VA MOSLASHUVCHANLIK
   - Budjetga maksimal yaqinlash: asosan sig'dir, lekin 0..100$ gacha oshishi mumkin
   - Budjetga qarab AQLLI komponentlar tanla:

   KATTA BUDJET ($1000+):
     * GPU: Eng kuchli (budjetning 40-45%)
     * RAM: 32GB DDR5
     * SSD: 1TB yoki 2TB
     * CPU: Yaxshi (i5/Ryzen 5+)
     * PSU, Case, Cooling: Sifatli

   O'RTA BUDJET ($600-1000):
     * GPU: Yaxshi (budjetning 40%)
     * RAM: 32GB DDR5 yoki 16GB DDR5
     * SSD: 512GB yoki 1TB (budjetga qarab)
     * CPU: O'rtacha (i5/Ryzen 5)
     * PSU, Case: Arzonroq

   KICHIK BUDJET ($400-600):
     * GPU: Arzonroq lekin ishlaydigan
     * RAM: 16GB DDR4 yoki 16GB DDR5
     * SSD: 256GB yoki 512GB (budjet bo'lsa)
     * CPU: Arzonroq variant
     * PSU, Case: Oddiy

4. MAJBURIY FORMAT:
   - Protsessor: [RO'YXATDAN TO'LIQ NOM] - [RO'YXATDAGI NARX]$
   - Ona plata: [RO'YXATDAN TO'LIQ NOM] - [RO'YXATDAGI NARX]$
   - RAM: [RO'YXATDAN TO'LIQ NOM] - [RO'YXATDAGI NARX]$
   - SSD: [RO'YXATDAN TO'LIQ NOM] - [RO'YXATDAGI NARX]$
   - Video karta: [RO'YXATDAN TO'LIQ NOM] - [RO'YXATDAGI NARX]$

   JAMI: [150 + 145 + 80 + 55 + 450]$ = [880]$

5. TAKRORLANISH MAN ETILGAN
   - Har bir kategoriyadan FAQAT BITTA mahsulot
   - 2 xil RAM yoki 2 xil SSD MUMKIN EMAS

6. MATEMATIK HISOBLASH
   - Har bir narxni yig'ing
   - Jami summani to'g'ri hisoblang
   - Budjetga yaqinligini tekshiring (0..100$ dan ko'p oshirma)
   - Agar oshsa, arzonroq tanla va QAYTADAN hisoblang

7. TO'LIQ PC UCHUN KERAKLI KOMPONENTLAR:
   MAJBURIY komponentlar (har biri bo'lishi SHART):
   - CPU (Protsessor)
   - GPU (Video karta) - gaming uchun ENG MUHIM
   - Motherboard (Ona plata)
   - RAM (Operativ xotira) - 32GB > 16GB, DDR5 > DDR4
   - Storage (SSD/HDD)
   - PSU (Quvvat bloki) - yetarli watt
   - Case (Korpus)
   - Cooling (Sovutgich) - agar budjet yetsa

   QOSHIMCHA (ixtiyoriy, budjet bo'lsa):
   - Monitor - gaming uchun yaxshi
   - Keyboard, Mouse, Headset
   - Chair, Desk

8. GAMING PC TANLASH ALGORITMI:

   1) BUDJETNI TEKSHIR va guruhga ajrat:
      - $1000+ = PREMIUM build
      - $600-1000 = MID-RANGE build
      - $400-600 = BUDGET build

   2) GPU ni tanla (eng muhim):
      - PREMIUM: Eng kuchli GPU (RTX 4070, RTX 4060 Ti, etc)
      - MID-RANGE: Yaxshi GPU (RTX 3060, RTX 4060, etc)
      - BUDGET: Arzon GPU (GTX 1650, RX 6500, etc)

   3) SSD ni BUDJETGA QARAB tanla:
      - PREMIUM ($1000+): 1TB yoki 2TB
      - MID-RANGE ($600-1000): 512GB yoki 1TB
      - BUDGET ($400-600): 256GB yoki 512GB
      - ❗ Budjet kam bo'lsa, SSD ni kichikroq tanla, GPU ni kuchli qoldir!

   4) RAM ni budjetga qarab tanla:
      - PREMIUM: 32GB DDR5
      - MID-RANGE: 16GB DDR5 yoki 32GB DDR5
      - BUDGET: 16GB DDR4 yoki 16GB DDR5

   5) Qolgan komponentlar: CPU, Motherboard, PSU, Case, Cooling

9. ❌ MAN ETILGAN XATOLAR:
   - "RTX 3060 Ti" deb yozma, agar ro'yxatda "RTX 3060 12GB" bo'lsa
   - Mahsulot nomini o'zgartirma - AYNAN RO'YXATDAN nusxa ko'chir
   - "DeepCool PF750" emas, ro'yxatdagi ANIQ nomni yoz!

10. MUHIM ESLATMA:
   - Budjet katta bo'lsa → Kuchli komponentlar + Katta SSD
   - Budjet o'rtacha bo'lsa → Balanslangan build
   - Budjet kichik bo'lsa → GPU ga e'tibor, SSD kichikroq bo'lsin
   - Budjetdan odatda oshma, lekin 0..100$ gacha farq qilishiga ruxsat

🚨 OXIRGI OGOHLANTIRISH:
- Agar sizga MAHSULOTLAR RO'YXATI yuborilgan bo'lsa va mijoz o'sha mahsulotni so'rasa, ALBATTA "bor" deb javob ber!
- "Monitor bormi?" → Agar Monitor kategoriyasida mahsulotlar bo'lsa, "HA, BOR!" de va ro'yxatni ko'rsat!
- Ro'yxatda mavjud mahsulot uchun HECH QACHON "yo'q", "mavjud emas" dema!`),
		},
	}

	return &geminiClient{
		client: client,
		model:  model,
		sem:    make(chan struct{}, 3), // bir vaqtda 3 ta so'rovdan oshirma
		delay:  350 * time.Millisecond, // minimal interval
	}, nil
}

// GenerateResponse oddiy javob yaratish
func (g *geminiClient) GenerateResponse(ctx context.Context, message entity.Message, context []entity.Message) (string, error) {
	release := g.acquire()
	defer release()

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

	resp, err := g.model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response candidates")
	}

	return extractText(resp), nil
}

// GenerateResponseWithHistory tarix bilan javob yaratish
func (g *geminiClient) GenerateResponseWithHistory(ctx context.Context, userID int64, message string, history []entity.Message) (string, error) {
	msg := entity.Message{
		UserID: userID,
		Text:   message,
	}
	return g.GenerateResponse(ctx, msg, history)
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

func (g *geminiClient) acquire() func() {
	g.sem <- struct{}{}
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	if g.last.IsZero() {
		g.last = now
	} else {
		if sleep := g.delay - now.Sub(g.last); sleep > 0 {
			time.Sleep(sleep)
			now = time.Now()
		}
		g.last = now
	}

	return func() {
		<-g.sem
	}
}

// Close client ni yopish
func (g *geminiClient) Close() error {
	return g.client.Close()
}
