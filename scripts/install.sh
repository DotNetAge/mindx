#!/bin/bash

# MindX Installation Script (macOS / Linux)
# This script ONLY installs pre-built binaries. Run build.sh first.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Darwin)  PLATFORM="macos" ;;
    Linux)   PLATFORM="linux" ;;
    *)       echo -e "${RED}✗ Unsupported OS: $OS${NC}"; exit 1 ;;
esac

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  MindX Installation Script${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}Platform: ${PLATFORM}${NC}"

# Read version
if [ -f "VERSION" ]; then
    VERSION=$(cat VERSION | tr -d '[:space:]')
else
    VERSION="dev"
fi
echo -e "${BLUE}Version: ${VERSION}${NC}"
echo ""

# Check binary exists
echo -e "${YELLOW}[1/7] Checking binary...${NC}"
if [ ! -f "bin/mindx" ]; then
    echo -e "${RED}✗ Binary not found: bin/mindx${NC}"
    echo -e "${YELLOW}  Please run build.sh first:${NC}"
    echo "    ./scripts/build.sh"
    exit 1
fi
echo -e "${GREEN}✓ Found bin/mindx${NC}"
echo ""

# Load environment variables
echo -e "${YELLOW}[2/7] Loading configuration...${NC}"

if [ -f ".env" ]; then
    source .env
    echo -e "${GREEN}✓ Loaded .env file${NC}"
fi

MINDX_PATH="${MINDX_PATH:-/usr/local/mindx}"
MINDX_WORKSPACE="${MINDX_WORKSPACE:-}"

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
                MINDX_WORKSPACE="${custom_path/#\~/$HOME}"
                echo -e "${GREEN}✓ Using custom workspace: $MINDX_WORKSPACE${NC}"
                break
                ;;
            *)
                echo -e "${RED}Invalid choice. Please enter 1 or 2.${NC}"
                ;;
        esac
    done
fi

echo -e "${BLUE}Install path: ${MINDX_PATH}${NC}"
echo -e "${BLUE}Workspace: ${MINDX_WORKSPACE}${NC}"
echo ""

# Install files
echo -e "${YELLOW}[3/7] Installing files to ${MINDX_PATH}...${NC}"

mkdir -p "$MINDX_PATH/bin"

# Copy binary
cp bin/mindx "$MINDX_PATH/bin/"
chmod +x "$MINDX_PATH/bin/mindx"
echo -e "${GREEN}✓ Copied binary${NC}"

# Copy skills
if [ -d "skills" ]; then
    mkdir -p "$MINDX_PATH/skills"
    cp -r skills/* "$MINDX_PATH/skills/" 2>/dev/null || true
    echo -e "${GREEN}✓ Copied skills${NC}"
fi

# Copy static files
if [ -d "dashboard/dist" ]; then
    mkdir -p "$MINDX_PATH/static"
    cp -r dashboard/dist/* "$MINDX_PATH/static/" 2>/dev/null || true
    echo -e "${GREEN}✓ Copied dashboard${NC}"
elif [ -d "static" ]; then
    mkdir -p "$MINDX_PATH/static"
    cp -r static/* "$MINDX_PATH/static/" 2>/dev/null || true
    echo -e "${GREEN}✓ Copied static files${NC}"
fi

# Copy config templates
if [ -d "config" ]; then
    mkdir -p "$MINDX_PATH/config"
    for file in config/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            cp "$file" "$MINDX_PATH/config/${filename}.template"
        fi
    done
    echo -e "${GREEN}✓ Copied config templates${NC}"
fi

# Copy uninstall script
if [ -f "scripts/uninstall.sh" ]; then
    cp scripts/uninstall.sh "$MINDX_PATH/"
    chmod +x "$MINDX_PATH/uninstall.sh"
    echo -e "${GREEN}✓ Copied uninstall script${NC}"
fi

echo ""

# Create symlink
echo -e "${YELLOW}[4/7] Creating symlink...${NC}"

INSTALL_DIR="/usr/local/bin"

if [ -w "$INSTALL_DIR" ]; then
    ln -sf "$MINDX_PATH/bin/mindx" "$INSTALL_DIR/mindx"
    echo -e "${GREEN}✓ Created symlink: $INSTALL_DIR/mindx${NC}"
else
    echo -e "${YELLOW}⚠ Cannot write to $INSTALL_DIR (need sudo)${NC}"
    echo -e "${YELLOW}  Run: sudo ln -sf $MINDX_PATH/bin/mindx $INSTALL_DIR/mindx${NC}"
fi

echo ""

# Create workspace
echo -e "${YELLOW}[5/7] Creating workspace...${NC}"

mkdir -p "$MINDX_WORKSPACE"
mkdir -p "$MINDX_WORKSPACE/config"
mkdir -p "$MINDX_WORKSPACE/logs"
mkdir -p "$MINDX_WORKSPACE/data/memory"
mkdir -p "$MINDX_WORKSPACE/data/sessions"
mkdir -p "$MINDX_WORKSPACE/data/training"
mkdir -p "$MINDX_WORKSPACE/data/vectors"

echo -e "${GREEN}✓ Created workspace: $MINDX_WORKSPACE${NC}"
echo ""

# Setup config files
echo -e "${YELLOW}[6/7] Setting up configuration...${NC}"

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

# Create .env
if [ ! -f "$MINDX_WORKSPACE/.env" ]; then
    cat > "$MINDX_WORKSPACE/.env" << ENV_EOF
# MindX Environment Configuration
MINDX_PATH=${MINDX_PATH}
MINDX_WORKSPACE=${MINDX_WORKSPACE}
ENV_EOF
    echo -e "${GREEN}✓ Created .env in workspace${NC}"
fi

echo ""

# Setup daemon
echo -e "${YELLOW}[7/7] Setting up daemon...${NC}"

CURRENT_USER=$(whoami)

if [ "$PLATFORM" = "macos" ]; then
    # macOS: launchd
    LAUNCHD_PLIST="$HOME/Library/LaunchAgents/com.mindx.agent.plist"
    
    mkdir -p "$(dirname "$LAUNCHD_PLIST")"
    
    cat > "$LAUNCHD_PLIST" << PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mindx.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>${MINDX_PATH}/bin/mindx</string>
        <string>kernel</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${MINDX_WORKSPACE}/logs/mindx.log</string>
    <key>StandardErrorPath</key>
    <string>${MINDX_WORKSPACE}/logs/mindx.error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>MINDX_PATH</key>
        <string>${MINDX_PATH}</string>
        <key>MINDX_WORKSPACE</key>
        <string>${MINDX_WORKSPACE}</string>
    </dict>
</dict>
</plist>
PLIST_EOF

    echo -e "${GREEN}✓ Created launchd plist${NC}"
    echo -e "${BLUE}  Location: $LAUNCHD_PLIST${NC}"
    echo ""
    echo -e "${YELLOW}To start the service:${NC}"
    echo "    launchctl load $LAUNCHD_PLIST"
    echo ""
    echo -e "${YELLOW}To stop the service:${NC}"
    echo "    launchctl unload $LAUNCHD_PLIST"

else
    # Linux: systemd
    SYSTEMD_SERVICE="/etc/systemd/system/mindx.service"
    
    SERVICE_CONTENT="[Unit]
Description=MindX AI Personal Assistant
After=network.target ollama.service
Wants=ollama.service

[Service]
Type=simple
User=${CURRENT_USER}
Group=$(id -gn)
ExecStart=${MINDX_PATH}/bin/mindx kernel run
WorkingDirectory=${MINDX_WORKSPACE}
Environment=MINDX_WORKSPACE=${MINDX_WORKSPACE}
Environment=MINDX_PATH=${MINDX_PATH}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target"

    if [ -w "/etc/systemd/system" ] || [ "$(id -u)" -eq 0 ]; then
        echo "$SERVICE_CONTENT" > "$SYSTEMD_SERVICE"
        systemctl daemon-reload
        systemctl enable mindx
        echo -e "${GREEN}✓ Created and enabled systemd service${NC}"
        echo ""
        echo -e "${YELLOW}To start the service:${NC}"
        echo "    sudo systemctl start mindx"
    else
        echo -e "${YELLOW}⚠ Cannot write to /etc/systemd/system (need sudo)${NC}"
        echo ""
        echo "  sudo tee $SYSTEMD_SERVICE << 'EOF'"
        echo "$SERVICE_CONTENT"
        echo "EOF"
        echo "  sudo systemctl daemon-reload"
        echo "  sudo systemctl enable mindx"
    fi
fi

echo ""

# Print summary
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Installation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Install path: $MINDX_PATH"
echo "Workspace:    $MINDX_WORKSPACE"
echo "Binary:       $MINDX_PATH/bin/mindx"
echo ""
echo -e "${YELLOW}Add to your shell profile (~/.zshrc or ~/.bashrc):${NC}"
echo "  export MINDX_PATH=$MINDX_PATH"
echo "  export MINDX_WORKSPACE=$MINDX_WORKSPACE"
echo ""
echo "Quick start:"
echo "  mindx kernel run"
echo ""
echo "To uninstall:"
echo "  $MINDX_PATH/uninstall.sh"
echo ""
