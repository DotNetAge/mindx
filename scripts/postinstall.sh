#!/bin/bash
# Post-install script for .deb/.rpm packages
set -e

# Enable and start MindX daemon via systemd
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl enable mindx-daemon >/dev/null 2>&1 || true
    systemctl start mindx-daemon >/dev/null 2>&1 || echo "  ⚠  mindx-daemon service not started (run 'sudo systemctl start mindx-daemon' manually)"
fi

echo ""
echo "MindX installed successfully!"
echo ""
echo "To get started:"
echo "  mindx --help          Show help"
echo "  mindx doctor          Check environment"
echo "  mindx start           Start daemon process"
echo "  systemctl status mindx-daemon   Check daemon service status"
echo ""
echo "Runtime data is in /usr/share/mindx/runtime/"
echo "User data will be created at ~/.mindx/ on first run."
