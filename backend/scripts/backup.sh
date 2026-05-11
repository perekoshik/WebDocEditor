#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
DATABASE_URL="${DATABASE_URL:-postgres://localhost:5432/legaledit?sslmode=disable}"
FILES_DIR="${FILES_DIR:-./data/uploads}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

mkdir -p "$BACKUP_DIR"

stamp="$(date +%Y-%m-%d_%H-%M-%S)"
db_dump="$BACKUP_DIR/legaledit-db-$stamp.sql.gz"
files_archive="$BACKUP_DIR/legaledit-files-$stamp.tar.gz"

echo "==> pg_dump -> $db_dump"
pg_dump "$DATABASE_URL" | gzip > "$db_dump"

if [ -d "$FILES_DIR" ]; then
    echo "==> tar files dir -> $files_archive"
    tar -czf "$files_archive" -C "$(dirname "$FILES_DIR")" "$(basename "$FILES_DIR")"
fi

echo "==> pruning backups older than $RETENTION_DAYS days"
find "$BACKUP_DIR" -type f -name 'legaledit-*' -mtime +"$RETENTION_DAYS" -print -delete || true

echo "done"
