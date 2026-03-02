#!/bin/bash

# MindX Build Script - Builds executables and dashboard for all platforms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  MindX Build Script${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Get version
if [ -f "VERSION" ]; then
    VERSION=$(cat VERSION | tr -d '[:space:]')
else
    VERSION="dev"
fi
echo -e "${CYAN}Version: ${VERSION}${NC}"
echo ""

# Clean and prepare
echo -e "${YELLOW}[1/5] Preparing...${NC}"
rm -rf dist
mkdir -p dist
echo -e "${GREEN}✓ Ready${NC}"
echo ""

# Build frontend
echo -e "${YELLOW}[2/5] Building frontend...${NC}"
cd dashboard
if [ ! -d "node_modules" ]; then
    npm install --silent
fi
npm run build --silent
cd "$PROJECT_ROOT"
echo -e "${GREEN}✓ Frontend built${NC}"
echo ""

# Build function
build_binary() {
    local OS=$1
    local ARCH=$2
    local OUTPUT_NAME=$3
    
    echo -e "${YELLOW}Building ${OS}/${ARCH}...${NC}"
    
    local BUILD_DIR="dist/${OUTPUT_NAME}"
    mkdir -p "$BUILD_DIR/bin"
    
    if [ "$OS" = "windows" ]; then
        CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" \
            go build -ldflags="-s -w -X main.Version=${VERSION}" \
            -o "$BUILD_DIR/bin/mindx.exe" ./cmd/main.go
    else
        CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" \
            go build -ldflags="-s -w -X main.Version=${VERSION}" \
            -o "$BUILD_DIR/bin/mindx" ./cmd/main.go
        chmod +x "$BUILD_DIR/bin/mindx"
    fi
    
    # Copy skills
    if [ -d "skills" ]; then
        cp -r skills "$BUILD_DIR/"
    fi
    
    # Copy config templates
    mkdir -p "$BUILD_DIR/config"
    for file in config/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            cp "$file" "$BUILD_DIR/config/${filename}.template"
        fi
    done
    
    # Copy frontend
    if [ -d "dashboard/dist" ]; then
        cp -r dashboard/dist "$BUILD_DIR/static"
    fi
    
    # Copy scripts
    cp scripts/install.sh "$BUILD_DIR/" 2>/dev/null || true
    cp scripts/uninstall.sh "$BUILD_DIR/" 2>/dev/null || true
    cp scripts/ollama.sh "$BUILD_DIR/" 2>/dev/null || true
    cp scripts/install.bat "$BUILD_DIR/" 2>/dev/null || true
    cp scripts/uninstall.bat "$BUILD_DIR/" 2>/dev/null || true
    cp VERSION "$BUILD_DIR/" 2>/dev/null || true
    cp README.md "$BUILD_DIR/" 2>/dev/null || true
    
    echo -e "${GREEN}  ✓ dist/${OUTPUT_NAME}${NC}"
}

# Build all platforms
echo -e "${YELLOW}[3/5] Building binaries...${NC}"
build_binary "darwin" "amd64" "mindx-${VERSION}-darwin-amd64"
build_binary "darwin" "arm64" "mindx-${VERSION}-darwin-arm64"
build_binary "linux" "amd64" "mindx-${VERSION}-linux-amd64"
build_binary "linux" "arm64" "mindx-${VERSION}-linux-arm64"
build_binary "windows" "amd64" "mindx-${VERSION}-windows-amd64"
build_binary "windows" "arm64" "mindx-${VERSION}-windows-arm64"
echo ""

# Build current platform binary
echo -e "${YELLOW}[4/5] Building local binary...${NC}"
mkdir -p bin
if [ "$(uname -s)" = "Darwin" ]; then
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.Version=${VERSION}" -o dist/mindx-darwin-amd64 ./cmd/main.go
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.Version=${VERSION}" -o dist/mindx-darwin-arm64 ./cmd/main.go
    lipo -create -output bin/mindx dist/mindx-darwin-amd64 dist/mindx-darwin-arm64
    rm -f dist/mindx-darwin-amd64 dist/mindx-darwin-arm64
    echo -e "${GREEN}✓ bin/mindx (Universal Binary)${NC}"
else
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${VERSION}" -o bin/mindx ./cmd/main.go
    echo -e "${GREEN}✓ bin/mindx${NC}"
fi
echo ""

echo -e "${YELLOW}[5/5] Build complete${NC}"
echo ""

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Build Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Build directories (in dist/):"
ls -ld dist/mindx-* 2>/dev/null || echo "  None found"
echo ""
echo "Local binary:"
ls -lh bin/mindx 2>/dev/null || echo "  None found"
echo ""
echo "Now run packaging scripts:"
echo "  ./scripts/build_pkg.sh"
echo "  ./scripts/build-linux.sh"
echo "  ./scripts/build-windows.sh"
