#!/bin/bash

# MindX Windows Package Script - Packages Windows builds from dist/

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

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  MindX Windows Package Build${NC}"
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

# Check dist exists
if [ ! -d "dist" ]; then
    echo "Error: dist/ directory not found. Run build.sh first."
    exit 1
fi

# Clean releases
echo -e "${YELLOW}[1/2] Cleaning previous releases...${NC}"
rm -f releases/mindx-*-windows-*.zip
mkdir -p releases
echo -e "${GREEN}✓ Ready${NC}"
echo ""

# Package Windows builds
echo -e "${YELLOW}[2/2] Packaging Windows builds...${NC}"

mkdir -p releases

package_windows() {
    local DIR="dist/mindx-${VERSION}-windows-$1"
    local OUTPUT="releases/mindx-${VERSION}-windows-$1.zip"
    
    echo "  Packaging $DIR -> $OUTPUT"
    
    if [ -d "$DIR" ]; then
        (cd "$DIR" && zip -q -r "$PROJECT_ROOT/$OUTPUT" .)
        echo -e "${GREEN}  ✓ $OUTPUT${NC}"
    else
        echo -e "${YELLOW}  ⚠ $DIR not found${NC}"
    fi
}

package_windows "amd64"
package_windows "arm64"
echo ""

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Package Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Release packages:"
ls -lh releases/mindx-*-windows-*.zip 2>/dev/null || echo "  None found"
echo ""
