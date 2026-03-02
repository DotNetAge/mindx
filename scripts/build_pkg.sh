#!/bin/bash

# MindX PKG Builder for macOS
# Usage: ./scripts/build_pkg.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$PROJECT_ROOT/dist"
PKG_WORK="$PROJECT_ROOT/.tmp/pkg_build"
PKG_OUTPUT="$PROJECT_ROOT/releases"

echo "=== MindX PKG Builder ==="
echo ""

# Clean and create work directory
rm -rf "$PKG_WORK"
mkdir -p "$PKG_WORK/root/usr/local/mindx/bin"
mkdir -p "$PKG_WORK/scripts"
mkdir -p "$PKG_WORK/root/Library/LaunchAgents"

# Get version first (VERSION_TAG for directory names, VERSION for package)
VERSION="1.0.0"
VERSION_TAG="v1.0.0"
if [ -f "$PROJECT_ROOT/VERSION" ]; then
    VERSION=$(cat "$PROJECT_ROOT/VERSION" | tr -d '[:space:]' | sed 's/^v//')
    VERSION_TAG=$(cat "$PROJECT_ROOT/VERSION" | tr -d '[:space:]')
fi

# Check binary exists
if [ ! -f "$DIST_DIR/mindx-${VERSION_TAG}-darwin-arm64/bin/mindx" ]; then
    echo "Error: dist/ directory not found. Run build.sh first."
    echo "Expected: $DIST_DIR/mindx-${VERSION_TAG}-darwin-arm64/bin/mindx"
    exit 1
fi

# Get current user
CURRENT_USER=$(whoami)

echo "[1/5] Copying binary..."
DIST_SRC="$DIST_DIR/mindx-${VERSION_TAG}-darwin-arm64"
cp "$DIST_SRC/bin/mindx" "$PKG_WORK/root/usr/local/mindx/bin/"
chmod +x "$PKG_WORK/root/usr/local/mindx/bin/mindx"

# Copy scripts
mkdir -p "$PKG_WORK/root/usr/local/mindx/scripts"
cp "$PROJECT_ROOT/scripts/ollama.sh" "$PKG_WORK/root/usr/local/mindx/scripts/" 2>/dev/null || true

echo "[2/5] Copying skills..."
if [ -d "$DIST_SRC/skills" ]; then
    cp -r "$DIST_SRC/skills" "$PKG_WORK/root/usr/local/mindx/"
fi

echo "[3/5] Copying static files..."
if [ -d "$DIST_SRC/static" ]; then
    cp -r "$DIST_SRC/static" "$PKG_WORK/root/usr/local/mindx/"
fi

# Copy config templates
if [ -d "$DIST_SRC/config" ]; then
    cp -r "$DIST_SRC/config" "$PKG_WORK/root/usr/local/mindx/"
fi

# Create postinstall script
cat > "$PKG_WORK/scripts/postinstall" << POSTEOF
#!/bin/bash

TARGET_USER="$CURRENT_USER"
HOME_DIR=$(eval echo "~$TARGET_USER")
INSTALL_PATH="/usr/local/mindx"
WORKSPACE_PATH="$HOME_DIR/.mindx"
LAUNCHD_PLIST="/Library/LaunchAgents/com.mindx.agent.plist"

echo "Setting up MindX for user: $TARGET_USER"

# Check and install Ollama if needed
echo "Checking Ollama..."
if ! command -v ollama &> /dev/null; then
    echo "Installing Ollama (this may take a minute)..."
    /usr/local/mindx/scripts/ollama.sh
    echo "Ollama installed!"
else
    echo "Ollama already installed."
fi

# Create symlink
mkdir -p /usr/local/bin
rm -f /usr/local/bin/mindx
ln -sf "$INSTALL_PATH/bin/mindx" /usr/local/bin/mindx

# Create workspace
mkdir -p "$WORKSPACE_PATH"
mkdir -p "$WORKSPACE_PATH/config"
mkdir -p "$WORKSPACE_PATH/logs"
mkdir -p "$WORKSPACE_PATH/data/memory"
mkdir -p "$WORKSPACE_PATH/data/sessions"
mkdir -p "$WORKSPACE_PATH/data/training"
mkdir -p "$WORKSPACE_PATH/data/vectors"

# Create .env
if [ ! -f "$WORKSPACE_PATH/.env" ]; then
    cat > "$WORKSPACE_PATH/.env" << 'ENV_EOF'
MINDX_PATH=/usr/local/mindx
MINDX_WORKSPACE=~/.mindx
ENV_EOF
fi

chmod +x "$INSTALL_PATH/bin/mindx"

# Setup MindX daemon
mkdir -p "$(dirname "$LAUNCHD_PLIST")"

cat > "$LAUNCHD_PLIST" << 'PLIST_EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mindx.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/mindx/bin/mindx</string>
        <string>kernel</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>~/.mindx/logs/mindx.log</string>
    <key>StandardErrorPath</key>
    <string>~/.mindx/logs/mindx.error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>MINDX_PATH</key>
        <string>/usr/local/mindx</string>
        <key>MINDX_WORKSPACE</key>
        <string>~/.mindx</string>
    </dict>
    <key>UserName</key>
    <string>USERNAME</string>
</dict>
</plist>
PLIST_EOF

sed -i '' "s/USERNAME/$TARGET_USER/" "$LAUNCHD_PLIST"
chmod 644 "$LAUNCHD_PLIST"

# Load MindX daemon
launchctl load "$LAUNCHD_PLIST"

echo ""
echo "=========================================="
echo "  MindX installed successfully!"
echo "=========================================="
echo ""
echo "MindX will auto-start on login."
echo "Dashboard: http://localhost:911"
POSTEOF
chmod +x "$PKG_WORK/scripts/postinstall"

echo "[4/5] Building PKG..."

mkdir -p "$PKG_OUTPUT"
OUTPUT_PKG="$PKG_OUTPUT/MindX-${VERSION}.pkg"

pkgbuild \
    --root "$PKG_WORK/root" \
    --scripts "$PKG_WORK/scripts" \
    --identifier "com.mindx.brain" \
    --version "$VERSION" \
    --install-location "/" \
    "$OUTPUT_PKG"

# Clean up work directory
rm -rf "$PKG_WORK"

echo "[5/5] Done!"
echo ""
echo "=== Build Complete ==="
echo "Output: $OUTPUT_PKG"
echo ""
echo "To install:"
echo "  sudo installer -pkg \"$OUTPUT_PKG\" -target /"
echo ""
echo "Done! MindX will auto-start after installation."
