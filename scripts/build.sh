#!/bin/bash

# MindX Build Script - Prepares deliverable files for release

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  MindX Build Script${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Read version
# Priority: git tag > VERSION file > dev-YYYYMMDD
if git describe --tags --exact-match HEAD >/dev/null 2>&1; then
    VERSION=$(git describe --tags --exact-match HEAD)
    echo -e "${CYAN}✓ Using git tag: ${VERSION}${NC}"
elif [ -f "VERSION" ]; then
    VERSION=$(cat VERSION | tr -d '[:space:]')
    echo -e "${CYAN}✓ Using VERSION file: ${VERSION}${NC}"
else
    VERSION="dev-$(date +%Y%m%d)"
    echo -e "${YELLOW}⚠ No git tag or VERSION file found, using: ${VERSION}${NC}"
fi

echo -e "${BLUE}Version: ${VERSION}${NC}"
echo ""

# Clean previous build
echo -e "${YELLOW}[1/6] Cleaning previous build...${NC}"
if [ -d "dist" ]; then
    rm -rf "dist"
    echo -e "${GREEN}✓ Removed old dist directory${NC}"
fi
if [ -d "bin" ]; then
    rm -rf "bin"
    echo -e "${GREEN}✓ Removed old bin directory${NC}"
fi
mkdir -p bin
mkdir -p dist
echo -e "${GREEN}✓ Clean complete${NC}"
echo ""

# Check prerequisites
echo -e "${YELLOW}[2/6] Checking prerequisites...${NC}"

# Check Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go is not installed${NC}"
    echo "Please install Go 1.21 or later"
    exit 1
fi
echo -e "${GREEN}✓ Go $(go version | awk '{print $3}')${NC}"

# Check Node.js (for dashboard)
BUILD_DASHBOARD=true
if ! command -v node &> /dev/null; then
    echo -e "${YELLOW}⚠ Node.js is not installed${NC}"
    echo -e "${YELLOW}  Dashboard will not be built${NC}"
    BUILD_DASHBOARD=false
else
    echo -e "${GREEN}✓ Node.js $(node -v)${NC}"
    if ! command -v npm &> /dev/null; then
        echo -e "${YELLOW}⚠ npm is not installed${NC}"
        echo -e "${YELLOW}  Dashboard will not be built${NC}"
        BUILD_DASHBOARD=false
    else
        echo -e "${GREEN}✓ npm $(npm -v)${NC}"
    fi
fi
echo ""

# Build Go binary
echo -e "${YELLOW}[3/6] Building Go binary...${NC}"

BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build for current platform
echo -e "${BLUE}Building for current platform...${NC}"

CGO_ENABLED=1 \
    go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o bin/mindx \
    ./cmd/main.go

echo -e "${GREEN}✓ Built bin/mindx${NC}"
echo ""

# Build dashboard if requested
if [ "$BUILD_DASHBOARD" = true ]; then
    echo -e "${YELLOW}[4/6] Building dashboard...${NC}"
    
    if [ -d "dashboard" ]; then
        echo -e "${BLUE}Building React dashboard...${NC}"
        cd dashboard
        
        # Install dependencies if node_modules not exists
        if [ ! -d "node_modules" ]; then
            echo -e "${CYAN}Installing npm dependencies...${NC}"
            npm install --silent
        fi
        
        # Build
        echo -e "${CYAN}Building dashboard...${NC}"
        npm run build --silent
        
        cd "$PROJECT_ROOT"
        
        # Copy build output to static directory (Vite outputs to dist, not build)
        if [ -d "dashboard/dist" ]; then
            mkdir -p dist/static
            cp -r dashboard/dist/* dist/static/
            echo -e "${GREEN}✓ Dashboard built and copied to dist/static${NC}"
        else
            echo -e "${YELLOW}⚠ Dashboard build output not found${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ dashboard directory not found, skipping${NC}"
    fi
else
    echo -e "${YELLOW}[4/6] Skipping dashboard build (Node.js/npm not available)${NC}"
fi
echo ""

# Prepare distribution package
echo -e "${YELLOW}[5/6] Preparing distribution package...${NC}"

# Copy binary to dist
cp bin/mindx dist/

# Copy skills
if [ -d "skills" ]; then
    cp -r skills dist/
    echo -e "${GREEN}✓ Copied skills${NC}"
fi

# Copy config templates
if [ -d "config" ]; then
    mkdir -p dist/config
    for file in config/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            cp "$file" "dist/config/${filename}.template"
        fi
    done
    echo -e "${GREEN}✓ Copied config templates${NC}"
fi

# Copy official docs if available
if [ -d "official" ]; then
    cp -r official dist/
    echo -e "${GREEN}✓ Copied official documentation${NC}"
fi

# Copy scripts
cp scripts/install.sh dist/
cp scripts/uninstall.sh dist/
chmod +x dist/install.sh
chmod +x dist/uninstall.sh
echo -e "${GREEN}✓ Copied install/uninstall scripts${NC}"

# Copy VERSION
if [ -f "VERSION" ]; then
    cp VERSION dist/
fi

# Copy READMEs
if [ -f "README.md" ]; then
    cp README.md dist/
fi
if [ -f "README_zh-cn.md" ]; then
    cp README_zh-cn.md dist/
fi

echo -e "${GREEN}✓ Distribution package prepared in dist/${NC}"
echo ""

# Create release archive
echo -e "${YELLOW}[6/6] Creating release archive...${NC}"

# Determine platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "aarch64" ]; then
    ARCH="arm64"
fi

RELEASE_NAME="mindx-${VERSION}-${OS}-${ARCH}"

cd dist
if command -v zip &> /dev/null; then
    zip -q -r "../${RELEASE_NAME}.zip" .
    echo -e "${GREEN}✓ Created ${RELEASE_NAME}.zip${NC}"
else
    tar -czf "../${RELEASE_NAME}.tar.gz" .
    echo -e "${GREEN}✓ Created ${RELEASE_NAME}.tar.gz${NC}"
fi
cd "$PROJECT_ROOT"

# Create releases directory and copy archive
mkdir -p releases
if [ -f "${RELEASE_NAME}.zip" ]; then
    mv "${RELEASE_NAME}.zip" releases/
    echo -e "${GREEN}✓ Copied to releases/${RELEASE_NAME}.zip${NC}"
elif [ -f "${RELEASE_NAME}.tar.gz" ]; then
    mv "${RELEASE_NAME}.tar.gz" releases/
    echo -e "${GREEN}✓ Copied to releases/${RELEASE_NAME}.tar.gz${NC}"
fi

echo ""

# Print summary
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Build Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Build artifacts:"
echo "  - bin/mindx (Go binary)"
echo "  - dist/ (Full distribution package)"
if [ -f "releases/${RELEASE_NAME}.zip" ]; then
    echo "  - releases/${RELEASE_NAME}.zip (Release archive)"
elif [ -f "releases/${RELEASE_NAME}.tar.gz" ]; then
    echo "  - releases/${RELEASE_NAME}.tar.gz (Release archive)"
fi
echo ""
echo "To install:"
echo "  cd dist && ./install.sh"
echo ""
