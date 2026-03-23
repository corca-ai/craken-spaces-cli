#!/bin/sh
set -eu

REPO="corca-ai/craken-spaces-cli"
ARCHIVE_PREFIX="craken"
BINARY="spaces"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Determine install directory
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
if [ ! -d "$INSTALL_DIR" ] 2>/dev/null; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || true
fi
if [ ! -w "$INSTALL_DIR" ] 2>/dev/null; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# Get latest version
VERSION="${VERSION:-$(curl -sSf "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)}"
if [ -z "$VERSION" ]; then
  echo "Failed to determine latest version" >&2
  exit 1
fi
VERSION_NUM="${VERSION#v}"

# Download and install
ARCHIVE="${ARCHIVE_PREFIX}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"
CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

verify_checksum() {
  archive_path="$1"
  checksums_path="$2"
  expected="$(awk -v file="$ARCHIVE" '$2 == file { print $1 }' "$checksums_path")"
  if [ -z "$expected" ]; then
    echo "Missing checksum for $ARCHIVE" >&2
    exit 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive_path" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
  else
    echo "missing required command: sha256sum or shasum" >&2
    exit 1
  fi

  if [ "$actual" != "$expected" ]; then
    echo "checksum verification failed for $ARCHIVE" >&2
    exit 1
  fi
}

echo "Installing $BINARY $VERSION ($OS/$ARCH) to $INSTALL_DIR"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -sSfL "$URL" -o "$TMP/$ARCHIVE"
curl -sSfL "$CHECKSUMS_URL" -o "$TMP/checksums.txt"
verify_checksum "$TMP/$ARCHIVE" "$TMP/checksums.txt"
tar xzf "$TMP/$ARCHIVE" -C "$TMP"
install "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
echo "Installed $INSTALL_DIR/$BINARY"
