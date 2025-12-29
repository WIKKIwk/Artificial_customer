#!/bin/sh
# PostgreSQL Database Restore Script
# Backupdan ma'lumotlar bazasini tiklaydi

set -e

# Check if backup file is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    echo ""
    echo "Available backups:"
    ls -lh /backups/*.sql.gz 2>/dev/null || echo "No backups found"
    exit 1
fi

BACKUP_FILE="$1"

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: Backup file not found: $BACKUP_FILE"
    exit 1
fi

# Environment variables
POSTGRES_USER=${POSTGRES_USER:-user}
POSTGRES_DB=${POSTGRES_DB:-converter_db}
PGHOST=${PGHOST:-converter_db}

echo "[$(date)] Starting restore from: $BACKUP_FILE"
echo "[$(date)] Target database: $POSTGRES_DB on $PGHOST"
echo ""
echo "WARNING: This will overwrite all data in the database!"
echo "Press Ctrl+C to cancel, or Enter to continue..."
read confirm

# Restore from backup
echo "[$(date)] Restoring database..."
gunzip -c "$BACKUP_FILE" | psql -h "$PGHOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB"

if [ $? -eq 0 ]; then
    echo "[$(date)] Restore completed successfully!"
else
    echo "[$(date)] Restore failed!"
    exit 1
fi
