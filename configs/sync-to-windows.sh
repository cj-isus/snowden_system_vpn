#!/usr/bin/env bash
# Применяет конфиги из configs/ (источник истины) к рабочей копии приложения
# (windows/assets/configs/ и windows/.env).
#
# Использование: bash configs/sync-to-windows.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/configs/singbox"
DEST="$ROOT/windows/assets/configs"
ENV_SRC="$ROOT/configs/env/.env"
ENV_DEST="$ROOT/windows/.env"

mkdir -p "$DEST"

# sing-box конфиги
for f in "$SRC"/*; do
  [ -e "$f" ] || continue
  cp "$f" "$DEST/$(basename "$f")"
  echo "synced $(basename "$f")"
done

# .env рядом с модулем
if [ -f "$ENV_SRC" ]; then
  cp "$ENV_SRC" "$ENV_DEST"
  echo "synced .env"
fi

echo "OK: конфиги применены к windows/assets/configs"