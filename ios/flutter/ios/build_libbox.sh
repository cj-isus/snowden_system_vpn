#!/bin/bash
#
# build_libbox.sh — сборка Libbox.xcframework для iOS из sing-box-lx.
#
# ТРЕБОВАНИЯ:
#   - macOS с Xcode 15+
#   - Go 1.22+ (brew install go)
#   - gomobile: go install golang.org/x/mobile/cmd/gomobile@latest
#   - gomobile init (один раз)
#
# ИСПОЛЬЗОВАНИЕ:
#   chmod +x build_libbox.sh
#   ./build_libbox.sh
#
# РЕЗУЛЬТАТ:
#   Libbox.xcframework — копируется в ../Frameworks/

set -e

echo "╔══════════════════════════════════════════════════════╗"
echo "║   snowden.system — libbox.xcframework builder       ║"
echo "╚══════════════════════════════════════════════════════╝"

# Проверка зависимостей
echo "=== Проверка зависимостей ==="

if ! command -v go &> /dev/null; then
    echo "❌ Go не установлен. Установите: brew install go"
    exit 1
fi

if ! command -v gomobile &> /dev/null; then
    echo "❌ gomobile не установлен. Установите:"
    echo "   go install golang.org/x/mobile/cmd/gomobile@latest"
    echo "   gomobile init"
    exit 1
fi

if ! command -v xcodebuild &> /dev/null; then
    echo "❌ Xcode не установлен. Установите из App Store."
    exit 1
fi

echo "✓ Go: $(go version)"
echo "✓ gomobile: $(gomobile version 2>/dev/null || echo 'installed')"
echo "✓ Xcode: $(xcodebuild -version | head -1)"

# Клонирование sing-box-lx
WORK_DIR="$(mktemp -d)"
REPO_URL="https://github.com/SagerNet/sing-box.git"
LX_BRANCH="dev-next"

echo ""
echo "=== Клонирование sing-box-lx → $WORK_DIR ==="
cd "$WORK_DIR"
git clone --depth 1 -b "$LX_BRANCH" "$REPO_URL" sing-box-lx
cd sing-box-lx

# Настройка окружения Go
export GOPROXY="https://goproxy.cn,https://proxy.golang.org,direct"
export GOSUMDB=off
export GOFLAGS="-mod=mod"

echo ""
echo "=== Сборка xcframework (это займёт 5-10 минут) ==="
echo "Включены: with_utls, with_wireguard, with_gvisor, with_clash_api, with_awg"

# gomobile bind — собирает xcframework для iOS + Simulator
gomobile bind -target ios,maccatalyst \
    -ldflags '-s -w' \
    -tags "with_utls,with_wireguard,with_gvisor,with_clash_api,with_awg" \
    -o Libbox.xcframework \
    ./experimental/libbox

echo ""
echo "=== Копирование xcframework ==="
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRAMEWORKS_DIR="$SCRIPT_DIR/Frameworks"
mkdir -p "$FRAMEWORKS_DIR"
cp -R Libbox.xcframework "$FRAMEWORKS_DIR/"

echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║   ✅ Libbox.xcframework собран!                      ║"
echo "║   Расположение: $FRAMEWORKS_DIR/Libbox.xcframework   ║"
echo "╚══════════════════════════════════════════════════════╝"
echo ""
echo "Следующий шаг:"
echo "  1. Открой проект в Xcode"
echo "  2. Перетащи Libbox.xcframework в проект (Embed & Sign)"
echo "  3. Выбери Signing Team (Apple Developer)"
echo "  4. Run на iPhone"
