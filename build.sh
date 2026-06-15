#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════════════
#  Entropy // Media Lift — Build Script
#  Usage: ./build.sh [dev|release|clean]
#
#  dev     — frontend only, vite dev server (hot reload)
#  release — full production build → releases/
#  clean   — remove build artifacts
#  (default = release)
# ═══════════════════════════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_DIR="$SCRIPT_DIR"
FRONTEND_DIR="$APP_DIR/frontend"
BACKEND_DIR="$APP_DIR/backend"
RELEASES_DIR="$APP_DIR/releases"

# ─── Version detection ───
VERSION=""
if [ -f "$APP_DIR/VERSION" ]; then
  VERSION="$(cat "$APP_DIR/VERSION" | tr -d '[:space:]')"
fi
if [ -z "$VERSION" ]; then
  VERSION="0.1.0"
fi

# ─── Platform detection ───
detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    msys*|mingw*|cygwin*) os="windows" ;;
    darwin*)               os="darwin" ;;
    linux*)                os="linux" ;;
  esac
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
  esac
  echo "${os}-${arch}"
}

PLATFORM="$(detect_platform)"
BINARY_NAME="entropy"
if [ "$PLATFORM" = "windows-amd64" ] || [ "$PLATFORM" = "windows-arm64" ]; then
  BINARY_NAME="entropy.exe"
fi
ARCHIVE_SLUG="entropy-v${VERSION}-${PLATFORM}"

# ─── Color helpers ───
c_green()  { printf '\033[0;32m%s\033[0m\n' "$1"; }
c_yellow() { printf '\033[0;33m%s\033[0m\n' "$1"; }
c_red()    { printf '\033[0;31m%s\033[0m\n' "$1"; }
c_bold()   { printf '\033[1m%s\033[0m\n' "$1"; }

step() {
  local n="$1" total="$2" msg="$3"
  printf "\n  [%d/%d] %s\n" "$n" "$total" "$msg"
}

# ═══════════════════════════════════════════════════════════════════════
#  Commands
# ═══════════════════════════════════════════════════════════════════════

cmd_dev() {
  c_bold "=== Entropy // Dev Mode ==="
  echo "  Starting Vite dev server with HMR..."
  echo "  Backend must be started separately: cd backend && go run ."
  echo ""
  cd "$FRONTEND_DIR"
  npx vite --host
}

cmd_clean() {
  c_bold "=== Entropy // Clean ==="
  echo "  Removing build artifacts..."
  rm -rf "$FRONTEND_DIR/build"
  rm -rf "$BACKEND_DIR/webdist"
  rm -rf "$RELEASES_DIR"
  rm -f "$BACKEND_DIR/$BINARY_NAME"
  rm -f "$BACKEND_DIR/entropy"
  rm -f "$BACKEND_DIR/entropy.exe"
  c_green "  Done."
}

cmd_release() {
  local total_steps=6
  c_bold "=== Entropy // Media Lift — Build v${VERSION} ==="
  echo "  Platform:  ${PLATFORM}"
  echo "  Binary:    ${BINARY_NAME}"
  echo "  Release:   ${RELEASES_DIR}/"
  echo ""

  # 1. Install frontend deps
  step 1 "$total_steps" "Installing frontend dependencies..."
  cd "$FRONTEND_DIR"
  rm -rf node_modules
  npm install --no-fund --no-audit --legacy-peer-deps

  # 2. Build frontend
  step 2 "$total_steps" "Building frontend (tsc + vite)..."
  npm run build
  c_green "    Frontend built → frontend/build/"

  # 3. Copy to webdist for go:embed
  step 3 "$total_steps" "Copying frontend → backend/webdist..."
  rm -rf "$BACKEND_DIR/webdist"
  cp -r "$FRONTEND_DIR/build" "$BACKEND_DIR/webdist"

  # 4. Build Go binary
  step 4 "$total_steps" "Building Go binary..."
  cd "$BACKEND_DIR"

  # Build flags
  local LDFLAGS="-s -w -X main.version=${VERSION}"
  go build -ldflags "$LDFLAGS" -o "$BINARY_NAME" .
  c_green "    Binary built → backend/${BINARY_NAME}"

  # 5. Verify webdist has content
  step 5 "$total_steps" "Verifying embedded frontend..."
  if [ ! -f "$BACKEND_DIR/webdist/index.html" ]; then
    c_red "    ERROR: webdist/index.html missing — frontend will be blank"
    exit 1
  fi
  local asset_count
  asset_count="$(find "$BACKEND_DIR/webdist/assets" -type f 2>/dev/null | wc -l)"
  c_green "    Embedded ${asset_count} asset file(s) into Go binary"

  # 6. Package into releases/
  step 6 "$total_steps" "Packaging release..."
  mkdir -p "$RELEASES_DIR"

  local ARCHIVE_PATH="${RELEASES_DIR}/${ARCHIVE_SLUG}.tar.gz"
  local STAGE_NAME="entropy-v${VERSION}"

  # Create a staging directory for the archive
  local STAGING_DIR
  STAGING_DIR="$(mktemp -d)"
  mkdir -p "$STAGING_DIR/$STAGE_NAME"

  # Copy binary into archive folder
  cp "$BACKEND_DIR/$BINARY_NAME" "$STAGING_DIR/$STAGE_NAME/"

  # Copy build info
  cat > "$STAGING_DIR/$STAGE_NAME/RELEASE_INFO" <<EOF
Entropy // Media Lift
Version:  ${VERSION}
Platform: ${PLATFORM}
Built:    $(date -u '+%Y-%m-%dT%H:%M:%SZ')
Go:       $(go version 2>/dev/null | head -1 || echo 'unknown')
Node:     $(node --version 2>/dev/null || echo 'unknown')
EOF

  # Create tar.gz
  tar -czf "$ARCHIVE_PATH" -C "$STAGING_DIR" "$STAGE_NAME"
  rm -rf "$STAGING_DIR"

  # Also copy raw binary directly into releases/ for quick access
  cp "$BACKEND_DIR/$BINARY_NAME" "${RELEASES_DIR}/entropy"
  if [ "$PLATFORM" = "windows-amd64" ] || [ "$PLATFORM" = "windows-arm64" ]; then
    cp "$BACKEND_DIR/$BINARY_NAME" "${RELEASES_DIR}/entropy.exe"
  fi

  # Generate checksum
  if command -v sha256sum &>/dev/null; then
    (cd "$RELEASES_DIR" && sha256sum "$(basename "$ARCHIVE_PATH")" > "$(basename "$ARCHIVE_PATH").sha256")
    c_green "    Checksum → $(basename "$ARCHIVE_PATH").sha256"
  fi

  # Summary
  local binary_size
  binary_size="$(du -h "${RELEASES_DIR}/$BINARY_NAME" | cut -f1)"
  local archive_size
  archive_size="$(du -h "$ARCHIVE_PATH" | cut -f1)"

  echo ""
  c_green "  ┌──────────────────────────────────────────────┐"
  c_green "  │  Build complete                              │"
  c_green "  ├──────────────────────────────────────────────┤"
  printf "  │  Binary:   %-31s│\n" "$BINARY_NAME ($binary_size)"
  printf "  │  Archive:  %-31s│\n" "$(basename "$ARCHIVE_PATH") ($archive_size)"
  printf "  │  Path:     %-31s│\n" "$RELEASES_DIR/"
  c_green "  └──────────────────────────────────────────────┘"
  echo ""
  c_yellow "  Run:  ./releases/entropy"
  echo ""
}

# ═══════════════════════════════════════════════════════════════════════
#  Main
# ═══════════════════════════════════════════════════════════════════════

COMMAND="${1:-release}"

case "$COMMAND" in
  dev|dev-server|serve)  cmd_dev ;;
  release|build|prod)    cmd_release ;;
  clean|c)               cmd_clean ;;
  *)
    echo "Usage: $0 [dev|release|clean]"
    echo ""
    echo "  dev     Start Vite dev server (frontend only, HMR)"
    echo "  release Full production build → releases/"
    echo "  clean   Remove all build artifacts"
    exit 1
    ;;
esac