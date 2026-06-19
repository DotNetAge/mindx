#!/bin/bash
# Pre-remove script for .deb/.rpm packages
set -e

# Stop and disable MindX daemon
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop mindx-daemon >/dev/null 2>&1 || true
    systemctl disable mindx-daemon >/dev/null 2>&1 || true
fi

echo "Removing MindX..."
echo "Note: User data (~/.mindx/) is preserved."
