# 🚀 SheetMaster AI - Production Deployment Guide

Production serverga joylashtirish uchun to'liq qo'llanma.

## 📋 Talablar

### Server talablari:
- **OS**: Ubuntu 22.04 LTS yoki yangi
- **RAM**: Minimum 4GB (8GB+ tavsiya etiladi)
- **CPU**: 2+ core (4+ core yaxshi)
- **Disk**: 50GB+ (SSD tavsiya)
- **Network**: Public IP manzil
- **Domain**: DNS manzil (masalan: `yourdomain.com`)

### Zarur dasturlar:
```bash
# Docker va Docker Compose
sudo apt update
sudo apt install -y docker.io docker-compose-v2 git

# Docker xizmatini ishga tushirish
sudo systemctl enable docker
sudo systemctl start docker

# User ni docker guruhiga qo'shish
sudo usermod -aG docker $USER
```

---

## 🔐 1. SSL Sertifikat olish (Let's Encrypt)

### Certbot o'rnatish:
```bash
sudo apt install -y certbot python3-certbot-nginx
```

### SSL sertifikat olish:
```bash
# Domenlaringiz uchun sertifikat olish
sudo certbot certonly --standalone -d yourdomain.com -d www.yourdomain.com -d api.yourdomain.com -d realtime.yourdomain.com

# Sertifikatlar /etc/letsencrypt/live/yourdomain.com/ da saqlanadi
```

### Sertifikatlarni loyihaga ko'chirish:
```bash
# SSL papka yaratish
mkdir -p ~/sheetmaster/nginx/ssl

# Sertifikatlarni nusxalash
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem ~/sheetmaster/nginx/ssl/
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem ~/sheetmaster/nginx/ssl/

# Permission berish
sudo chmod 644 ~/sheetmaster/nginx/ssl/*.pem
```

### Auto-renewal sozlash:
```bash
# Certbot avtomatik yangilashni sozlash
sudo systemctl enable certbot.timer
sudo systemctl start certbot.timer
```

---

## 📥 2. Kodni serverga ko'chirish

```bash
# Uy papkasiga o'tish
cd ~

# Repository clone qilish
git clone https://github.com/your-username/sheetmaster.git sheetmaster
cd sheetmaster/database
```

---

## ⚙️ 3. Environment o'zgaruvchilarini sozlash

### .env.production yaratish:
```bash
# Template dan nusxa olish
cp .env.production.example .env.production

# Faylni tahrirlash
nano .env.production
```

### MUHIM: Quyidagi qiymatlarni o'zgartiring:

```bash
# 1. Database credentials
DB_USER=sheetmaster_user
DB_PASSWORD=KUCHLI_PAROL_64_XARAKTER  # openssl rand -base64 48
DB_NAME=sheetmaster_production

# 2. JWT Secret
JWT_SECRET=KUCHLI_JWT_SECRET_64_XARAKTER  # openssl rand -base64 64

# 2.1. Internal API secret (Go <-> Realtime bridge)
INTERNAL_API_SECRET=KUCHLI_INTERNAL_SECRET_32_XARAKTER  # openssl rand -base64 32

# 3. Elixir Secret
ELIXIR_SECRET_KEY_BASE=KUCHLI_SECRET_64_XARAKTER  # openssl rand -base64 64

# 4. Redis password
REDIS_PASSWORD=KUCHLI_REDIS_PAROL  # openssl rand -base64 32

# 5. Domen sozlamalari
DOMAIN=yourdomain.com
API_DOMAIN=api.yourdomain.com
REALTIME_DOMAIN=realtime.yourdomain.com

# 6. CORS origins
ALLOWED_ORIGINS=https://yourdomain.com,https://www.yourdomain.com

# 7. Email (SMTP)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=noreply@yourdomain.com
```

### Kuchli parol generatsiya qilish:
```bash
# JWT Secret
openssl rand -base64 64

# Database password
openssl rand -base64 48

# Redis password
openssl rand -base64 32
```

---

## 🎯 4. Nginx konfiguratsiyasini sozlash

### Domenlarni o'zgartirish:
```bash
# nginx.prod.conf faylini tahrirlash
nano nginx/nginx.prod.conf

# Quyidagi joylarni o'zgartiring:
# - yourdomain.com -> sizning domeningiz
# - api.yourdomain.com -> sizning API domeningiz
# - realtime.yourdomain.com -> sizning realtime domeningiz
```

---

## 🐳 5. Docker Compose sozlash

### docker-compose.prod.yml da URL larni o'zgartirish:
```bash
nano docker-compose.prod.yml

# Frontend build args ni o'zgartiring:
# VITE_API_BASE=https://api.yourdomain.com
# VITE_REALTIME_URL=wss://realtime.yourdomain.com/socket
```

---

## 🚀 6. Production ishga tushirish

### Birinchi marta ishga tushirish:
```bash
# Docker image larni build qilish
docker-compose -f docker-compose.prod.yml build

# Barcha konteynerlarni ishga tushirish
docker-compose -f docker-compose.prod.yml up -d

# Loglarni ko'rish
docker-compose -f docker-compose.prod.yml logs -f
```

### Xizmat holatini tekshirish:
```bash
# Barcha konteynerlar holatini ko'rish
docker-compose -f docker-compose.prod.yml ps

# Health check
curl http://localhost:8080/health
```

---

## 🔍 7. DNS sozlamalari

Domen provayderi (Namecheap, GoDaddy, va hokazo) da quyidagi DNS recordlarni qo'shing:

```
Type    Name        Value               TTL
A       @           SERVER_IP_ADDRESS   3600
A       www         SERVER_IP_ADDRESS   3600
A       api         SERVER_IP_ADDRESS   3600
A       realtime    SERVER_IP_ADDRESS   3600
```

DNS yangilanishini tekshirish:
```bash
# Ping qilib ko'rish
ping yourdomain.com
ping api.yourdomain.com
```

---

## 🔄 8. Backup sozlash

### Avtomatik backup faollashtirilgan:
- Har kuni soat 02:00 da avtomatik backup olinadi
- Backuplar `/backups` papkasida saqlanadi
- 30 kundan eski backuplar avtomatik o'chiriladi

### Qo'lda backup olish:
```bash
# Backup olish
docker exec converter_db /backup.sh

# Backuplarni ko'rish
ls -lh backups/
```

### Backupdan tiklash:
```bash
# Backup faylini tanlash
ls -lh backups/

# Tiklash
docker exec -it converter_db /restore.sh /backups/converter_db_20240101_020000.sql.gz
```

---

## 📊 9. Monitoring va Logging

### Loglarni ko'rish:
```bash
# Barcha servislar
docker-compose -f docker-compose.prod.yml logs -f

# Bitta servis
docker-compose -f docker-compose.prod.yml logs -f backend-go

# Oxirgi 100 ta log
docker-compose -f docker-compose.prod.yml logs --tail=100
```

### Container holatini tekshirish:
```bash
# Barcha konteynerlar
docker ps

# Resource usage
docker stats
```

---

## 🔧 10. Maintenance Commands

### Dasturni yangilash:
```bash
# Yangi kodni olish
git pull origin main

# Rebuild va restart
docker-compose -f docker-compose.prod.yml build
docker-compose -f docker-compose.prod.yml up -d
```

### Barcha konteynerlarni to'xtatish:
```bash
docker-compose -f docker-compose.prod.yml down
```

### Ma'lumotlar bazasini tozalash (EHTIYOT!):
```bash
# Volume larni ham o'chirish
docker-compose -f docker-compose.prod.yml down -v
```

### Database migration:
```bash
# Go backend migratsiya avtomatik ishga tushadi
# Agar qo'lda kerak bo'lsa:
docker exec -it backend-go /bin/sh
# Ichida migration buyruqlarini bajaring
```

---

## 🛡️ 11. Xavfsizlik

### Firewall sozlash:
```bash
# UFW o'rnatish va sozlash
sudo apt install -y ufw

# SSH ruxsat berish
sudo ufw allow 22/tcp

# HTTP/HTTPS ruxsat berish
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Faollashtirish
sudo ufw enable
sudo ufw status
```

### Regular yangilanishlar:
```bash
# System yangilanishlari
sudo apt update && sudo apt upgrade -y

# Docker image yangilanishlari
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d
```

---

## 🐛 12. Troubleshooting

### Database ulanmasa:
```bash
# Database konteyner holatini tekshirish
docker logs converter_db

# Database ichiga kirish
docker exec -it converter_db psql -U sheetmaster_user -d sheetmaster_production
```

### Frontend ishlamasa:
```bash
# Frontend logs
docker-compose -f docker-compose.prod.yml logs frontend

# Nginx konfiguratsiyani tekshirish
docker exec frontend nginx -t
```

### WebSocket ishlamasa:
```bash
# Elixir backend logs
docker-compose -f docker-compose.prod.yml logs backend-elixir

# Redis holatini tekshirish
docker exec converter_redis redis-cli ping
```

---

## 📈 13. Performance Optimization

### Database tuning:
```bash
# PostgreSQL konfiguratsiyasi
# shared_buffers, effective_cache_size sozlash
# /var/lib/postgresql/data/postgresql.conf
```

### Nginx cache sozlash:
```bash
# Static file cache kengaytirish
# nginx.prod.conf da proxy_cache sozlamalari
```

---

## 🎉 14. Tayyor!

Dasturingiz production da ishlayapti! Quyidagi URL larga kiring:

- **Frontend**: https://yourdomain.com
- **API**: https://api.yourdomain.com/health
- **WebSocket**: wss://realtime.yourdomain.com/socket

---

## 📞 Support

Muammolar bo'lsa:
1. Loglarni tekshiring: `docker-compose -f docker-compose.prod.yml logs`
2. Health endpoint ni tekshiring: `curl http://localhost:8080/health`
3. Database connection ni tekshiring

---

## 📝 Eslatmalar

- **SSL sertifikatlar** har 90 kunda avtomatik yangilanadi (Certbot)
- **Database backuplar** har kuni 02:00 da olinadi
- **30 kundan eski backuplar** avtomatik o'chiriladi
- **Loglar** 100MB hajmga cheklangan va 5 ta fayl saqlanadi

**MUHIM**: `.env.production` faylini hech qachon git ga commit qilmang!
