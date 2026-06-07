#!/bin/bash
# Post-install script for .deb package

set -e

echo "MindX installed successfully!"
echo ""
echo "To get started:"
echo "  mindx --help          Show help"
echo "  mindx start           Start daemon mode"
echo "  mindx doctor          Check environment"
echo ""
echo "Runtime data is in /usr/share/mindx/runtime/"
echo "User data will be created at ~/.mindx/ on first run."
