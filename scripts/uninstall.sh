#!/bin/bash

# MindX Uninstallation Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${RED}========================================${NC}"
echo -e "${RED}  MindX Uninstallation Script${NC}"
echo -e "${RED}========================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Load environment variables
if [ -f ".env" ]; then
    source .env
    MINDX_INSTALL_PATH="${MINDX_INSTALL_PATH:-/usr/local/mindx}"
    MINDX_WORKSPACE="${MINDX_WORKSPACE:-~/.mindx}"
else
    MINDX_INSTALL_PATH="${MINDX_INSTALL_PATH:-/usr/local/mindx}"
    MINDX_WORKSPACE="${MINDX_WORKSPACE:-~/.mindx}"
fi

# Check if we're running from install directory
if [ -f "bin/mindx" ] || [ -f "skills" ]; then
    echo -e "${BLUE}Running from install directory${NC}"
    if [ -f ".env" ]; then
        source .env
    fi
elif [ -f "cmd/main.go" ]; then
    echo -e "${BLUE}Running from source directory${NC}"
    # Check .env in workspace for source mode
    if [ -f "$MINDX_WORKSPACE/.env" ]; then
        source "$MINDX_WORKSPACE/.env"
    fi
fi

echo -e "${BLUE}Install path: ${MINDX_INSTALL_PATH}${NC}"
echo -e "${BLUE}Workspace: ${MINDX_WORKSPACE}${NC}"
echo ""

# Confirm uninstallation
echo -e "${YELLOW}This will uninstall MindX from your system.${NC}"
echo -e "${YELLOW}The following will be removed:${NC}"
echo "  - Symlink from /usr/local/bin/mindx"
echo "  - MindX installation from ${MINDX_INSTALL_PATH}"
echo "  - System services (if installed)"
echo "  - Workspace directory (optional)"
echo ""
read -p "Do you want to continue? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Uninstallation cancelled${NC}"
    exit 0
fi

echo ""

# Stop running services
echo -e "${YELLOW}[1/7] Stopping running services...${NC}"

# Stop dashboard
if pgrep -f "mindx dashboard" > /dev/null 2>&1; then
    echo "  Stopping dashboard service..."
    pkill -f "mindx dashboard" 2>/dev/null || true
    echo -e "${GREEN}✓ Stopped dashboard${NC}"
else
    echo -e "${BLUE}ℹ Dashboard not running${NC}"
fi

# Stop training daemon
if pgrep -f "mindx train" > /dev/null 2>&1; then
    echo "  Stopping training daemon..."
    pkill -f "mindx train" 2>/dev/null || true
    echo -e "${GREEN}✓ Stopped training daemon${NC}"
else
    echo -e "${BLUE}ℹ Training daemon not running${NC}"
fi

# Stop kernel service
if pgrep -f "mindx kernel" > /dev/null 2>&1; then
    echo "  Stopping kernel service..."
    pkill -f "mindx kernel" 2>/dev/null || true
    echo -e "${GREEN}✓ Stopped kernel service${NC}"
else
    echo -e "${BLUE}ℹ Kernel service not running${NC}"
fi

echo ""

# Remove symlink from system path
echo -e "${YELLOW}[2/7] Removing symlink from system path...${NC}"

SYMLINK_PATH="/usr/local/bin/mindx"
if [ -L "$SYMLINK_PATH" ]; then
    if [ -w "$(dirname "$SYMLINK_PATH")" ]; then
        rm -f "$SYMLINK_PATH"
        echo -e "${GREEN}✓ Removed symlink $SYMLINK_PATH${NC}"
    else
        sudo rm -f "$SYMLINK_PATH"
        echo -e "${GREEN}✓ Removed symlink $SYMLINK_PATH (with sudo)${NC}"
    fi
elif [ -f "$SYMLINK_PATH" ]; then
    echo -e "${YELLOW}⚠ $SYMLINK_PATH exists but is not a symlink, removing anyway${NC}"
    if [ -w "$(dirname "$SYMLINK_PATH")" ]; then
        rm -f "$SYMLINK_PATH"
    else
        sudo rm -f "$SYMLINK_PATH"
    fi
    echo -e "${GREEN}✓ Removed $SYMLINK_PATH${NC}"
else
    echo -e "${BLUE}ℹ $SYMLINK_PATH not found${NC}"
fi

echo ""

# Remove system services
echo -e "${YELLOW}[3/7] Removing system services...${NC}"

# Detect OS and remove services accordingly
case "$(uname -s)" in
    Darwin)
        # macOS
        echo "  Checking for macOS launchd services..."
        
        # Check for plist files
        PLIST_PATH="$HOME/Library/LaunchAgents/com.mindx.service.plist"
        if [ -f "$PLIST_PATH" ]; then
            echo "  Unloading launchd service..."
            launchctl unload "$PLIST_PATH" 2>/dev/null || true
            rm -f "$PLIST_PATH"
            echo -e "${GREEN}✓ Removed launchd service${NC}"
        else
            echo -e "${BLUE}ℹ No launchd service found${NC}"
        fi
        ;;
        
    Linux)
        # Linux
        echo "  Checking for systemd services..."
        
        if [ -f "/etc/systemd/system/mindx.service" ]; then
            echo "  Stopping systemd service..."
            sudo systemctl stop mindx 2>/dev/null || true
            sudo systemctl disable mindx 2>/dev/null || true
            sudo rm -f /etc/systemd/system/mindx.service
            sudo systemctl daemon-reload 2>/dev/null || true
            echo -e "${GREEN}✓ Removed systemd service${NC}"
        else
            echo -e "${BLUE}ℹ No systemd service found${NC}"
        fi
        ;;
        
    *)
        echo -e "${BLUE}ℹ Unknown OS, skipping service removal${NC}"
        ;;
esac

echo ""

# Remove installation directory
echo -e "${YELLOW}[4/7] Removing installation directory...${NC}"

if [ -d "$MINDX_INSTALL_PATH" ]; then
    echo "  Removing $MINDX_INSTALL_PATH..."
    if [ -w "$MINDX_INSTALL_PATH" ]; then
        rm -rf "$MINDX_INSTALL_PATH"
        echo -e "${GREEN}✓ Removed installation directory${NC}"
    else
        sudo rm -rf "$MINDX_INSTALL_PATH"
        echo -e "${GREEN}✓ Removed installation directory (with sudo)${NC}"
    fi
else
    echo -e "${BLUE}ℹ Installation directory not found${NC}"
fi

echo ""

# Ask about workspace removal
echo -e "${YELLOW}[5/7] Workspace directory${NC}"
echo "  Workspace: $MINDX_WORKSPACE"
echo ""
read -p "Do you want to remove the workspace directory? (y/n): " -n 1 -r
echo
REMOVE_WORKSPACE=false
if [[ $REPLY =~ ^[Yy]$ ]]; then
    REMOVE_WORKSPACE=true
    echo -e "${YELLOW}  Workspace will be removed${NC}"
else
    echo -e "${BLUE}ℹ Workspace will be preserved${NC}"
fi

echo ""

# Remove workspace if requested
if [ "$REMOVE_WORKSPACE" = true ]; then
    echo -e "${YELLOW}[6/7] Removing workspace directory...${NC}"
    
    if [ -d "$MINDX_WORKSPACE" ]; then
        echo "  Removing $MINDX_WORKSPACE..."
        rm -rf "$MINDX_WORKSPACE"
        echo -e "${GREEN}✓ Removed workspace${NC}"
    else
        echo -e "${BLUE}ℹ Workspace directory not found${NC}"
    fi
else
    echo -e "${YELLOW}[6/7] Preserving workspace directory...${NC}"
    echo -e "${BLUE}ℹ Workspace preserved: $MINDX_WORKSPACE${NC}"
fi

echo ""

# Clean up temporary files
echo -e "${YELLOW}[7/7] Cleaning up temporary files...${NC}"

# Remove log files
if [ -f "/tmp/mindx-dashboard.log" ]; then
    rm -f "/tmp/mindx-dashboard.log"
    echo -e "${GREEN}✓ Removed /tmp/mindx-dashboard.log${NC}"
fi

if [ -f "/tmp/mindx-train.log" ]; then
    rm -f "/tmp/mindx-train.log"
    echo -e "${GREEN}✓ Removed /tmp/mindx-train.log${NC}"
fi

if [ -f "/tmp/mindx-dashboard.pid" ]; then
    rm -f "/tmp/mindx-dashboard.pid"
    echo -e "${GREEN}✓ Removed /tmp/mindx-dashboard.pid${NC}"
fi

echo ""

# Print summary
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Uninstallation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Removed:"
echo "  ✓ Symlink from /usr/local/bin/mindx"
echo "  ✓ Installation directory"
echo "  ✓ System services"
echo "  ✓ Temporary files"
if [ "$REMOVE_WORKSPACE" = true ]; then
    echo "  ✓ Workspace directory"
else
    echo "  ℹ Workspace directory preserved: $MINDX_WORKSPACE"
fi
echo ""
echo "To reinstall MindX:"
echo "  1. Run: ./install.sh"
echo "  2. Or: make install"
echo ""
