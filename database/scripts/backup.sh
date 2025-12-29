#!/bin/sh
# PostgreSQL Database Backup Script
# Har kuni avtomatik backup oladi va eski backuplarni o'chiradi

set -e

# Environment variables
POSTGRES_USER=${POSTGRES_USER:-user}
POSTGRES_DB=${POSTGRES_DB:-converter_db}
BACKUP_DIR=${BACKUP_PATH:-/backups}
BACKUP_KEEP_DAYS=${BACKUP_KEEP_DAYS:-30}

# Timestamp
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/${POSTGRES_DB}_${TIMESTAMP}.sql.gz"

echo "[$(date)] Starting backup: $BACKUP_FILE"

# Backup directory yaratish
mkdir -p "$BACKUP_DIR"

# PostgreSQL dump
pg_dump -h "$PGHOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" | gzip > "$BACKUP_FILE"

if [ $? -eq 0 ]; then
    echo "[$(date)] Backup completed successfully: $BACKUP_FILE"

    # Backup hajmini ko'rsatish
    SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    echo "[$(date)] Backup size: $SIZE"
else
    echo "[$(date)] Backup failed!"
    exit 1
fi

# Eski backuplarni o'chirish (30 kundan eski)
echo "[$(date)] Removing backups older than ${BACKUP_KEEP_DAYS} days..."
find "$BACKUP_DIR" -name "*.sql.gz" -type f -mtime +${BACKUP_KEEP_DAYS} -delete

# Qolgan backuplar sonini ko'rsatish
BACKUP_COUNT=$(find "$BACKUP_DIR" -name "*.sql.gz" -type f | wc -l)
echo "[$(date)] Total backups: $BACKUP_COUNT"

echo "[$(date)] Backup process completed"
