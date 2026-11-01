#!/bin/bash
# Post-install script for .deb/.rpm packages
set -e

echo ""
echo "MindX installed successfully!"
echo ""
echo "To get started:"
echo "  mindx --help          Show help"
echo "  mindx install         Set up workspace and register the per-user background service"
echo "  mindx doctor          Check environment"
echo ""
echo "Runtime data is in /usr/share/mindx/runtime/"
echo "User data will be created at ~/.mindx/ on first run."
