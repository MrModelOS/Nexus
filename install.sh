#!/bin/sh
set -e

REPO="MrModelOS/Nexus"
BINARY="nex"
VERSION="v1.0.0"
INSTALL_DIR="$HOME/.local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() {
  echo "${GREEN}[nex]${NC} $1"
}

warn() {
  echo "${YELLOW}[nex]${NC} $1"
}

error() {
  echo "${RED}[nex]${NC} $1"
  exit 1
}

# Detect OS and architecture
detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)

  case "$OS" in
    linux*)
      OS="linux"
      ;;
    darwin*)
      OS="darwin"
      ;;
    *)
      error "Unsupported OS: $OS"
      ;;
  esac

  case "$ARCH" in
    x86_64|amd64)
      ARCH="amd64"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      ;;
    armv7l|armhf)
      ARCH="arm"
      ;;
    *)
      error "Unsupported architecture: $ARCH"
      ;;
  esac

  PLATFORM="${OS}-${ARCH}"
  info "Detected platform: ${PLATFORM}"
}

# Check if curl or wget is available
check_deps() {
  if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
  elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
  else
    error "curl or wget is required"
  fi
}

# Download binary
download() {
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}-${PLATFORM}"

  info "Downloading ${URL}..."

  mkdir -p "$INSTALL_DIR"

  if [ "$DOWNLOADER" = "curl" ]; then
    curl -fsSL "$URL" -o "${INSTALL_DIR}/${BINARY}"
  else
    wget -q "$URL" -O "${INSTALL_DIR}/${BINARY}"
  fi

  chmod +x "${INSTALL_DIR}/${BINARY}"
}

# Verify installation
verify() {
  if [ -x "${INSTALL_DIR}/${BINARY}" ]; then
    VERSION_OUTPUT=$("${INSTALL_DIR}/${BINARY}" --version 2>/dev/null || echo "unknown")
    info "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
    info "Version: ${VERSION_OUTPUT}"
  else
    error "Installation failed"
  fi
}

# Check if install dir is in PATH
check_path() {
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*)
      ;;
    *)
      warn "${INSTALL_DIR} is not in your PATH"
      warn "Add this to your shell config:"
      warn "  export PATH=\"\$HOME/.local/bin:\$PATH\""
      ;;
  esac
}

# Main
main() {
  info "Installing Nexus AI CLI..."
  detect_platform
  check_deps
  download
  verify
  check_path
  info "Done! Run 'nex' to start."
}

main "$@"
