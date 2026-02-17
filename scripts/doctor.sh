#!/bin/bash

# MindX Environment Doctor - 检查运行环境与已安装环境

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

# Counters
PASSED=0
WARNINGS=0
ERRORS=0
ISSUES=()

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  MindX Environment Doctor${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# ============================================
# 1. Check system dependencies
# ============================================
echo -e "${YELLOW}[1/8] Checking system dependencies...${NC}"
echo ""

# Check Go
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "${GREEN}✓ Go ${GO_VERSION}${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ Go is not installed${NC}"
    ISSUES+=("Go is not installed. Please install Go 1.21 or later from https://golang.org/dl/")
    ((ERRORS++))
fi

# Check Node.js (optional)
if command -v node &> /dev/null; then
    NODE_VERSION=$(node -v)
    echo -e "${GREEN}✓ Node.js ${NODE_VERSION}${NC}"
    ((PASSED++))
else
    echo -e "${YELLOW}⚠ Node.js is not installed (optional, only needed for building dashboard)${NC}"
    ISSUES+=("Node.js is not installed. You won't be able to build the dashboard from source.")
    ((WARNINGS++))
fi

# Check Ollama (required)
if command -v ollama &> /dev/null; then
    OLLAMA_VERSION=$(ollama --version 2>/dev/null || echo "installed")
    echo -e "${GREEN}✓ Ollama ${OLLAMA_VERSION}${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ Ollama is not installed${NC}"
    ISSUES+=("Ollama is not installed. Please install Ollama from https://ollama.com or run: curl -fsSL https://ollama.com/install.sh | sh")
    ((ERRORS++))
fi

echo ""

# ============================================
# 2. Check Ollama models
# ============================================
echo -e "${YELLOW}[2/8] Checking Ollama models...${NC}"
echo ""

REQUIRED_MODELS=(
    "qllama/bge-small-zh-v1.5:latest"
    "qwen3:1.7b"
    "qwen3:0.6b"
)

for model in "${REQUIRED_MODELS[@]}"; do
    if ollama list 2>/dev/null | grep -q "^${model//:/\\:}"; then
        echo -e "${GREEN}✓ ${model}${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ ${model}${NC}"
        ISSUES+=("Model ${model} is missing. Please run: ollama pull ${model}")
        ((ERRORS++))
    fi
done

echo ""

# ============================================
# 3. Check installation
# ============================================
echo -e "${YELLOW}[3/8] Checking MindX installation...${NC}"
echo ""

# Check if mindx is in PATH
if command -v mindx &> /dev/null; then
    echo -e "${GREEN}✓ mindx is in PATH${NC}"
    ((PASSED++))
    
    # Check version
    if mindx --version &> /dev/null; then
        echo -e "${GREEN}✓ mindx --version works${NC}"
        ((PASSED++))
    else
        echo -e "${YELLOW}⚠ mindx --version failed${NC}"
        ((WARNINGS++))
    fi
else
    echo -e "${RED}✗ mindx is not in PATH${NC}"
    ISSUES+=("mindx is not in PATH. Please run: make install")
    ((ERRORS++))
fi

# Check install path
MINDX_INSTALL_PATH="${MINDX_PATH:-/usr/local/mindx}"
if [ -d "$MINDX_INSTALL_PATH" ]; then
    echo -e "${GREEN}✓ Install directory exists: $MINDX_INSTALL_PATH${NC}"
    ((PASSED++))
    
    # Check binary
    if [ -f "$MINDX_INSTALL_PATH/bin/mindx" ]; then
        echo -e "${GREEN}✓ Binary exists: $MINDX_INSTALL_PATH/bin/mindx${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ Binary missing: $MINDX_INSTALL_PATH/bin/mindx${NC}"
        ISSUES+=("Binary missing. Please run: make build && make install")
        ((ERRORS++))
    fi
    
    # Check static files
    if [ -d "$MINDX_INSTALL_PATH/static" ]; then
        echo -e "${GREEN}✓ Static files exist: $MINDX_INSTALL_PATH/static${NC}"
        ((PASSED++))
    else
        echo -e "${YELLOW}⚠ Static files missing: $MINDX_INSTALL_PATH/static${NC}"
        ISSUES+=("Static files missing. Dashboard may not work. Please run: make build")
        ((WARNINGS++))
    fi
else
    echo -e "${RED}✗ Install directory missing: $MINDX_INSTALL_PATH${NC}"
    ISSUES+=("Install directory missing. Please run: make install")
    ((ERRORS++))
fi

echo ""

# ============================================
# 4. Check workspace
# ============================================
echo -e "${YELLOW}[4/8] Checking workspace...${NC}"
echo ""

MINDX_WORKSPACE="${MINDX_WORKSPACE:-$HOME/.mindx}"
if [ -d "$MINDX_WORKSPACE" ]; then
    echo -e "${GREEN}✓ Workspace exists: $MINDX_WORKSPACE${NC}"
    ((PASSED++))
    
    # Check config directory
    if [ -d "$MINDX_WORKSPACE/config" ]; then
        echo -e "${GREEN}✓ Config directory exists${NC}"
        ((PASSED++))
        
        # Check config files
        CONFIG_FILES=("server.yml" "models.json" "capabilities.json" "channels.json")
        for cfg in "${CONFIG_FILES[@]}"; do
            if [ -f "$MINDX_WORKSPACE/config/$cfg" ]; then
                echo -e "${GREEN}✓ Config exists: $cfg${NC}"
                ((PASSED++))
            else
                echo -e "${YELLOW}⚠ Config missing: $cfg${NC}"
                ISSUES+=("Config file missing: $cfg. It will be created on first run.")
                ((WARNINGS++))
            fi
        done
    else
        echo -e "${RED}✗ Config directory missing${NC}"
        ISSUES+=("Config directory missing. Please run: make install")
        ((ERRORS++))
    fi
    
    # Check data directories
    DATA_DIRS=("data/memory" "data/sessions" "data/vectors" "data/training" "logs")
    for dir in "${DATA_DIRS[@]}"; do
        if [ -d "$MINDX_WORKSPACE/$dir" ]; then
            echo -e "${GREEN}✓ Directory exists: $dir${NC}"
            ((PASSED++))
        else
            echo -e "${YELLOW}⚠ Directory missing: $dir${NC}"
            ISSUES+=("Directory missing: $dir. It will be created on first run.")
            ((WARNINGS++))
        fi
    done
else
    echo -e "${RED}✗ Workspace missing: $MINDX_WORKSPACE${NC}"
    ISSUES+=("Workspace missing. Please run: make install")
    ((ERRORS++))
fi

echo ""

# ============================================
# 5. Check permissions
# ============================================
echo -e "${YELLOW}[5/8] Checking permissions...${NC}"
echo ""

if [ -d "$MINDX_WORKSPACE" ]; then
    if [ -w "$MINDX_WORKSPACE" ]; then
        echo -e "${GREEN}✓ Workspace is writable${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ Workspace is not writable${NC}"
        ISSUES+=("Workspace is not writable. Please check permissions: chmod -R 755 $MINDX_WORKSPACE")
        ((ERRORS++))
    fi
fi

echo ""

# ============================================
# 6. Check ports
# ============================================
echo -e "${YELLOW}[6/8] Checking ports...${NC}"
echo ""

PORTS=(911 1314)
for port in "${PORTS[@]}"; do
    if lsof -Pi :$port -sTCP:LISTEN -t 2>/dev/null | grep -q .; then
        echo -e "${YELLOW}⚠ Port $port is in use${NC}"
        ISSUES+=("Port $port is in use. You may need to stop other processes or change the port in config.")
        ((WARNINGS++))
    else
        echo -e "${GREEN}✓ Port $port is available${NC}"
        ((PASSED++))
    fi
done

echo ""

# ============================================
# 7. Check source directory (if applicable)
# ============================================
echo -e "${YELLOW}[7/8] Checking source directory...${NC}"
echo ""

if [ -f "cmd/main.go" ]; then
    echo -e "${GREEN}✓ Source directory detected${NC}"
    ((PASSED++))
    
    # Check go.mod
    if [ -f "go.mod" ]; then
        echo -e "${GREEN}✓ go.mod exists${NC}"
        ((PASSED++))
    fi
    
    # Check dashboard
    if [ -d "dashboard" ]; then
        echo -e "${GREEN}✓ Dashboard directory exists${NC}"
        ((PASSED++))
        
        if [ -f "dashboard/package.json" ]; then
            echo -e "${GREEN}✓ dashboard/package.json exists${NC}"
            ((PASSED++))
        fi
    fi
else
    echo -e "${BLUE}ℹ Not in source directory${NC}"
fi

echo ""

# ============================================
# 8. Summary
# ============================================
echo -e "${YELLOW}[8/8] Summary...${NC}"
echo ""

echo -e "${GREEN}✓ Passed: ${PASSED}${NC}"
echo -e "${YELLOW}⚠ Warnings: ${WARNINGS}${NC}"
echo -e "${RED}✗ Errors: ${ERRORS}${NC}"
echo ""

if [ ${#ISSUES[@]} -gt 0 ]; then
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}  Issues Found${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    
    for i in "${!ISSUES[@]}"; do
        echo -e "$((i+1)). ${ISSUES[$i]}"
        echo ""
    done
    
    echo -e "${YELLOW}========================================${NC}"
    echo ""
fi

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  All checks passed! 🎉${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "You're ready to go! Try:"
    echo "  make run"
    echo "  mindx dashboard"
    echo ""
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}  Some warnings, but usable${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
else
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}  Please fix the errors above${NC}"
    echo -e "${RED}========================================${NC}"
    echo ""
fi

# Exit with error code if there are errors
exit $ERRORS
