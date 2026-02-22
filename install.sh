#!/bin/sh
# Install whatsapp-mcp — downloads the pre-built binary for your platform.
# Usage: curl -fsSL https://raw.githubusercontent.com/ihiteshgupta/whatsapp-mcp/main/install.sh | sh

set -e

REPO="ihiteshgupta/whatsapp-mcp"
BINARY="whatsapp-mcp"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  darwin|linux) ;;
  *)
    echo "Unsupported OS: $OS. Please build from source:"
    echo "  cd whatsapp-bridge-v2 && go build -o whatsapp-mcp ./cmd/whatsapp-mcp"
    exit 1
    ;;
esac

# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
if [ -z "$LATEST" ]; then
  echo "Failed to fetch latest release"
  exit 1
fi

TARBALL="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${TARBALL}"

echo "Installing ${BINARY} ${LATEST} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/$TARBALL"
tar -xzf "$TMP/$TARBALL" -C "$TMP"

if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo install -m 755 "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  install -m 755 "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "Installed: $INSTALL_DIR/$BINARY"
echo ""
echo "Add to your MCP client config:"
echo '{'
echo '  "mcpServers": {'
echo '    "whatsapp": {'
echo "      \"command\": \"$INSTALL_DIR/$BINARY\","
echo '      "args": []'
echo '    }'
echo '  }'
echo '}'
