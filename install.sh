#!/bin/sh
set -e

REPO="edimuj/codex-rig"
BINARY="codex-rig"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [ "$OS" = "windows" ] && [ "$ARCH" = "arm64" ]; then
  echo "Windows arm64 is not supported. Use amd64." >&2
  exit 1
fi

VERSION=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version" >&2
  exit 1
fi

if [ "$OS" = "windows" ]; then
  EXT="zip"
  ARCHIVE="${BINARY}_${OS}_${ARCH}.${EXT}"
else
  EXT="tar.gz"
  ARCHIVE="${BINARY}_${OS}_${ARCH}.${EXT}"
fi

URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE}"

echo "Downloading ${BINARY} v${VERSION} for ${OS}/${ARCH}..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -sSL "$URL" -o "${TMPDIR}/${ARCHIVE}"

cd "$TMPDIR"
if [ "$EXT" = "zip" ]; then
  unzip -q "$ARCHIVE"
else
  tar xzf "$ARCHIVE"
fi

if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "$INSTALL_DIR/"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "$BINARY" "$INSTALL_DIR/"
fi

echo "Installed ${BINARY} v${VERSION} to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Run '${BINARY} --help' to get started."

