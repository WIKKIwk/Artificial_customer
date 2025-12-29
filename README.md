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
- 🔐 **Parol bilan himoyalangan** - Environment variable orqali (`.env`: `ADMIN_PASSWORD`)
- 📤 **Excel yuklash** - Mahsulot katalogini Excel fayldan yuklash (max 5MB)
- 📊 **Katalog boshqaruvi** - Mahsulotlar va kategoriyalarni ko'rish
- 📝 **Admin log** - Barcha admin harakatlari loglanadi
- 🗄️ **Database (SheetMaster) sync** - Katalogni API orqali yangilash
- 🕐 **Session timeout** - 24 soat avtomatik logout

### 📦 Mahsulot Katalogi
- 🗂️ **Excel import** - .xlsx va .xls formatlarini qo'llab-quvvatlash
- 🗄️ **Database import** - API orqali katalogni yangilash (/db\_sync)
- 🔄 **Multi-admin** - Bir necha admin bir vaqtda tahrirlashi mumkin
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

### 1. Repository ni clone qiling

```bash
git clone <repository-url>
cd telegram-ai-bot
```

### 2. Dependencies ni o'rnating

```bash
go mod download
```

Yoki Makefile dan foydalaning:

```bash
make deps
```

### 3. Environment faylni sozlang

```bash
cp .env.example .env
```

`.env` faylini tahrirlang:

```env
TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
GEMINI_API_KEY=AIzaSy...
ADMIN_PASSWORD=your_secure_password_here
```

**MUHIM:** Admin parolni murakkab va xavfsiz parolga o'zgartiring!

**Postgres (buyurtmalar uchun):**
- Docker Compose bilan ishga tushirganda Postgres avtomatik ishga tushadi va DB yaratiladi, `POSTGRES_DSN` ni `.env` ga yozish shart emas.
- Agar tashqi Postgres ishlatmoqchi bo'lsangiz, `.env` da `POSTGRES_DSN` ni kiriting.

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

### 4. Botni ishga tushiring

**Development:**
```bash
go run cmd/bot/main.go
```

Yoki Makefile:
```bash
make run
```

**Production:**
```bash
make build
./bot
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

Parolni kiriting (`.env` faylda sozlangan parol)

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

**Katalog boshqaruvi:**
- `/catalog` - Hozirgi katalog haqida ma'lumot
- `/products` - Barcha mahsulotlar ro'yxati
- `/search` - Mahsulot qidirish (so'rovni keyin yozing yoki `/search <query>` ishlating)
- `/clean` - Ma'lumotlarni tozalash

**Database (SheetMaster):**
- `/db_status` - Ulanish holatini ko'rish
- `/db_sync` - Katalogni yangilash (API → XLSX → katalog + CSV)
- `/import` - `/db_sync` alias
- `/database_select` - Import qilinadigan faylni tanlash

**Foydalanuvchilar:**
- `/online` - Onlayn statistika
- `/users` - Foydalanuvchilar ro'yxati
- `/broadcast <xabar>` - Hammaga xabar yuborish

**Buyurtmalar:**
- `/orders [limit]` - So'nggi buyurtmalar
- `/top [limit]` - TOP mahsulotlar

**Sozlamalar:**
- `/val` - Valyuta rejimini o'zgartirish
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
    AdminPassword  string // Admin panel paroli (.env dan)
    MaxContextSize int    // Chat tarixida saqlanadigan max xabarlar
}
```

**Konstant qiymatlar:** [internal/domain/constants/constants.go](internal/domain/constants/constants.go)
```go
const (
    DefaultMaxContextSize = 20  // Chat tarixida max xabarlar
    DefaultSessionTimeout = 24  // Admin session timeout (soat)
    MaxFileUploadSize = 5MB     // Maksimal fayl hajmi
    GeminiModelName = "gemini-2.5-flash"
    AITemperature = 0.3         // AI javob aniqlik darajasi
    MaxRetries = 3              // AI ga so'rov max urinishlar
    RetryDelay = 10             // Har bir urinish orasida kutish (s)
)
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
botHandler := telegram.NewBotHandler(
    token,
    group1ChatID,      // masalan: -1001234567890 yoki -1001234567890/2 (forum topic)
    group1ThreadID,    // yuqoridagi qiymatdan olinadi, 0 bo'lsa forum topiksiz
    group2ChatID,
    group2ThreadID,
    chatUseCase,
    adminUseCase,
    productUseCase,
)
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

- Admin paroli `.env` faylda (environment variable)
- Parol kiritilgan xabar avtomatik o'chiriladi
- Session timeout: 24 soat (avtomatik logout)
- File upload: Faqat admin
- Max file size: 5MB
- Magic numbers va strings constants da

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
