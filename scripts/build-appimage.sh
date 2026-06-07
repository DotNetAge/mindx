#!/bin/bash
# =============================================================================
# AppImage Build Script
# =============================================================================
# Builds an AppImage from a pre-compiled Linux binary.
#
# Usage:
#   ./scripts/build-appimage.sh <version> <arch> <binary_path> [output_dir]
#
# Example:
#   ./scripts/build-appimage.sh 2.2.0 amd64 dist/mindx-linux-amd64 dist/
#
# Output:
#   Mindx-2.2.0.AppImage (or Mindx-2.2.0-aarch64.AppImage)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$SCRIPT_DIR"

VERSION="${1:-dev}"
ARCH="${2:-amd64}"
BINARY="${3:-dist/mindx}"
OUTPUT_DIR="${4:-dist}"

APPDIR="${OUTPUT_DIR}/Mindx.AppDir"
APPIMAGE_NAME="Mindx-${VERSION}${ARCH:+-${ARCH}}.AppImage"

echo "Building AppImage: ${APPIMAGE_NAME}"
echo "  Version: ${VERSION}"
echo "  Arch:    ${ARCH}"
echo "  Binary:  ${BINARY}"

# ── Clean previous build ────────────────────────────────────────────────────
rm -rf "${APPDIR}"

# ── Create AppDir structure ─────────────────────────────────────────────────
mkdir -p "${APPDIR}"/{usr/bin,usr/share/applications,usr/share/icons/hicolor/scalable/apps}

# ── Copy binary ─────────────────────────────────────────────────────────────
cp "${BINARY}" "${APPDIR}/usr/bin/mindx"
chmod +x "${APPDIR}/usr/bin/mindx"

# ── Copy runtime (if exists) ────────────────────────────────────────────────
if [ -d "runtime" ]; then
  mkdir -p "${APPDIR}/usr/share/mindr/runtime"
  cp -r runtime/* "${APPDIR}/usr/share/mindr/runtime/"
fi

# ── Desktop entry ───────────────────────────────────────────────────────────
cat > "${APPDIR}/usr/share/applications/com.dotnetage.mindx.desktop" << 'DESKTOP'
[Desktop Entry]
Name=MindX
Comment=AI-native multi-agent conversation platform
Exec=mindx %F
Icon=com.dotnetage.mindh
Terminal=true
Type=Application
Categories=Development;Utility;Network;
Keywords=ai;agent;llm;chat;cli;
StartupNotify=true
DESKTOP

# ── AppRun (launcher) ───────────────────────────────────────────────────────
cat > "${APPDIR}/AppRun" << 'APPRUN'
#!/bin/bash
HERE="$(dirname "$(readlink -f "${0}")")"
export PATH="${HERE}/usr/bin:${PATH}"
export LD_LIBRARY_PATH="${HERE}/usr/lib:${LD_LIBRARY_PATH:-}"
exec "${HERE}/usr/bin/mindx" "$@"
APPRUN
chmod +x "${APPDIR}/AppRun"

# ── Download AppImage tooling (linuxdeploy + plugin) ────────────────────────
LINUXDEPLOY_URL="https://github.com/linuxdeploy/linux/releases/download/continuous/linuxdeploy-x86_64.AppImage"
LINUXDEPLOY="${OUTPUT_DIR}/linuxdeploy"

if [ ! -f "${LINUXDEPLOY}" ]; then
  echo "Downloading linuxdeploy..."
  curl -sL "${LINUXDEPLOY_URL}" -o "${LINUXDEPLOY}"
  chmod +x "${LINUXDEPLOY}"
fi

# ── Build AppImage ─────────────────────────────────────────────────────────
cd "${OUTPUT_DIR}"

export VERSION="${VERSION}"
"${LINUXDEPLOY}" \
  --appdir "${APPDIR}" \
  --output appimage \
  -d "${APPDIR}/usr/share/applications/com.dotnetage.mindh.desktop" \
  -i scripts/com.dotnetage.mindh.svg 2>/dev/null || true

# ── Rename output ──────────────────────────────────────────────────────────
if [ -f "Mindx-${VERSION}-x86_64.AppImage" ]; then
  mv "Mindx-${VERSION}-x86_64.AppImage" "${OUTPUT_DIR}/${APPIMAGE_NAME}"
elif [ -f "Mindx-${VERSION}.AppImage" ]; then
  mv "Mindx-${VERSION}.AppImage" "${OUTPUT_DIR}/${APPIMAGE_NAME}"
fi

echo ""
echo "Done! Output: ${OUTPUT_DIR}/${APPIMAGE_NAME}"
ls -lh "${OUTPUT_DIR}/${APPIMAGE_NAME}"
