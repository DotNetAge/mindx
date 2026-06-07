#!/bin/bash
# =============================================================================
# MindX Homebrew Release Script
#
# Generates a Homebrew formula (.rb) from a GitHub Release.
# Requires: gh CLI, git tag, and release assets already uploaded.
#
# Usage:
#   ./scripts/homebrew-release.sh              # auto-detect version from git tag
#   ./scripts/homebrew-release.sh v2.2.0       # specify version explicitly
#
# Output:
#   dist/mindx-<version>.rb                   # Homebrew formula (ready for tap)
#
# This script uses scripts/homebrew-formula.rb.tpl as template,
# keeping it in sync with the CI pipeline in .github/workflows/release.yml.
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
HOMEBREW_TAP="${HOMEBREW_TAP:-DotNetAge/homebrew-mindx}"

# ── Version ──────────────────────────────────────────────────────────────────
if [ -n "${1:-}" ]; then
  VERSION_TAG="$1"
else
  VERSION_TAG="$(git describe --tags --abbrev=0 2>/dev/null)"
fi

if [ -z "$VERSION_TAG" ]; then
  echo "ERROR: No git tag found. Create one first:"
  echo "  git tag v2.2.0 && git push origin v2.2.0"
  exit 1
fi

VERSION="${VERSION_TAG#v}"
echo "Version: ${VERSION_TAG} (${VERSION})"

# ── Check for gh CLI ────────────────────────────────────────────────────────
if ! command -v gh >/dev/null 2>&1; then
  echo "ERROR: gh CLI required. Install: brew install gh"
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "ERROR: gh not logged in. Run: gh auth login"
  exit 1
fi

# ── Download darwin assets from GitHub Release ───────────────────────────────
echo ""
echo "Downloading darwin assets from release ${VERSION_TAG}..."
mkdir -p dist

gh release download "${VERSION_TAG}" \
  --pattern "mindx-*darwin-*.tar.gz" \
  --dir dist \
  --repo "${GITHUB_REPO}" 2>/dev/null || {
  echo "ERROR: Could not download assets. Does release ${VERSION_TAG} exist?"
  echo "  Run 'make release-publish' or push a tag to create it."
  exit 1
}

ls -lh dist/

# ── Calculate SHA256 ─────────────────────────────────────────────────────────
SHA256_AMD64=$(shasum -a 256 "dist/mindx-${VERSION}-darwin-amd64.tar.gz" | cut -d' ' -f1)
SHA256_ARM64=$(shasum -a 256 "dist/mindx-${VERSION}-darwin-arm64.tar.gz" | cut -d' ' -f1)

echo ""
echo "SHA256 (amd64): ${SHA256_AMD64}"
echo "SHA256 (arm64): ${SHA256_ARM64}"

# ── Generate formula from template ──────────────────────────────────────────
echo ""
echo "Generating Homebrew formula..."

sed \
  -e "s|__GITHUB_REPO__|${GITHUB_REPO}|g" \
  -e "s|__VERSION__|${VERSION}|g" \
  -e "s|__TAG__|${VERSION_TAG}|g" \
  -e "s|__SHA256_AMD64__|${SHA256_AMD64}|g" \
  -e "s|__SHA256_ARM64__|${SHA256_ARM64}|g" \
  scripts/homebrew-formula.rb.tpl > "dist/mindx-${VERSION}.rb"

echo "---"
cat "dist/mindx-${VERSION}.rb"
echo ""

# ── Cleanup downloaded tarballs (keep only formula) ────────────────────────
rm -f dist/mindx-*-darwin-*.tar.gz

# ── Done ────────────────────────────────────────────────────────────────────
echo "Done!"
echo ""
echo "  Formula:  dist/mindx-${VERSION}.rb"
echo "  Tap:       ${HOMEBREW_TAP}"
echo ""
echo "To publish to your tap:"
echo "  cp dist/mindx-${VERSION}.rb /path/to/${HOMEBREW_TAP}/Formula/mindx.rb"
echo "  cd /path/to/${HOMEBREW_TAP} && git add . && git commit -m \"mindx ${VERSION}\" && git push"
echo ""
echo "Users can install with:"
echo "  brew install ${HOMEBREW_TAP}/mindx"
