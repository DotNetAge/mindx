#!/bin/bash
# =============================================================================
# MindX Homebrew Release Script
#
# Builds the macOS binary, creates a tarball, calculates SHA256, and generates
# a Homebrew formula (.rb) ready for use in a Homebrew tap.
#
# Usage:
#   ./scripts/homebrew-release.sh
#   Requires a git tag (e.g. git tag v1.0.0) — the tag is used as the version.
#
# Output:
#   dist/mindx-<version>-darwin-amd64.tar.gz
#   dist/mindx-<version>-darwin-arm64.tar.gz
#   dist/mindx-<version>.rb               # Homebrew formula
#
# Environment:
#   GITHUB_REPO     Default: DotNetAge/mindx
#   HOMEBREW_TAP    Default: DotNetAge/homebrew-tap
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$SCRIPT_DIR"

# ── Config ──────────────────────────────────────────────────────────────────
GITHUB_REPO="${GITHUB_REPO:-DotNetAge/mindx}"
HOMEBREW_TAP="${HOMEBREW_TAP:-DotNetAge/homebrew-tap}"

# ── Version ──────────────────────────────────────────────────────────────────
VERSION="$(git describe --tags --abbrev=0 2>/dev/null)"
if [ -z "$VERSION" ]; then
  echo "❌ No git tag found. Create a tag first: git tag v1.0.0"
  exit 1
fi
VERSION="${VERSION#v}"
echo "🔖 Version: $VERSION"

# ── Build ────────────────────────────────────────────────────────────────────
echo "🏗️  Building darwin/amd64..."
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/mindx-darwin-amd64 .

echo "🏗️  Building darwin/arm64..."
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/mindx-darwin-arm64 .

# ── Package ──────────────────────────────────────────────────────────────────
echo "📦 Creating tarballs..."

mkdir -p dist

tar czf "dist/mindx-${VERSION}-darwin-amd64.tar.gz" -C dist mindx-darwin-amd64
tar czf "dist/mindx-${VERSION}-darwin-arm64.tar.gz" -C dist mindx-darwin-arm64

SHA256_AMD64=$(shasum -a 256 "dist/mindx-${VERSION}-darwin-amd64.tar.gz" | cut -d' ' -f1)
SHA256_ARM64=$(shasum -a 256 "dist/mindx-${VERSION}-darwin-arm64.tar.gz" | cut -d' ' -f1)

rm -f dist/mindx-darwin-amd64 dist/mindx-darwin-arm64

echo "  amd64: $SHA256_AMD64"
echo "  arm64: $SHA256_ARM64"

# ── Generate Formula ─────────────────────────────────────────────────────────
echo "📝 Generating Homebrew formula..."

cat > "dist/mindx-${VERSION}.rb" << EOF
# typed: false
# frozen_string_literal: true

class Mindx < Formula
  desc "MindX - AI-native multi-agent conversation platform with OPC capabilities"
  homepage "https://github.com/${GITHUB_REPO}"
  license "MIT"
  version "${VERSION}"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/mindx-${VERSION}-darwin-amd64.tar.gz"
      sha256 "${SHA256_AMD64}"
    end

    if Hardware::CPU.arm?
      url "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/mindx-${VERSION}-darwin-arm64.tar.gz"
      sha256 "${SHA256_ARM64}"
    end
  end

  depends_on "go" => :build

  def install
    bin.install "mindx-darwin-#{Hardware::CPU.arm? ? "arm64" : "amd64"}" => "mindx"
  end

  test do
    assert_match "MindX", shell_output("#{bin}/mindx --help")
  end
end
EOF

echo "✅ Done!"
echo ""
echo "  Formula: dist/mindx-${VERSION}.rb"
echo "  amd64:   dist/mindx-${VERSION}-darwin-amd64.tar.gz"
echo "  arm64:   dist/mindx-${VERSION}-darwin-arm64.tar.gz"
echo ""
echo "To publish:"
echo "  1. Upload tarballs to GitHub release v${VERSION}"
echo "  2. Copy formula to your tap:"
echo "     cp dist/mindx-${VERSION}.rb ../homebrew-tap/Formula/mindx.rb"
echo "  3. Commit and push the tap"
echo ""
echo "Users can then install with:"
echo "  brew install ${HOMEBREW_TAP}/mindx"
