#!/bin/bash
# MindX APT/YUM Repository Installer
# Usage: curl -fsSL https://dotnetage.github.io/mindx/install.sh | sudo bash
# Or:    curl -fsSL https://dotnetage.github.io/mindx/install.sh | bash

set -eu

REPO_BASE="https://dotnetage.github.io/mindx"
GPG_KEY_URL="${REPO_BASE}/KEY.gpg"

# ──────────────────────────────────────────────
# 1. Detect OS
# ──────────────────────────────────────────────
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS_ID="${ID}"
    OS_LIKE="${ID_LIKE:-}"
else
    echo "Cannot detect OS. Please install manually."
    exit 1
fi

# ──────────────────────────────────────────────
# 2. Install via APT (Debian/Ubuntu)
# ──────────────────────────────────────────────
install_apt() {
    echo "📦 Installing MindX via APT..."

    # Install prerequisites
    sudo apt-get update -qq
    sudo apt-get install -y -qq curl gnupg

    # Add GPG key
    curl -fsSL "${GPG_KEY_URL}" | sudo gpg --dearmor -o /usr/share/keyrings/mindx.gpg

    # Add repository
    echo "deb [signed-by=/usr/share/keyrings/mindx.gpg] ${REPO_BASE}/apt stable main" \
        | sudo tee /etc/apt/sources.list.d/mindx.list > /dev/null

    # Install
    sudo apt-get update -qq
    sudo apt-get install -y mindx

    echo ""
    echo "✅ MindX installed! Run 'mindx doctor' to verify."
}

# ──────────────────────────────────────────────
# 3. Install via YUM/DNF (Fedora/RHEL)
# ──────────────────────────────────────────────
install_yum() {
    echo "📦 Installing MindX via YUM/DNF..."

    # Add repository
    cat <<EOF | sudo tee /etc/yum.repos.d/mindx.repo > /dev/null
[mindx]
name=MindX Repository
baseurl=${REPO_BASE}/rpm
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=${GPG_KEY_URL}
EOF

    # Install
    if command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y mindx
    else
        sudo yum install -y mindx
    fi

    echo ""
    echo "✅ MindX installed! Run 'mindx doctor' to verify."
}

# ──────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────
case "${OS_ID}" in
    debian|ubuntu|linuxmint|pop|elementary|kali)
        install_apt
        ;;
    fedora|rhel|centos|rocky|almalinux)
        install_yum
        ;;
    *)
        case "${OS_LIKE}" in
            *debian*) install_apt ;;
            *fedora*) install_yum ;;
            *)
                echo "Unsupported OS: ${OS_ID} (${OS_LIKE})"
                echo "Please install via Snap, Flatpak, or manual binary:"
                echo "  https://github.com/DotNetAge/mindx/releases"
                exit 1
                ;;
        esac
        ;;
esac
