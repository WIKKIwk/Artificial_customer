# ⚡ SheetMaster AI - Quick Start Guide

## 🚀 Production ga 5 daqiqada deploy qilish

### 1️⃣ Server tayyorlash (Ubuntu 22.04)

```bash
# Docker o'rnatish
sudo apt update
sudo apt install -y docker.io docker-compose-v2 git

# Docker ishga tushirish
sudo systemctl enable docker
sudo systemctl start docker

# User ni docker guruhiga qo'shish
sudo usermod -aG docker $USER
newgrp docker
```

### 2️⃣ Kodni yuklab olish

```bash
cd ~
git clone <your-repo-url> sheetmaster
cd sheetmaster/database
```

### 3️⃣ Environment sozlash

```bash
# Template dan nusxa olish
cp .env.production.example .env.production

# Kuchli parollar generatsiya qilish
echo "DB_PASSWORD=$(openssl rand -base64 48)"
echo "JWT_SECRET=$(openssl rand -base64 64)"
echo "INTERNAL_API_SECRET=$(openssl rand -base64 32)"
echo "ELIXIR_SECRET_KEY_BASE=$(openssl rand -base64 64)"
echo "REDIS_PASSWORD=$(openssl rand -base64 32)"

# .env.production ni tahrirlash
nano .env.production
```

**MUHIM sozlamalar:**
- `DB_USER`, `DB_PASSWORD`, `DB_NAME` - Database
- `JWT_SECRET` - 64+ characters
- `DOMAIN` - Sizning domeningiz
- `ALLOWED_ORIGINS` - Frontend URL
- `SMTP_*` - Email sozlamalari

### 4️⃣ SSL sertifikat olish (Let's Encrypt)

```bash
# Certbot o'rnatish
sudo apt install -y certbot

# Sertifikat olish (80 port ochiq bo'lishi kerak)
sudo certbot certonly --standalone \
  -d yourdomain.com \
  -d www.yourdomain.com \
  -d api.yourdomain.com \
  -d realtime.yourdomain.com

# Sertifikatlarni nusxalash
mkdir -p nginx/ssl
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/ssl/
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/ssl/
sudo chmod 644 nginx/ssl/*.pem
```

### 5️⃣ Deploy qilish

```bash
# Deploy script ishga tushirish
chmod +x scripts/deploy-production.sh
./scripts/deploy-production.sh
```

Yoki qo'lda:

```bash
# Docker image larni build qilish
docker-compose -f docker-compose.prod.yml build

# Ishga tushirish
docker-compose -f docker-compose.prod.yml up -d

# Holatni tekshirish
docker-compose -f docker-compose.prod.yml ps
```

### 6️⃣ DNS sozlash

Domen provayderida (Namecheap, GoDaddy):

```
Type    Name        Value               TTL
A       @           YOUR_SERVER_IP      3600
A       www         YOUR_SERVER_IP      3600
A       api         YOUR_SERVER_IP      3600
A       realtime    YOUR_SERVER_IP      3600
```

---

## 🔥 Development setup (local)

```bash
# Environment yaratish
cp .env.example .env

# Docker bilan ishga tushirish
docker-compose up -d

# Yoki Makefile
make up
```

**URL lar:**
- Frontend: http://localhost:8001
- API: http://localhost:8080
- WebSocket: ws://localhost:4000

---

## 📋 Foydali buyruqlar

### Production:

```bash
# Loglarni ko'rish
docker-compose -f docker-compose.prod.yml logs -f

# Holatni tekshirish
docker-compose -f docker-compose.prod.yml ps

# To'xtatish
docker-compose -f docker-compose.prod.yml down

# Qayta ishga tushirish
docker-compose -f docker-compose.prod.yml restart

# Yangilanishlarni deploy qilish
git pull
docker-compose -f docker-compose.prod.yml build
docker-compose -f docker-compose.prod.yml up -d
```

### Backup:

```bash
# Backup olish (avtomatik har kuni 02:00)
docker exec converter_db /backup.sh

# Backuplarni ko'rish
ls -lh backups/

# Backupdan tiklash
docker exec -it converter_db /restore.sh /backups/file.sql.gz
```

### Monitoring:

```bash
# Health check
curl http://localhost:8080/health

# Container resource usage
docker stats

# Loglar (oxirgi 100 ta)
docker-compose -f docker-compose.prod.yml logs --tail=100
```

---

## 🔧 Troubleshooting

### Database ulanmasa:

```bash
docker logs converter_db
docker exec -it converter_db psql -U user -d converter_db
```

### Frontend ishlamasa:

```bash
docker logs frontend
docker exec frontend nginx -t
```

### SSL muammosi:

```bash
# SSL sertifikatni tekshirish
sudo certbot certificates

# Yangilash
sudo certbot renew
```

---

## 🎯 Checklist (Production)

- [ ] Server tayyorlangan (Docker o'rnatilgan)
- [ ] .env.production sozlangan
- [ ] Kuchli parollar generatsiya qilingan
- [ ] SSL sertifikatlari olindi
- [ ] DNS sozlamalari kiritildi
- [ ] Docker containers ishga tushdi
- [ ] Health check muvaffaqiyatli
- [ ] Backup script ishlayapti
- [ ] SMTP email ishlayapti
- [ ] Frontend ochiladi (HTTPS)
- [ ] API ishlaydi
- [ ] WebSocket ulanadi

---

## 🆘 Support

**Muammo bo'lsa:**
1. Loglarni tekshiring: `docker-compose -f docker-compose.prod.yml logs`
2. Health endpoint: `curl http://localhost:8080/health`
3. To'liq qo'llanma: [PRODUCTION_DEPLOYMENT.md](./PRODUCTION_DEPLOYMENT.md)

---

## 🎉 Tayyor!

Dasturingiz ishlamoqda:
- 🌐 Frontend: https://yourdomain.com
- 🔌 API: https://api.yourdomain.com
- ⚡ WebSocket: wss://realtime.yourdomain.com/socket

**Happy coding! 🚀**
