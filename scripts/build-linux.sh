#!/bin/bash

set -e

# MindX Linux Build Script - Builds full Linux release packages

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  MindX Linux Release Build${NC}"
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

# Build function
build_linux() {
    local ARCH=$1
    local ARCH_NAME=$2
    local DIST_DIR="dist/linux-${ARCH_NAME}"
    local TAR_NAME="mindx-${VERSION}-linux-${ARCH_NAME}.tar.gz"

    echo -e "${YELLOW}Building Linux ${ARCH_NAME}...${NC}"
    
    rm -rf "${DIST_DIR}"
    mkdir -p "${DIST_DIR}/bin"
    
    # Build binary
    CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -ldflags="-s -w -X main.Version=${VERSION}" -o "${DIST_DIR}/bin/mindx" ./cmd/main.go
    chmod +x "${DIST_DIR}/bin/mindx"
    echo -e "${GREEN}  ✓ Binary built${NC}"
    
    # Copy skills
    if [ -d "skills" ]; then
        cp -r skills "${DIST_DIR}/"
        echo -e "${GREEN}  ✓ Skills copied${NC}"
    fi
    
    # Copy config templates
    mkdir -p "${DIST_DIR}/config"
    for file in config/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            cp "$file" "${DIST_DIR}/config/${filename}.template"
        fi
    done
    echo -e "${GREEN}  ✓ Config templates copied${NC}"
    
    # Copy scripts
    cp scripts/install.sh "${DIST_DIR}/"
    cp scripts/uninstall.sh "${DIST_DIR}/"
    chmod +x "${DIST_DIR}/install.sh"
    chmod +x "${DIST_DIR}/uninstall.sh"
    echo -e "${GREEN}  ✓ Scripts copied${NC}"
    
    # Copy frontend
    if [ -d "dashboard/dist" ]; then
        cp -r dashboard/dist "${DIST_DIR}/static"
        echo -e "${GREEN}  ✓ Frontend copied${NC}"
    fi
    
    # Copy docs
    if [ -f "README.md" ]; then
        cp README.md "${DIST_DIR}/"
    fi
    if [ -f "README_zh-cn.md" ]; then
        cp README_zh-cn.md "${DIST_DIR}/"
    fi
    if [ -f "VERSION" ]; then
        cp VERSION "${DIST_DIR}/"
    fi
    
    # Create tarball
    cd "${DIST_DIR}"
    tar -czf "../${TAR_NAME}" .
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}  ✓ ${TAR_NAME} created${NC}"
}

# Build both architectures
echo -e "${YELLOW}[3/4] Building Linux binaries...${NC}"
build_linux "amd64" "amd64"
build_linux "arm64" "arm64"
echo ""

# Move to releases
echo -e "${YELLOW}[4/4] Preparing release packages...${NC}"
mkdir -p releases
cp dist/mindx-*-linux-*.tar.gz releases/ 2>/dev/null || true
echo -e "${GREEN}✓ Release packages moved to releases/${NC}"
echo ""

# Summary
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Build Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Release packages:"
ls -lh releases/mindx-*-linux-*.tar.gz 2>/dev/null || echo "  None found"
echo ""
