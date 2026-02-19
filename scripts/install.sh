#!/bin/bash

# MindX Installation Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  MindX Installation Script${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Check if running from source directory or from release package
if [ -f "cmd/main.go" ]; then
    INSTALL_MODE="source"
    echo -e "${BLUE}Installation mode: Source${NC}"
else
    INSTALL_MODE="release"
    echo -e "${BLUE}Installation mode: Release package${NC}"
fi

# Read version
if [ -f "VERSION" ]; then
    VERSION=$(cat VERSION | tr -d '[:space:]')
else
    VERSION="latest"
fi

echo -e "${BLUE}Version: ${VERSION}${NC}"
echo ""

# Check prerequisites
echo -e "${YELLOW}[1/9] Checking prerequisites...${NC}"

# Check Go (only for source mode)
if [ "$INSTALL_MODE" = "source" ]; then
    if ! command -v go &> /dev/null; then
        echo -e "${RED}✗ Go is not installed${NC}"
        echo "Please install Go 1.21 or later"
        exit 1
    fi
    echo -e "${GREEN}✓ Go $(go version | awk '{print $3}')${NC}"
fi

# Check Node.js (for dashboard - source only)
if [ "$INSTALL_MODE" = "source" ]; then
    if ! command -v node &> /dev/null; then
        echo -e "${YELLOW}⚠ Node.js is not installed (dashboard will not be built)${NC}"
    else
        echo -e "${GREEN}✓ Node.js $(node -v)${NC}"
    fi
fi

# Check Ollama (required)
if command -v ollama &> /dev/null; then
    echo -e "${GREEN}✓ Ollama is installed${NC}"
    OLLAMA_AVAILABLE=true
else
    echo -e "${YELLOW}⚠ Ollama is not installed, installing now...${NC}"
    
    # Install Ollama
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        echo -e "${BLUE}  Installing Ollama for macOS...${NC}"
        curl -fsSL https://ollama.com/install.sh | sh
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        echo -e "${BLUE}  Installing Ollama for Linux...${NC}"
        curl -fsSL https://ollama.com/install.sh | sh
    else
        echo -e "${RED}✗ Unsupported OS for automatic Ollama installation${NC}"
        echo "Please install Ollama manually from https://ollama.com"
        exit 1
    fi
    
    # Verify installation
    if command -v ollama &> /dev/null; then
        echo -e "${GREEN}✓ Ollama installed successfully${NC}"
        OLLAMA_AVAILABLE=true
    else
        echo -e "${RED}✗ Ollama installation failed${NC}"
        exit 1
    fi
fi

echo ""

# Load environment variables
echo -e "${YELLOW}[2/9] Loading configuration...${NC}"

# Read from .env if exists, otherwise use default
if [ -f ".env" ]; then
    source .env
    MINDX_PATH="${MINDX_PATH:-/usr/local/mindx}"
    MINDX_WORKSPACE="${MINDX_WORKSPACE:-~/.mindx}"
    echo -e "${GREEN}✓ Loaded .env file${NC}"
else
    MINDX_PATH="${MINDX_PATH:-/usr/local/mindx}"
    MINDX_WORKSPACE="${MINDX_WORKSPACE:-}"
fi

# Interactive workspace selection
if [ -z "$MINDX_WORKSPACE" ]; then
    echo ""
    echo -e "${BLUE}Please choose your workspace directory:${NC}"
    echo ""
    echo "  1) Default: ~/.mindx"
    echo "  2) Custom directory"
    echo ""
    
    while true; do
        read -p "Enter your choice (1 or 2): " choice
        case $choice in
            1)
                MINDX_WORKSPACE="$HOME/.mindx"
                echo -e "${GREEN}✓ Using default workspace: $MINDX_WORKSPACE${NC}"
                break
                ;;
            2)
                read -p "Enter custom workspace path: " custom_path
                # Expand ~ if present
                MINDX_WORKSPACE="${custom_path/#\~/$HOME}"
                echo -e "${GREEN}✓ Using custom workspace: $MINDX_WORKSPACE${NC}"
                break
                ;;
            *)
                echo -e "${RED}Invalid choice. Please enter 1 or 2.${NC}"
                ;;
        esac
    done
else
    echo -e "${BLUE}Using workspace from .env: $MINDX_WORKSPACE${NC}"
fi

echo -e "${BLUE}Install path: ${MINDX_PATH}${NC}"
echo -e "${BLUE}Workspace: ${MINDX_WORKSPACE}${NC}"
echo ""

# Build binary (source mode only)
echo -e "${YELLOW}[3/9] Preparing binary...${NC}"

if [ "$INSTALL_MODE" = "source" ]; then
    # Build from source
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

    CGO_ENABLED=1 \
        go build \
        -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
        -o bin/mindx \
        ./cmd/main.go

    echo -e "${GREEN}✓ Built mindx binary${NC}"
else
    # Copy from release package
    # Check bin/ first, then root
    if [ -f "bin/mindx" ]; then
        mkdir -p bin
        chmod +x bin/mindx
        echo -e "${GREEN}✓ Found mindx binary in bin/${NC}"
    elif [ -f "mindx" ]; then
        mkdir -p bin
        cp mindx bin/
        chmod +x bin/mindx
        echo -e "${GREEN}✓ Copied mindx binary to bin/${NC}"
    else
        echo -e "${RED}✗ mindx binary not found (looked in bin/ and root)${NC}"
        exit 1
    fi
fi

echo ""

# Install to MINDX_PATH
echo -e "${YELLOW}[4/9] Installing files to ${MINDX_PATH}...${NC}"

mkdir -p "$MINDX_PATH"
mkdir -p "$MINDX_PATH/bin"

# Copy binary
cp bin/mindx "$MINDX_PATH/bin/"
chmod +x "$MINDX_PATH/bin/mindx"

# Also create a symlink for compatibility
ln -sf "bin/mindx" "$MINDX_PATH/mindx"

# Copy skills
if [ -d "skills" ]; then
    mkdir -p "$MINDX_PATH/skills"
    cp -r skills/* "$MINDX_PATH/skills/" 2>/dev/null || true
    echo -e "${GREEN}✓ Copied skills${NC}"
fi

# Copy static files for dashboard (check Vite's dist first, then build for CRA)
if [ -d "dashboard/dist" ]; then
    mkdir -p "$MINDX_PATH/static"
    cp -r dashboard/dist/* "$MINDX_PATH/static/" 2>/dev/null || true
    echo -e "${GREEN}✓ Copied dashboard static files${NC}"
elif [ -d "dashboard/build" ]; then
    mkdir -p "$MINDX_PATH/static"
    cp -r dashboard/build/* "$MINDX_PATH/static/" 2>/dev/null || true
    echo -e "${GREEN}✓ Copied dashboard static files${NC}"
elif [ -d "static" ]; then
    mkdir -p "$MINDX_PATH/static"
    cp -r static/* "$MINDX_PATH/static/" 2>/dev/null || true
    echo -e "${GREEN}✓ Copied static files${NC}"
fi

# Copy config templates
if [ -d "config" ]; then
    mkdir -p "$MINDX_PATH/config"
    
    # Copy and rename to .template
    for file in config/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            cp "$file" "$MINDX_PATH/config/${filename}.template"
        fi
    done
    echo -e "${GREEN}✓ Copied config templates${NC}"
fi

# Copy uninstall script
if [ -f "uninstall.sh" ]; then
    cp uninstall.sh "$MINDX_PATH/"
    chmod +x "$MINDX_PATH/uninstall.sh"
    echo -e "${GREEN}✓ Copied uninstall script${NC}"
fi

echo -e "${GREEN}✓ Installed to ${MINDX_PATH}${NC}"
echo ""

# Create symlink to system path
echo -e "${YELLOW}[5/9] Creating symlink to system path...${NC}"

INSTALL_DIR="/usr/local/bin"

if [ -w "$INSTALL_DIR" ]; then
    ln -sf "$MINDX_PATH/bin/mindx" "$INSTALL_DIR/mindx"
    echo -e "${GREEN}✓ Created symlink $INSTALL_DIR/mindx -> $MINDX_PATH/bin/mindx${NC}"
else
    echo -e "${YELLOW}⚠ Cannot write to $INSTALL_DIR${NC}"
    echo -e "${YELLOW}  Please run: sudo ln -sf $MINDX_PATH/bin/mindx $INSTALL_DIR/mindx${NC}"
    echo -e "${YELLOW}  Or add $MINDX_PATH/bin to your PATH${NC}"
fi

echo ""

# Create workspace directory
echo -e "${YELLOW}[6/9] Creating workspace directory...${NC}"

mkdir -p "$MINDX_WORKSPACE"
mkdir -p "$MINDX_WORKSPACE/config"
mkdir -p "$MINDX_WORKSPACE/logs"
mkdir -p "$MINDX_WORKSPACE/data"
mkdir -p "$MINDX_WORKSPACE/data/memory"
mkdir -p "$MINDX_WORKSPACE/data/sessions"
mkdir -p "$MINDX_WORKSPACE/data/training"
mkdir -p "$MINDX_WORKSPACE/data/vectors"

echo -e "${GREEN}✓ Created workspace: $MINDX_WORKSPACE${NC}"
echo ""

# Copy config templates to workspace
echo -e "${YELLOW}[7/9] Setting up configuration...${NC}"

if [ -d "$MINDX_PATH/config" ]; then
    for template in "$MINDX_PATH/config"/*.template; do
        if [ -f "$template" ]; then
            filename=$(basename "$template" .template)
            dest="$MINDX_WORKSPACE/config/$filename"
            if [ ! -f "$dest" ]; then
                cp "$template" "$dest"
                echo -e "${GREEN}✓ Created config: $filename${NC}"
            else
                echo -e "${BLUE}ℹ Config exists: $filename${NC}"
            fi
        fi
    done
fi

echo ""

# Setup .env file
echo -e "${YELLOW}[8/9] Setting up environment...${NC}"

# Create .env file in workspace if not exists
if [ ! -f "$MINDX_WORKSPACE/.env" ]; then
    cat > "$MINDX_WORKSPACE/.env" << ENV_EOF
# MindX Environment Configuration
MINDX_PATH=${MINDX_PATH}
MINDX_WORKSPACE=${MINDX_WORKSPACE}
ENV_EOF
    echo -e "${GREEN}✓ Created .env file in workspace${NC}"
else
    echo -e "${BLUE}ℹ .env file exists in workspace${NC}"
fi

# Create/update .env file in current directory (both source and release mode)
CURRENT_DIR=$(pwd)
if [ -f ".env" ]; then
    # Update MINDX_PATH and MINDX_WORKSPACE in existing .env
    if [ "$INSTALL_MODE" = "source" ]; then
        sed -i.bak "s|^MINDX_PATH=.*|MINDX_PATH=${PROJECT_ROOT}|" .env 2>/dev/null || sed -i '' "s|^MINDX_PATH=.*|MINDX_PATH=${PROJECT_ROOT}|" .env 2>/dev/null || true
    else
        sed -i.bak "s|^MINDX_PATH=.*|MINDX_PATH=${MINDX_PATH}|" .env 2>/dev/null || sed -i '' "s|^MINDX_PATH=.*|MINDX_PATH=${MINDX_PATH}|" .env 2>/dev/null || true
    fi
    sed -i.bak "s|^MINDX_WORKSPACE=.*|MINDX_WORKSPACE=${MINDX_WORKSPACE}|" .env 2>/dev/null || sed -i '' "s|^MINDX_WORKSPACE=.*|MINDX_WORKSPACE=${MINDX_WORKSPACE}|" .env 2>/dev/null || true
    rm -f .env.bak 2>/dev/null || true
    echo -e "${GREEN}✓ Updated .env file in current directory${NC}"
else
    # Create .env file in current directory
    if [ "$INSTALL_MODE" = "source" ]; then
        cat > ".env" << ENV_EOF
# Environment variables for Bot application
MINDX_WORKSPACE=${MINDX_WORKSPACE}
MINDX_PATH=${PROJECT_ROOT}

ENV_EOF
    else
        cat > ".env" << ENV_EOF
# MindX Environment Configuration
MINDX_PATH=${MINDX_PATH}
MINDX_WORKSPACE=${MINDX_WORKSPACE}
ENV_EOF
    fi
    echo -e "${GREEN}✓ Created .env file in current directory${NC}"
fi

echo ""

# Pull required Ollama models
echo -e "${YELLOW}[9/9] Pulling Ollama models...${NC}"

# Models to pull
REQUIRED_MODELS=(
    "qllama/bge-small-zh-v1.5:latest"
    "qwen3:1.7b"
    "qwen3:0.6b"
)

for model in "${REQUIRED_MODELS[@]}"; do
    echo ""
    echo -e "${BLUE}  Checking ${model}...${NC}"
    
    # Check if model is already installed
    if ollama list | grep -q "^${model//:/\\:}"; then
        echo -e "${GREEN}✓ ${model} is already installed${NC}"
    else
        echo -e "${YELLOW}  Pulling ${model}...${NC}"
        ollama pull "$model" 2>&1 | while IFS= read -r line; do
            echo "    $line"
        done
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ Pulled ${model}${NC}"
        else
            echo -e "${YELLOW}⚠ Failed to pull ${model} (will try later)${NC}"
        fi
    fi
done

echo ""

# Print summary
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Installation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "MindX has been successfully installed!"
echo ""
echo "Install path: $MINDX_PATH"
echo "Workspace:    $MINDX_WORKSPACE"
echo "Binary:       $MINDX_PATH/bin/mindx"
echo "Symlink:      /usr/local/bin/mindx (pointing to $MINDX_PATH/mindx)"
echo ""
echo -e "${YELLOW}Recommended: Set global environment variables${NC}"
echo "Add the following lines to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
echo ""
echo "  export MINDX_PATH=$MINDX_PATH"
echo "  export MINDX_WORKSPACE=$MINDX_WORKSPACE"
echo ""
echo "Then reload your shell profile:"
echo "  source ~/.zshrc  # or ~/.bashrc"
echo ""
echo "Quick start:"
echo "  1. Start MindX service: mindx start"
echo "  2. Open Dashboard WebUI: mindx dashboard"
echo "  3. Visit: http://localhost:911"
echo ""
echo "To uninstall:"
echo "  $MINDX_PATH/uninstall.sh"
echo ""
echo "Or run from source directory:"
echo "  ./uninstall.sh"
echo ""
