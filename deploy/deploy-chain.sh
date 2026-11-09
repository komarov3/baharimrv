#!/usr/bin/env zsh
# ─────────────────────────────────────────────────────────────────────────────
# BahariMRV — conservation_chain Deployer
#
# Deploys conservation_chain to IOTA Rebased Testnet and transfers the AdminCap
# to the dedicated BahariMRV admin wallet (from deploy/.admin-keys).
#
# Modules deployed:
#   roles            — AdminCap, ManagerCap, MonitorCap, VerifierCap
#   notary           — ProvenanceAnchor (conservation provenance)
#   identity         — IdentityDocument + IdentityRegistry
#   project_ledger   — ProjectLedger (NGO / buyer contribution tracking)
#   planting_record  — PlantingRecord (mangrove MRV)
#   restoration      — RestorationClaim + FulfillmentRecord
#   certificates     — ConservationCertificate NFTs
#   carbon_credit    — CarbonCredit tokens + CarbonCreditRegistry
#   mrv_registry     — MRVAttestation registry (links records to proofs)
#
# Shared objects created on deploy (captured in deploy/.admin-keys):
#   AnchorRegistry, IdentityRegistry, MRVRegistry, CarbonCreditRegistry
#
# Usage:
#   ./deploy/deploy-chain.sh        # deploy
#   ./deploy/deploy-chain.sh test   # run Move tests only
#
# After deploy, add the printed env vars to:
#   backend/.env           (Go backend)
#   executor/.env          (TypeScript executor / gas station)
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

SCRIPT_DIR="${0:a:h}"
REPO_ROOT="${SCRIPT_DIR}/.."
CHAIN_DIR="${REPO_ROOT}/chain"
KEYS_FILE="${SCRIPT_DIR}/.admin-keys"

IOTA_FRAMEWORK_GIT="https://github.com/iotaledger/iota.git"
IOTA_FRAMEWORK_SUBDIR="crates/iota-framework/packages/iota-framework"
IOTA_FRAMEWORK_REV="v1.19.1"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'
info()    { echo -e "${BLUE}[info]${NC}  $*"; }
success() { echo -e "${GREEN}[ok]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[warn]${NC}  $*"; }
error()   { echo -e "${RED}[error]${NC} $*" >&2; exit 1; }
banner()  { echo -e "\n${BOLD}${BLUE}── $* ──${NC}"; }

# ─── Checks ───────────────────────────────────────────────────────────────────
check_prereqs() {
    command -v iota &>/dev/null || error "iota CLI not found"

    local env
    env=$(iota client active-env 2>/dev/null || true)
    [[ "${env}" == *testnet* ]] || error "Active IOTA env is '${env}', not testnet. Switch: iota client switch --env testnet"

    local addr
    addr=$(iota client active-address 2>/dev/null || true)
    [[ -n "${addr}" ]] || error "No active IOTA address. Create one: iota keytool generate ed25519"
    info "Deploying from: ${addr}"
}

# ─── Move.toml management ────────────────────────────────────────────────────
patch_move_toml() {
    local toml="${CHAIN_DIR}/Move.toml"
    cp "${toml}" "${toml}.bak"
    sed -i.tmp \
        "s|Iota = { local = .*}|Iota = { git = \"${IOTA_FRAMEWORK_GIT}\", subdir = \"${IOTA_FRAMEWORK_SUBDIR}\", rev = \"${IOTA_FRAMEWORK_REV}\" }|" \
        "${toml}"
    rm -f "${toml}.tmp"
}

restore_move_toml() {
    local toml="${CHAIN_DIR}/Move.toml"
    [[ -f "${toml}.bak" ]] && mv "${toml}.bak" "${toml}"
}

# ─── Tests ────────────────────────────────────────────────────────────────────
run_tests() {
    banner "Running conservation_chain tests"
    cd "${CHAIN_DIR}"
    iota move test
    success "All tests passed"
}

# ─── Deploy ───────────────────────────────────────────────────────────────────
deploy() {
    banner "Building conservation_chain"
    cd "${CHAIN_DIR}"
    iota move build

    banner "Publishing to IOTA Testnet"
    local raw
    raw=$(iota client publish --gas-budget 100000000 --json 2>&1)

    local pkg_id
    pkg_id=$(echo "${raw}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for c in data.get('objectChanges', []):
    if c.get('type') == 'published':
        print(c['packageId'])
        break
" 2>/dev/null || true)

    [[ -n "${pkg_id}" ]] || { echo "${raw}"; error "Publish failed — no packageId found"; }
    success "Package published: ${pkg_id}"

    # Extract shared object IDs from objectChanges
    # Shared objects: AnchorRegistry, IdentityRegistry, MRVRegistry,
    #                 CarbonCreditRegistry, CapRegistry
    local shared_ids
    shared_ids=$(echo "${raw}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
pkg = '${pkg_id}'
results = {}
for c in data.get('objectChanges', []):
    if c.get('type') != 'created': continue
    t = c.get('objectType', '')
    oid = c.get('objectId', '')
    if not oid: continue
    if 'notary::AnchorRegistry' in t:     results['ANCHOR_REGISTRY'] = oid
    if 'identity::IdentityRegistry' in t: results['IDENTITY_REGISTRY'] = oid
    if 'mrv_registry::MRVRegistry' in t:  results['MRV_REGISTRY'] = oid
    if 'carbon_credit::CarbonCreditRegistry' in t: results['CARBON_CREDIT_REGISTRY'] = oid
    if 'roles::AdminCap' in t:            results['ADMIN_CAP'] = oid
for k,v in results.items():
    print(f'{k}={v}')
" 2>/dev/null || true)

    local anchor_reg="" identity_reg="" mrv_reg="" carbon_reg="" admin_cap_id=""
    while IFS='=' read -r key val; do
        case "${key}" in
            ANCHOR_REGISTRY)          anchor_reg="${val}" ;;
            IDENTITY_REGISTRY)        identity_reg="${val}" ;;
            MRV_REGISTRY)             mrv_reg="${val}" ;;
            CARBON_CREDIT_REGISTRY)   carbon_reg="${val}" ;;
            ADMIN_CAP)                admin_cap_id="${val}" ;;
        esac
    done <<< "${shared_ids}"

    # Transfer AdminCap to dedicated BahariMRV admin wallet
    local bahari_admin=""
    if [[ -f "${KEYS_FILE}" ]]; then
        bahari_admin=$(grep "^BAHARIMRV_ADMIN_ADDRESS=" "${KEYS_FILE}" | cut -d= -f2 || true)
    fi

    if [[ -n "${bahari_admin}" && -n "${admin_cap_id}" ]]; then
        info "Waiting for objects to be indexed..."
        sleep 4
        info "Transferring AdminCap (${admin_cap_id}) → ${bahari_admin}..."
        iota client transfer --to "${bahari_admin}" --object-id "${admin_cap_id}" --gas-budget 10000000 >/dev/null
        success "AdminCap transferred to BahariMRV admin"
    elif [[ -z "${bahari_admin}" ]]; then
        warn "No BAHARIMRV_ADMIN_ADDRESS in .admin-keys — AdminCap stays with deployer"
        warn "Run: ./deploy/gen-admin-keys.sh to generate a dedicated admin wallet"
    elif [[ -z "${admin_cap_id}" ]]; then
        warn "Could not auto-detect AdminCap object ID — transfer manually:"
        warn "  iota client objects --json | grep AdminCap"
    fi

    # Update .admin-keys
    if [[ -f "${KEYS_FILE}" ]]; then
        local sed_cmd="sed -i.bak"
        ${sed_cmd} "s|CONSERVATION_CHAIN_PACKAGE_ID=.*|CONSERVATION_CHAIN_PACKAGE_ID=${pkg_id}|" "${KEYS_FILE}"
        [[ -n "${admin_cap_id}" ]]  && ${sed_cmd} "s|BAHARIMRV_ADMIN_CAP_ID=.*|BAHARIMRV_ADMIN_CAP_ID=${admin_cap_id}|" "${KEYS_FILE}"
        [[ -n "${anchor_reg}" ]]    && ${sed_cmd} "s|ANCHOR_REGISTRY_OBJECT_ID=.*|ANCHOR_REGISTRY_OBJECT_ID=${anchor_reg}|" "${KEYS_FILE}"
        [[ -n "${identity_reg}" ]]  && ${sed_cmd} "s|IDENTITY_REGISTRY_OBJECT_ID=.*|IDENTITY_REGISTRY_OBJECT_ID=${identity_reg}|" "${KEYS_FILE}"
        [[ -n "${mrv_reg}" ]]       && ${sed_cmd} "s|MRV_REGISTRY_OBJECT_ID=.*|MRV_REGISTRY_OBJECT_ID=${mrv_reg}|" "${KEYS_FILE}"
        [[ -n "${carbon_reg}" ]]    && ${sed_cmd} "s|CARBON_CREDIT_REGISTRY_OBJECT_ID=.*|CARBON_CREDIT_REGISTRY_OBJECT_ID=${carbon_reg}|" "${KEYS_FILE}"
        rm -f "${KEYS_FILE}.bak"
        success "deploy/.admin-keys updated"
    fi

    # Print env blocks
    banner "Deployment complete"
    echo ""
    echo "────────────────────────────────────────────────────────────────"
    echo "  Add to backend/.env:"
    echo ""
    echo "  CHAIN_MODE=iota"
    echo "  IOTA_PACKAGE_ID=${pkg_id}"
    echo "  IOTA_ADMIN_CAP_ID=${admin_cap_id:-<see .admin-keys>}"
    echo "  IOTA_ADMIN_ADDRESS=${bahari_admin:-<see .admin-keys>}"
    echo "  IOTA_RPC_URL=https://api.testnet.iota.cafe"
    echo "  IOTA_EXPLORER_URL=https://explorer.iota.cafe"
    echo "  ANCHOR_REGISTRY_OBJECT_ID=${anchor_reg:-<not captured>}"
    echo "  IDENTITY_REGISTRY_OBJECT_ID=${identity_reg:-<not captured>}"
    echo "  MRV_REGISTRY_OBJECT_ID=${mrv_reg:-<not captured>}"
    echo "  CARBON_CREDIT_REGISTRY_OBJECT_ID=${carbon_reg:-<not captured>}"
    echo ""
    echo "  Add to executor/.env:"
    echo ""
    echo "  IOTA_PACKAGE_ID=${pkg_id}"
    echo "  IOTA_RPC_URL=https://api.testnet.iota.cafe"
    echo "  ANCHOR_REGISTRY_OBJECT_ID=${anchor_reg:-<not captured>}"
    echo "  IDENTITY_REGISTRY_OBJECT_ID=${identity_reg:-<not captured>}"
    echo "  MRV_REGISTRY_OBJECT_ID=${mrv_reg:-<not captured>}"
    echo "  CARBON_CREDIT_REGISTRY_OBJECT_ID=${carbon_reg:-<not captured>}"
    echo "  GAS_BUDGET=50000000"
    echo "  # SPONSOR_PRIVATE_KEY=<BahariMRV admin private key — bech32 or base64>"
    echo "  # API_KEY=<random secret for backend→executor auth>"
    echo "────────────────────────────────────────────────────────────────"
    echo ""
}

# ─── Main ─────────────────────────────────────────────────────────────────────
main() {
    case "${1:-deploy}" in
        test)
            run_tests
            ;;
        deploy|"")
            check_prereqs
            run_tests
            patch_move_toml
            trap "restore_move_toml" EXIT
            deploy
            restore_move_toml
            trap - EXIT
            ;;
        *)
            echo "Usage: $0 [deploy | test]"
            exit 1
            ;;
    esac
}

main "$@"
