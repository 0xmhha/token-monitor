#!/usr/bin/env sh
# token-monitor one-line installer.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/0xmhha/token-monitor/main/scripts/install.sh | sh
#
# Detects OS+arch, downloads the latest release tarball, extracts it,
# and moves the binary to /usr/local/bin (or $TOKEN_MONITOR_INSTALL_DIR
# if set). Falls back to sudo if the install dir is not writable.
#
# Environment overrides:
#   TOKEN_MONITOR_INSTALL_DIR   target dir (default: /usr/local/bin)
#   TOKEN_MONITOR_VERSION       specific tag instead of latest (e.g. v0.2.0)
#
# Windows is not supported by this script; download a release zip from
# https://github.com/0xmhha/token-monitor/releases manually.

set -eu

REPO="0xmhha/token-monitor"
INSTALL_DIR="${TOKEN_MONITOR_INSTALL_DIR:-/usr/local/bin}"
VERSION="${TOKEN_MONITOR_VERSION:-latest}"

# --- detect OS ---
case "$(uname -s)" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux"  ;;
    *)
        echo "error: unsupported OS '$(uname -s)' — see https://github.com/${REPO}/releases for manual install" >&2
        exit 1
        ;;
esac

# --- detect arch ---
case "$(uname -m)" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)
        echo "error: unsupported arch '$(uname -m)'" >&2
        exit 1
        ;;
esac

ASSET="token-monitor_${OS}_${ARCH}.tar.gz"

if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

# --- prerequisites ---
for cmd in curl tar mktemp; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "error: required command '$cmd' not found in PATH" >&2
        exit 1
    fi
done

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t token-monitor)
trap 'rm -rf "$TMP"' EXIT INT TERM

echo "Downloading ${ASSET} (${VERSION})..."
if ! curl -fsSL "$URL" -o "$TMP/$ASSET"; then
    echo "error: download failed: $URL" >&2
    echo "       check that the asset exists at https://github.com/${REPO}/releases" >&2
    exit 1
fi

echo "Extracting..."
tar -xzf "$TMP/$ASSET" -C "$TMP"

if [ ! -f "$TMP/token-monitor" ]; then
    echo "error: token-monitor binary not found in archive" >&2
    exit 1
fi

# --- install ---
if [ -w "$INSTALL_DIR" ] || ([ ! -e "$INSTALL_DIR" ] && [ -w "$(dirname "$INSTALL_DIR")" ]); then
    mkdir -p "$INSTALL_DIR"
    mv "$TMP/token-monitor" "$INSTALL_DIR/token-monitor"
elif command -v sudo >/dev/null 2>&1; then
    echo "Moving to ${INSTALL_DIR} (requires sudo)..."
    sudo mkdir -p "$INSTALL_DIR"
    sudo mv "$TMP/token-monitor" "$INSTALL_DIR/token-monitor"
else
    echo "error: ${INSTALL_DIR} is not writable and sudo is unavailable" >&2
    echo "       set TOKEN_MONITOR_INSTALL_DIR to a writable directory and re-run" >&2
    exit 1
fi

# --- verify ---
INSTALLED_BIN="$INSTALL_DIR/token-monitor"
chmod +x "$INSTALLED_BIN" 2>/dev/null || sudo chmod +x "$INSTALLED_BIN" 2>/dev/null || true

echo ""
echo "Installed: ${INSTALLED_BIN}"
if "$INSTALLED_BIN" --version >/dev/null 2>&1; then
    echo "Version:   $("$INSTALLED_BIN" --version)"
fi

echo ""
echo "Next steps:"
echo "  1) Wire into Claude Code (idempotent, atomic, with backups):"
echo "       token-monitor install all"
echo ""
echo "  2) Restart Claude Code so the statusline + hook + MCP server are picked up."
echo ""
echo "  See https://github.com/${REPO}#installation-automation for details."
