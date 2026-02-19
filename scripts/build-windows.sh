#!/bin/bash

set -e

# MindX Windows Build Script - Builds full Windows release packages

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  MindX Windows Release Build${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Get version
if [ -f "VERSION" ]; then
    VERSION=$(cat VERSION | tr -d '[:space:]')
else
    VERSION="dev"
fi

echo -e "${CYAN}Version: ${VERSION}${NC}"
echo ""

# Clean previous build
echo -e "${YELLOW}[1/4] Cleaning previous build...${NC}"
rm -rf dist bin
mkdir -p dist bin
echo -e "${GREEN}✓ Clean complete${NC}"
echo ""

# Build frontend
echo -e "${YELLOW}[2/4] Building frontend...${NC}"
cd dashboard
if [ ! -d "node_modules" ]; then
    npm install --silent
fi
npm run build --silent
cd "$PROJECT_ROOT"
echo -e "${GREEN}✓ Frontend built${NC}"
echo ""

# Build Windows binaries
echo -e "${YELLOW}[3/4] Building Windows binaries...${NC}"

# Build Windows AMD64
echo -e "${YELLOW}Building Windows amd64...${NC}"
rm -rf dist/windows-amd64
mkdir -p dist/windows-amd64/bin
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.Version=${VERSION}" -o dist/windows-amd64/bin/mindx.exe cmd/main.go
echo -e "${GREEN}  ✓ Binary built${NC}"

# Copy skills
if [ -d "skills" ]; then
    cp -r skills dist/windows-amd64/
    echo -e "${GREEN}  ✓ Skills copied${NC}"
fi

# Copy config templates
mkdir -p dist/windows-amd64/config
for file in config/*; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        cp "$file" "dist/windows-amd64/config/${filename}.template"
    fi
done
echo -e "${GREEN}  ✓ Config templates copied${NC}"

# Copy scripts
cp scripts/install.sh dist/windows-amd64/ 2>/dev/null || true
cp scripts/uninstall.sh dist/windows-amd64/ 2>/dev/null || true
chmod +x dist/windows-amd64/install.sh 2>/dev/null || true
chmod +x dist/windows-amd64/uninstall.sh 2>/dev/null || true
echo -e "${GREEN}  ✓ Scripts copied${NC}"

# Copy frontend
if [ -d "dashboard/dist" ]; then
    cp -r dashboard/dist dist/windows-amd64/static
    echo -e "${GREEN}  ✓ Frontend copied${NC}"
fi

# Copy README and VERSION
cp README* dist/windows-amd64/ 2>/dev/null || true
cp VERSION dist/windows-amd64/ 2>/dev/null || true

# Create zip
cd dist/windows-amd64
zip -q -r "../mindx-${VERSION}-windows-amd64.zip" .
cd "$PROJECT_ROOT"
echo -e "${GREEN}  ✓ mindx-${VERSION}-windows-amd64.zip created${NC}"

# Build Windows ARM64 (if supported)
if go tool dist list | grep -q "windows/arm64"; then
    echo -e "${YELLOW}Building Windows arm64...${NC}"
    rm -rf dist/windows-arm64
    mkdir -p dist/windows-arm64/bin
    CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -X main.Version=${VERSION}" -o dist/windows-arm64/bin/mindx.exe cmd/main.go
    echo -e "${GREEN}  ✓ Binary built${NC}"
    
    # Copy assets
    if [ -d "skills" ]; then
        cp -r skills dist/windows-arm64/
    fi
    mkdir -p dist/windows-arm64/config
    for file in config/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            cp "$file" "dist/windows-arm64/config/${filename}.template"
        fi
    done
    cp scripts/install.sh dist/windows-arm64/ 2>/dev/null || true
    cp scripts/uninstall.sh dist/windows-arm64/ 2>/dev/null || true
    if [ -d "dashboard/dist" ]; then
        cp -r dashboard/dist dist/windows-arm64/static
    fi
    cp README* dist/windows-arm64/ 2>/dev/null || true
    cp VERSION dist/windows-arm64/ 2>/dev/null || true
    
    # Create zip
    cd dist/windows-arm64
    zip -q -r "../mindx-${VERSION}-windows-arm64.zip" .
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}  ✓ mindx-${VERSION}-windows-arm64.zip created${NC}"
fi

echo ""

# Prepare release packages
echo -e "${YELLOW}[4/4] Preparing release packages...${NC}"
mkdir -p releases
cp dist/mindx-*-windows-*.zip releases/ 2>/dev/null || true
echo -e "${GREEN}✓ Release packages moved to releases/${NC}"
echo ""

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Build Complete!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${CYAN}Release packages:${NC}"
ls -lh releases/mindx-*-windows-*.zip 2>/dev/null || true
echo ""
