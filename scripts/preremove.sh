#!/bin/bash
# Pre-remove script for .deb package

set -e

echo "Removing MindX..."
echo "Note: User data (~/.mindx/) is preserved."
echo "To completely remove, run:"
echo "  rm -rf ~/.mindx/"
