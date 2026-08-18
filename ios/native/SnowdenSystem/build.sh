#!/bin/bash
# Build script for Snowden System iOS
# Full port of PC algorithms: VLESS+Hysteria2+urltest, split-tunneling, adaptive engine

set -e

# Configuration
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOCORE_DIR="$PROJECT_DIR/GoCore"
BUILD_DIR="$PROJECT_DIR/build"
IOS_DEPLOYMENT_TARGET="15.0"

echo "========================================"
echo "Snowden System iOS — Build Script"
echo "========================================"

# Step 1: Check dependencies
echo "[1/5] Checking dependencies..."

if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Install from https://go.dev/dl/"
    exit 1
fi

if ! command -v gomobile &> /dev/null; then
    echo "📦 Installing gomobile..."
    go install golang.org/x/mobile/cmd/gomobile@latest
    go install golang.org/x/mobile/cmd/gobind@latest
    gomobile init
fi

GO_VERSION=$(go version | awk '{print $3}')
echo "✅ Go: $GO_VERSION"
echo "✅ gomobile: $(gomobile version 2>/dev/null || echo 'installed')"

# Step 2: Build GoCore framework (gomobile bind)
echo ""
echo "[2/5] Building GoCore.xcframework..."
cd "$GOCORE_DIR"

# Download dependencies
echo "  → go mod tidy"
go mod tidy

# Build for iOS (arm64) and iOS Simulator (arm64, x86_64)
echo "  → gomobile bind -target ios"
gomobile bind \
    -target ios \
    -o "$BUILD_DIR/SnowdenCore.xcframework" \
    -tags "with_awg,with_wireguard,with_utls,with_gvisor" \
    -v \
    . || {
        echo "❌ gomobile bind failed"
        echo "   Common issues:"
        echo "   - Missing Xcode: xcode-select --install"
        echo "   - Missing iOS SDK: sudo xcode-select -s /Applications/Xcode.app"
        echo "   - Go version too old: need Go 1.21+"
        exit 1
    }

echo "✅ SnowdenCore.xcframework built"

# Step 3: Copy assets
echo ""
echo "[3/5] Copying assets..."
mkdir -p "$BUILD_DIR/Assets"

# Copy logo assets
if [ -d "$PROJECT_DIR/../../logo_assets" ]; then
    cp "$PROJECT_DIR/../../logo_assets/snowden_system_512x512.png" "$BUILD_DIR/Assets/snowden_logo.png"
    echo "✅ Logo assets copied"
else
    echo "⚠️  Logo assets not found at ../../logo_assets"
fi

# Copy CIDR list
if [ -f "$PROJECT_DIR/../../unkillable-vpn/assets/configs/ru-cidr.lst" ]; then
    cp "$PROJECT_DIR/../../unkillable-vpn/assets/configs/ru-cidr.lst" "$BUILD_DIR/Assets/ru-cidr.lst"
    echo "✅ ru-cidr.lst copied"
else
    echo "⚠️  ru-cidr.lst not found"
fi

# Step 4: Generate Xcode project (if using xcodegen)
echo ""
echo "[4/5] Generating Xcode project..."

if command -v xcodegen &> /dev/null; then
    cd "$PROJECT_DIR"
    xcodegen generate
    echo "✅ Xcode project generated"
else
    echo "⚠️  xcodegen not found. Install: brew install xcodegen"
    echo "   Or open the project manually in Xcode"
fi

# Step 5: Instructions
echo ""
echo "========================================"
echo "[5/5] Build complete!"
echo "========================================"
echo ""
echo "Next steps:"
echo ""
echo "1. Open Xcode project:"
echo "   open '$PROJECT_DIR/SnowdenSystem.xcodeproj'"
echo ""
echo "2. Add SnowdenCore.xcframework to project:"
echo "   - Drag '$BUILD_DIR/SnowdenCore.xcframework' into Xcode"
echo "   - Add to both 'SnowdenSystem' and 'SnowdenSystemVPN' targets"
echo "   - Set 'Embed & Sign'"
echo ""
echo "3. Add assets to app bundle:"
echo "   - Drag '$BUILD_DIR/Assets/' into Xcode"
echo "   - Check 'Create folder references'"
echo ""
echo "4. Configure signing:"
echo "   - Select 'SnowdenSystem' target → Signing & Capabilities"
echo "   - Set Team, Bundle Identifier (com.snowdensystem.app)"
echo "   - Add 'App Groups' capability: group.com.snowdensystem"
echo "   - Add 'Network Extensions' capability"
echo ""
echo "5. Configure VPN extension:"
echo "   - Select 'SnowdenSystemVPN' target → Signing & Capabilities"
echo "   - Set same Team, Bundle ID: com.snowdensystem.app.vpn"
echo "   - Add 'App Groups' capability: group.com.snowdensystem"
echo ""
echo "6. Build and run:"
echo "   - Select device/simulator"
echo "   - Press Cmd+R"
echo ""
echo "========================================"
echo "Build outputs:"
echo "  Framework: $BUILD_DIR/SnowdenCore.xcframework"
echo "  Assets:    $BUILD_DIR/Assets/"
echo "========================================"
