# 🤖 Telegram AI Kompyuter Do'konchisi Bot

Go dasturlash tilida yozilgan va Google Gemini 2.0 Flash AI dan foydalanadigan professional Telegram bot. Bot kompyuter texnikasi bo'yicha maslahat beruvchi virtual do'konchi vazifasini bajaradi va Excel fayllar orqali mahsulot katalogini boshqaradi.

## 🏗️ Arxitektura

Loyiha **Clean Architecture** prinsiplari asosida qurilgan:

```
telegram-ai-bot/
├── cmd/
│   └── bot/                    # Application entry point
│       └── main.go
├── config/                     # Configuration layer
│   └── config.go
├── internal/
│   ├── domain/                 # Domain layer (entities, interfaces)
│   │   ├── entity/             # Message, Product, Admin entities
│   │   └── repository/         # Repository interfaces
│   ├── usecase/                # Business logic layer
│   │   ├── chat_usecase.go
│   │   ├── admin_usecase.go
│   │   └── product_usecase.go
│   ├── infrastructure/         # External services implementations
│   │   ├── gemini/             # Gemini AI client
│   │   ├── storage/            # In-memory storage
│   │   └── parser/             # Excel file parser
│   └── delivery/               # Delivery layer
│       └── telegram/           # Telegram bot handlers
└── pkg/                        # Shared packages
    └── logger/
```

### 📦 Layers Tushunchasi

1. **Domain Layer** - Biznes logikasining markaziy qismi, external dependencies dan mustaqil
2. **Use Case Layer** - Ilovaning biznes qoidalari va orchestration
3. **Infrastructure Layer** - External services bilan ishlash (AI, Storage, Parser)
4. **Delivery Layer** - User interface (Telegram bot handlers)

## ✨ Xususiyatlar

### 🤖 AI va Chat
- 🧠 **Gemini 2.0 Flash AI** - Google ning eng so'nggi AI modeli
- 💬 **Kontekstli suhbat** - Bot oldingi xabarlarni eslaydi
- 🛍️ **Smart do'konchi** - Mahsulot katalogi asosida savdo qiladi

### 👨‍💼 Admin Panel
- 🔐 **Parol bilan himoyalangan** - Admin panel (parol: `@#12`)
- 📤 **Excel yuklash** - Mahsulot katalogini Excel fayldan yuklash (max 5MB)
- 📊 **Katalog boshqaruvi** - Mahsulotlar va kategoriyalarni ko'rish
- 📝 **Admin log** - Barcha admin harakatlari loglanadi

### 📦 Mahsulot Katalogi
- 🗂️ **Excel import** - .xlsx va .xls formatlarini qo'llab-quvvatlash
- 🔍 **Avtomatik parsing** - Kategoriya, narx, tavsif va boshqalar
- 💰 **Narx ma'lumotlari** - Turli valyuta formatlarini qo'llab-quvvatlash
- 📊 **Ombor ma'lumotlari** - Stock tracking

### 🔧 Texnik
- 🔄 **Graceful shutdown** - To'g'ri to'xtatish mexanizmi
- 🏗️ **Clean Architecture** - Kengaytirish va test qilish oson
- 🔒 **Type-safe** - Go ning kuchli type system
- 💾 **In-memory storage** - Tez va samarali (keyinchalik DB qo'shish mumkin)

## 🚀 O'rnatish va Ishga Tushirish

### Talablar

- Go 1.21 yoki yuqori
- Telegram Bot Token
- Google Gemini API Key

### Tez start (`make`)

```bash
git clone <repository-url>
cd telegram-ai-bot
make
```

`make` quyidagilarni bajaradi:
- `.env` mavjud bo'lmasa `.env.example` dan nusxa olib, tokenlarni kiritish uchun jarayonni to'xtatadi
- Dependencies ni avtomatik yuklaydi (`go mod download` + `go mod tidy`)
- `data/` papkasini yaratadi
- Tokenlar to'g'ri kiritilgan bo'lsa botni darhol ishga tushiradi

Tokenlarni kiritib bo'lgach `make` ni qayta ishga tushiring. Faqat tayyorgarlikni bajarish uchun `make prepare` dan foydalanishingiz mumkin.

### Qo'lda o'rnatish

1. Repository ni clone qiling:
   ```bash
   git clone <repository-url>
   cd telegram-ai-bot
   ```
2. Dependencies ni o'rnating:
   ```bash
   go mod download
   ```
3. Environment faylni sozlang:
   ```bash
   cp .env.example .env
   ```
   `.env` faylini tahrirlang:
   ```env
   TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
   GEMINI_API_KEY=AIzaSy...
   ```
4. Botni ishga tushiring:
   **Development:**
   ```bash
   go run cmd/bot/main.go
   ```
   Yoki Makefile:
   ```bash
   make        # default holatda prepare + run
   ```
   **Production:**
   ```bash
   make build
   ./bot
   ```

#### API Key'larni qanday olish:

**Telegram Bot Token:**
1. Telegram da [@BotFather](https://t.me/botfather) ga yozing
2. `/newbot` komandasi bilan yangi bot yarating
3. Bot uchun nom va username tanlang
4. Token ni nusxalang

**Gemini API Key:**
1. [Google AI Studio](https://aistudio.google.com/app/apikey) ga kiring
2. "Create API Key" tugmasini bosing
3. API key ni nusxalang

## 🚚 Deployment (tar.gz)

1. Toza arxiv yarating (mod cache va maxfiy fayllarsiz):
   ```bash
   make dist
   ```
2. Arxivni serverga yuboring:
   ```bash
   scp upg.tar.gz root@server-ip:/root/
   ```
3. Serverda oching va ishga tushiring:
   ```bash
   ssh root@server-ip
   cd /root
   tar xzf upg.tar.gz
   cd upg
   cp .env.example .env   # tokenlarni kiriting
   make run
   ```

**Eslatma:** Agar `go/pkg/mod/...@vX.Y.Z` haqida modul path xatosi chiqsa, arxiv ichida tasodifan Go mod cache (`./go/`) qolgan bo‘ladi. Uni o‘chirib qayta deps o‘rnating:
```bash
rm -rf ./go
make deps
```

## 🎮 Foydalanish

### Oddiy Foydalanuvchilar

#### Asosiy komandalar:
- `/start` - Botni ishga tushirish
- `/help` - Yordam va komandalar ro'yxati
- `/clear` - Chat tarixini tozalash
- `/history` - Chat tarixini ko'rish
- `/products` - Mavjud mahsulotlar ro'yxati

#### Misol suhbatlar:

```
👤 Foydalanuvchi: Assalomu alaykum! Gaming uchun kompyuter kerak

🤖 Bot: Assalomu alaykum! Albatta yordam beraman. Gaming uchun qanday
       budjet mo'ljallagan va qaysi o'yinlarni o'ynaysiz?

👤 Foydalanuvchi: 10 million atrofida. GTA 5, Valorant

🤖 Bot: Juda yaxshi tanlov! Sizga quyidagi konfiguratsiyani tavsiya qilaman:
       - CPU: Intel Core i5-12400F - 2,500,000 so'm
       - GPU: RTX 3060 12GB - 3,800,000 so'm
       ...
```

### Admin Foydalanish

#### 1. Admin sifatida kirish

```
/admin
```

Bot parol so'raydi:
```
🔐 Admin parolini kiriting:
```

Parolni kiriting: `@#12`

Muvaffaqiyatli login:
```
✅ Admin panelga xush kelibsiz!

🔧 Admin imkoniyatlari:
• Excel fayl yuklash orqali mahsulot katalogini yangilash
• Mahsulotlar ro'yxatini ko'rish
• Katalog statistikasi
```

#### 2. Mahsulot katalogini yuklash

Excel fayl tayyorlang quyidagi ustunlar bilan:

| Nomi | Kategoriya | Narx | Tavsif | Soni |
|------|------------|------|--------|------|
| Intel Core i5-12400F | CPU | 2500000 | 6 yadroli, 12 threadli | 10 |
| RTX 4070 | GPU | 5200000 | 12GB GDDR6X | 5 |
| Corsair 16GB DDR4 | RAM | 450000 | 3200MHz | 20 |

Excel faylni botga yuboring. Bot avtomatik qabul qiladi:

```
✅ Katalog muvaffaqiyatli yangilandi!

📦 Yuklangan mahsulotlar: 45 ta
📄 Fayl: products.xlsx

Endi men ushbu mahsulotlar bilan mijozlarga xizmat ko'rsataman!
```

#### 3. Admin komandalar

- `/catalog` - Hozirgi katalog haqida ma'lumot
```
📦 Katalog: products.xlsx
📅 Yangilangan: 2025-01-15 14:30
📊 Jami mahsulotlar: 45 ta

📂 Kategoriyalar:
  • CPU: 8 ta
  • GPU: 12 ta
  • RAM: 10 ta
  • SSD: 15 ta
```

- `/products` - Barcha mahsulotlar ro'yxati
- `/logout` - Admin paneldan chiqish

## 📋 Excel Fayl Formati

### Qo'llab-quvvatlanadigan ustunlar:

**Majburiy:**
- `Nomi` / `Name` - Mahsulot nomi
- `Kategoriya` / `Category` - Mahsulot kategoriyasi
- `Narx` / `Price` - Narx (raqam)

**Ixtiyoriy:**
- `Tavsif` / `Description` - Mahsulot tavsifi
- `Soni` / `Stock` - Ombordagi miqdor

**Qo'shimcha ustunlar:**
Boshqa barcha ustunlar avtomatik "Texnik xususiyatlar" sifatida saqlanadi.

### Misol:

```
| Nomi              | Kategoriya | Narx    | Tavsif                | Soni | Частота | Ядра |
|-------------------|------------|---------|----------------------|------|---------|------|
| i5-12400F         | CPU        | 2500000 | Gaming uchun ideal   | 10   | 4.4 GHz | 6    |
```

### Qo'llab-quvvatlanadigan formatlar:
- `.xlsx` (Excel 2007+)
- `.xls` (Excel 97-2003)

**Maksimal hajm:** 5 MB

## 🔧 Konfiguratsiya

`config/config.go` sozlamalari:

```go
type Config struct {
    TelegramToken  string // Telegram bot token
    GeminiAPIKey   string // Gemini API key
    MaxContextSize int    // Chat tarixida saqlanadigan max xabarlar (default: 20)
    Group1ChatID   int64  // Ixtiyoriy guruh ID
    Group2ChatID   int64  // Ixtiyoriy guruh ID
    ChatDBPath     string // Chat tarix SQLite yo'li (default: ~/.config/upg/chat.db)
}
```

`CHAT_DB_PATH` bo'sh qoldirilsa, bot avtomatik ravishda joriy foydalanuvchining config papkasiga (`~/.config/upg/chat.db`) yozadi, shuning uchun har bir foydalanuvchi uchun yo'l moslashadi.

Admin paroli: [internal/usecase/admin_usecase.go:10](internal/usecase/admin_usecase.go#L10)
```go
const AdminPassword = "@#12"
```

## 📚 Arxitektura Patternlari

### Dependency Injection

Loyihada manual dependency injection ishlatilgan:

```go
// 1. Infrastructure layer yaratish
aiRepo := gemini.NewGeminiClient(apiKey)
productRepo := storage.NewMemoryProductRepository()
adminRepo := storage.NewMemoryAdminRepository()
excelParser := parser.NewExcelParser()

// 2. Use cases yaratish
chatUseCase := usecase.NewChatUseCase(aiRepo, chatRepo, productRepo)
adminUseCase := usecase.NewAdminUseCase(adminRepo, productRepo, excelParser)

// 3. Delivery layer yaratish
botHandler := telegram.NewBotHandler(token, chatUseCase, adminUseCase, productUseCase)
```

### Repository Pattern

Har bir repository interface orqali aniqlanadi:

```go
type ProductRepository interface {
    SaveProduct(ctx context.Context, product entity.Product) error
    GetAll(ctx context.Context) ([]entity.Product, error)
    Search(ctx context.Context, query string) ([]entity.Product, error)
    // ...
}
```

## 🧪 Testing

Test qo'shish uchun mock repository'lar yarating:

```go
type mockProductRepository struct{}

func (m *mockProductRepository) GetAll(ctx context.Context) ([]entity.Product, error) {
    return []entity.Product{
        {Name: "Test Product", Price: 100},
    }, nil
}
```

## 🚧 Kelajakdagi Rejalar

- [ ] PostgreSQL/MySQL database qo'shish
- [ ] Redis caching layer
- [ ] Buyurtma berish tizimi
- [ ] To'lov integratsiyasi (Click, Payme)
- [ ] Admin web panel
- [ ] Statistika va analytics
- [ ] Multi-language support (O'zbek, Rus, Ingliz)
- [ ] Rate limiting va anti-spam
- [ ] Product images support
- [ ] Shopping cart funksiyasi

## 🐛 Debug va Logging

Loglar avtomatik `stdout` va `stderr` ga yoziladi:

```bash
INFO: 2025/01/15 14:30:00 🚀 Ilova ishga tushmoqda...
INFO: 2025/01/15 14:30:01 ✅ Gemini AI client tayyor (gemini-2.0-flash-exp)
INFO: 2025/01/15 14:30:01 ✅ Repositories tayyor (in-memory)
INFO: 2025/01/15 14:30:01 🤖 Bot ishlayapti...
```

## 🤝 Contributing

Pull request'lar qabul qilinadi! Katta o'zgarishlar uchun avval issue oching.

### Yangi funksiya qo'shish:

1. **Domain layer** - Yangi entity yoki repository interface
2. **Infrastructure** - Repository implementation
3. **Use case** - Biznes logika
4. **Delivery** - Telegram handler

## 📄 License

MIT

## 👨‍💻 Muallif

Senior Go Developer - Clean Architecture va Best Practices

---

## 📞 Qo'shimcha Ma'lumot

**Bot ishlash printsipi:**

1. Foydalanuvchi xabar yuboradi
2. Bot chat tarixini yuklaydi
3. Agar mahsulot katalogi mavjud bo'lsa, AI ga kontekst sifatida yuboriladi
4. Gemini AI javob yaratadi
5. Javob foydalanuvchiga yuboriladi va tarixga saqlanadi

**Xavfsizlik:**

- Admin parollari xavfsiz saqlanadi
- Parol kiritilgan xabar avtomatik o'chiriladi
- Session timeout: 24 soat
- File upload: Faqat admin
- Max file size: 5MB

**Performance:**

- In-memory storage: Juda tez
- Concurrent goroutines: Ko'p foydalanuvchilarni qo'llab-quvvatlash
- Context-aware shutdown: Graceful termination

---

**Muammo bo'lsa:**
1. `.env` faylni tekshiring
2. API key'lar to'g'riligini tasdiqlang
3. Loglarni o'qing
4. Issue oching

**Savollar:**
- Telegram: @yourusername
- Email: your@email.com
- GitHub Issues: [issues](https://github.com/yourusername/telegram-ai-bot/issues)
