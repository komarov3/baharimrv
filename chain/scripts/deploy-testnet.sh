#!/usr/bin/env bash
# BahariMRV chain — deploy conservation_chain to IOTA Testnet
# 
# Prerequisites:
#   - iota CLI installed: brew install iotaledger/tap/iota
#   - Active testnet address with gas: iota client active-address && iota client gas
#   - Testnet tokens: iota client faucet
#
# After deploy, set in backend/.env:
#   CHAIN_MODE=iota
#   IOTA_PACKAGE_ID=<output PackageID>
#   IOTA_EXECUTOR_URL=<executor service URL>
set -euo pipefail

CHAIN_DIR="$(cd "$(dirname "$0")/.." && pwd)"
echo "Deploying conservation_chain from: $CHAIN_DIR"

# Switch Move.toml to git dependency for deployment (not local path)
echo "Updating Move.toml to use git dependency for deployment..."
cat > "$CHAIN_DIR/Move.toml" << 'TOML'
[package]
name = "conservation_chain"
edition = "2024"

[dependencies]
Iota = { git = "https://github.com/iotaledger/iota.git", subdir = "crates/iota-framework/packages/iota-framework", rev = "v1.19.1" }

[addresses]
conservation_chain = "0x0"

[dev-addresses]
TOML

# Build first to catch any errors
echo "Building..."
cd "$CHAIN_DIR"
iota move build

# Check wallet has gas
echo "Checking wallet balance..."
iota client gas

# Publish
echo ""
echo "Publishing to IOTA Testnet..."
RESULT=$(iota client publish --gas-budget 100000000 2>&1)
echo "$RESULT"

# Extract the package ID
PACKAGE_ID=$(echo "$RESULT" | grep -oE '0x[a-f0-9]{64}' | head -1)
echo ""
echo "═══════════════════════════════════════════"
echo "DEPLOYMENT COMPLETE"
echo "Package ID: $PACKAGE_ID"
echo ""
echo "Add to backend/.env:"
echo "  CHAIN_MODE=iota"
echo "  IOTA_PACKAGE_ID=$PACKAGE_ID"
echo "  IOTA_EXECUTOR_URL=<your executor service URL>"
echo "  IOTA_EXPLORER_URL=https://explorer.rebased.iota.org/testnet"
echo "═══════════════════════════════════════════"
echo ""
echo "Find shared object IDs (AnchorRegistry, IdentityRegistry):"
echo "  iota client objects --address \$(iota client active-address)"
