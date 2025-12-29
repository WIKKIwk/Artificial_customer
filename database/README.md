# 🚀 SheetMaster AI

Google Sheets kabi ishlash imkoniyatiga ega bo'lgan AI bilan birlashtirilgan spreadsheet dasturi.

## ✨ Asosiy imkoniyatlar

### 📊 Spreadsheet
- **100+ Formula** - Excel/Google Sheets formulalari
- **Real-time hamkorlik** - Bir vaqtning o'zida ko'p foydalanuvchi ishlashi
- **CRDT** - Konfliktlarsiz ma'lumotlar replikatsiyasi
- **Copy/Paste** - To'liq clipboard qo'llab-quvvatlash
- **Keyboard shortcuts** - Tez ishlash uchun tugmalar kombinatsiyasi

### 🤖 AI Assistant (Gemini 2.5 Flash)
- **Formula generatsiya** - Tabiiy til orqali formula yaratish
- **Ma'lumotlar tahlili** - Trendlar, outlierlar, korrelyatsiya
- **Chart tavsiyalar** - Eng mos vizualizatsiya tavsiyasi
- **Ma'lumotlarni tozalash** - Dublikatlar, xatolar aniqlash
- **Statistika** - Mean, median, std dev va boshqalar

### 🔒 Xavfsizlik
- **JWT autentifikatsiya** - Token-based auth
- **API kalitlar** - Har bir foydalanuvchi o'z AI kalitidan foydalanadi
- **CORS himoya** - Xavfsiz cross-origin requests
- **Rate limiting** - DDoS himoyasi

## 🏗️ Arxitektura

```
┌─────────────────────────────────────────────────────────┐
│                     Frontend (React)                     │
│  Vite + TypeScript + TailwindCSS + Lucide React         │
└─────────────┬───────────────────────────┬───────────────┘
              │                           │
    ┌─────────▼─────────┐       ┌────────▼────────┐
    │  Backend (Go)     │       │ Backend (Elixir)│
    │  Gin Framework    │       │ Phoenix + CRDT  │
    │  REST API         │       │ WebSocket       │
    └─────────┬─────────┘       └────────┬────────┘
              │                          │
    ┌─────────▼──────────────────────────▼────────┐
    │                PostgreSQL 15                 │
    │           Redis (CRDT real-time)             │
    └──────────────────────────────────────────────┘
```

### Technology Stack

**Frontend:**
- React 19.2 + TypeScript
- Vite 6.2 (build tool)
- TailwindCSS 3.4 (styling)
- Lucide React (icons)
- Google Generative AI SDK

**Backend (Go):**
- Gin Web Framework
- GORM (ORM)
- JWT authentication
- PostgreSQL driver

**Backend (Elixir):**
- Phoenix Framework
- Phoenix Channels (WebSocket)
- CRDT (Conflict-free Replicated Data Types)

**Database:**
- PostgreSQL 15
- Redis 7

**DevOps:**
- Docker & Docker Compose
- Nginx (reverse proxy + SSL)
- Let's Encrypt (SSL certificates)

## 🚀 Development Setup

### Talablar:
- Docker & Docker Compose
- Node.js 20+ (local development uchun)
- Go 1.21+ (local development uchun)

### Ishga tushirish:

```bash
# Repository ni clone qilish
git clone <your-repo-url>
cd database

# Environment o'zgaruvchilarini sozlash
cp .env.example .env
nano .env  # Kerakli qiymatlarni kiriting

# Docker Compose bilan ishga tushirish
docker-compose up -d

# Yoki Makefile dan foydalanish
make up
```

Dastur quyidagi addresslarda ochiladi:
- Frontend: http://localhost:8001
- Backend API: http://localhost:8080
- Elixir Backend: http://localhost:4000
- PostgreSQL: localhost:5439

### Foydali buyruqlar:

```bash
# Barcha servislarni ishga tushirish
make up

# Barcha servislarni to'xtatish
make down

# Loglarni ko'rish
make logs

# Backend rebuild qilish
make rebuild-backend

# Frontend rebuild qilish
make rebuild-frontend

# Database ni tozalash (EHTIYOT!)
make clean
```

## 🌐 Production Deployment

Production serverga joylashtirish uchun to'liq qo'llanma: [PRODUCTION_DEPLOYMENT.md](./PRODUCTION_DEPLOYMENT.md)

### Tezkor deployment:

```bash
# 1. .env.production sozlash
cp .env.production.example .env.production
nano .env.production

# 2. Deploy script ishga tushirish
./scripts/deploy-production.sh
```

## 📝 Formulas

100+ ta formula qo'llab-quvvatlanadi:

**Matematik:** SUM, AVERAGE, COUNT, MAX, MIN, ROUND, ABS, SQRT, POWER, MOD...

**Statistika:** MEDIAN, MODE, STDEV, VAR, SUMIF, COUNTIF, AVERAGEIF...

**Mantiqiy:** IF, AND, OR, NOT, XOR, IFS, SWITCH, IFERROR...

**Matn:** LEN, UPPER, LOWER, TRIM, CONCATENATE, LEFT, RIGHT, MID, FIND...

**Sana/Vaqt:** TODAY, NOW, YEAR, MONTH, DAY, DATE, TIME, DAYS...

**Lookup:** VLOOKUP, HLOOKUP, INDEX, MATCH...

To'liq ro'yxat: [Formula Documentation](./docs/formulas.md)

## 🔑 Environment Variables

### Development (.env):
```bash
DB_DSN=host=converter_db user=user password=password dbname=converter_db port=5432
PORT=8080
JWT_SECRET=your-dev-secret
ALLOWED_ORIGINS=http://localhost:8001,http://localhost:5173
```

### Production (.env.production):
Template: `.env.production.example` faylidan nusxa oling va kerakli qiymatlarni kiriting.

## 🐳 Docker Services

| Service | Port | Description |
|---------|------|-------------|
| frontend | 8001 | React + Vite app with Nginx |
| backend-go | 8080 | Go REST API |
| backend-elixir | 4000 | Phoenix WebSocket server |
| converter_db | 5439 | PostgreSQL database |
| redis | 6379 | Redis (CRDT) |

## 🔐 API Endpoints

### Authentication:
- `POST /register` - Foydalanuvchi ro'yxatdan o'tish
- `POST /login` - Login
- `GET /api/me` - Foydalanuvchi ma'lumotlari
- `POST /api/v1/api-key/generate` - API key yaratish (integratsiya uchun)

Auth:
- `Authorization: Bearer <jwt>` yoki `X-API-Key: sk_...`

### Files:
- `GET /api/files` - Fayllar ro'yxati
- `POST /api/files` - Yangi fayl saqlash
- `GET /api/files/:id` - Faylni olish
- `DELETE /api/files/:id` - Faylni o'chirish
- `PATCH /api/v1/files/:id/cells` - Kataklarni batch yangilash
- `GET /api/v1/files/:id/cells?range=A1:D20` - Range bo‘yicha kataklarni o‘qish
- `GET /api/v1/files/:id/schema` - Ustun/headerlar (schema) + used range

To‘liq integratsiya qo‘llanma: [INTEGRATIONS.md](./INTEGRATIONS.md)

### Health:
- `GET /health` - Service health check

## 📦 Project Structure

```
database/
├── backend-go/              # Go REST API
│   ├── cmd/server/         # Main entry point
│   ├── internal/           # Internal packages
│   │   ├── handlers/       # HTTP handlers
│   │   ├── models/         # Database models
│   │   └── services/       # Business logic
│   ├── Dockerfile          # Development
│   └── Dockerfile.prod     # Production
├── backend-elixir/          # Elixir/Phoenix
│   ├── lib/                # Application code
│   ├── Dockerfile
│   └── Dockerfile.prod
├── shlyux/                  # Frontend (React)
│   ├── src/
│   │   ├── components/     # React components
│   │   ├── types/          # TypeScript types
│   │   └── utils/          # Utility functions
│   ├── Dockerfile
│   └── Dockerfile.prod
├── nginx/                   # Nginx configs
│   └── nginx.prod.conf
├── scripts/                 # Deployment scripts
│   ├── backup.sh
│   ├── restore.sh
│   └── deploy-production.sh
├── docker-compose.yml       # Development
├── docker-compose.prod.yml  # Production
└── README.md
```

## 🧪 Testing

```bash
# Frontend tests
cd shlyux
npm test

# Go tests
cd backend-go
go test ./...

# Elixir tests
cd backend-elixir
mix test
```

## 📊 Monitoring

### Logs:
```bash
# Barcha servislar
docker-compose logs -f

# Bitta servis
docker-compose logs -f backend-go

# Production
docker-compose -f docker-compose.prod.yml logs -f
```

### Health checks:
```bash
# API health
curl http://localhost:8080/health

# Frontend health
curl http://localhost:8001/health
```

## 🔧 Troubleshooting

### Database connection errors:
```bash
# Database holatini tekshirish
docker logs converter_db

# Database ichiga kirish
docker exec -it converter_db psql -U user -d converter_db
```

### Frontend build errors:
```bash
# Node modules ni qayta o'rnatish
cd shlyux
rm -rf node_modules
npm install
```

### Port conflicts:
Agar portlar band bo'lsa, `.env` faylida portlarni o'zgartiring.

## 📄 License

MIT License - See LICENSE file for details

## 👥 Contributors

- Your Name - Initial work

## 📞 Support

Issues: [GitHub Issues](https://github.com/your-username/sheetmaster/issues)

---

**Built with ❤️ using React, Go, Elixir, and AI**
