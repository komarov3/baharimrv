#!/usr/bin/env bash
# BahariMRV chain — local dev dependency setup
# Extracts the iota v1.19.1 framework matching CLI 1.19.1-fa39361cc638
# into /tmp/iota-framework-v1.19.1 so Move tests can run offline.
set -euo pipefail

CACHE_DIR="$HOME/.move/https___github_com_iotaledger_iota_git_testnet"
TARGET="/tmp/iota-framework-v1.19.1"

# Check CLI version
CLI_VER=$(iota --version 2>/dev/null || echo "")
if [[ -z "$CLI_VER" ]]; then
  echo "ERROR: iota CLI not found. Install via: brew install iotaledger/tap/iota" >&2
  exit 1
fi
echo "IOTA CLI: $CLI_VER"

# If the cache dir doesn't exist, we need to fetch it first
if [[ ! -d "$CACHE_DIR" ]]; then
  echo "Fetching IOTA framework from GitHub (one-time)..."
  mkdir -p "$(dirname "$CACHE_DIR")"
  git clone --filter=blob:none https://github.com/iotaledger/iota.git "$CACHE_DIR"
fi

# Extract v1.19.1 framework packages
echo "Extracting iota v1.19.1 framework..."
mkdir -p "$TARGET"
git -C "$CACHE_DIR" archive v1.19.1 crates/iota-framework/packages \
  | tar -x -C "$TARGET"

echo "Framework extracted to: $TARGET"
echo "Packages:"
ls "$TARGET/crates/iota-framework/packages/"

echo ""
echo "Run tests with:"
echo "  cd chain && iota move test --skip-fetch-latest-git-deps"
