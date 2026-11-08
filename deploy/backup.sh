#!/bin/sh
# Резервное копирование БД (REQ-AVAIL-003): ежедневный pg_dump, 7 последних копий.
#   backup.sh                — одна копия;
#   backup.sh --loop         — копия раз в BACKUP_INTERVAL секунд (контейнер backup);
#   backup.sh --restore FILE — восстановление из копии.
set -eu
DIR=${BACKUP_DIR:-/backups}
KEEP=${BACKUP_KEEP:-7}

prune() {
  # Оставляет KEEP самых новых копий в каталоге DIR.
  n=0
  for f in $(ls -1t "$DIR"/shopper-*.dump 2>/dev/null); do
    n=$((n + 1))
    if [ "$n" -gt "$KEEP" ]; then
      rm -f -- "$f"
    fi
  done
}

dump() {
  ts=$(date -u +%Y%m%dT%H%M%SZ)
  file="$DIR/shopper-$ts.dump"
  pg_dump --format=custom --file="$file.tmp"
  mv "$file.tmp" "$file"
  echo "{\"level\":\"INFO\",\"component\":\"backup\",\"msg\":\"backup created\",\"file\":\"$file\"}"
  prune
}

case "${1:-}" in
  --loop)
    until pg_isready -q; do sleep 2; done
    while true; do
      dump
      sleep "${BACKUP_INTERVAL:-86400}"
    done
    ;;
  --restore)
    pg_restore --clean --if-exists --no-owner --dbname="$PGDATABASE" "$2"
    ;;
  *)
    dump
    ;;
esac
