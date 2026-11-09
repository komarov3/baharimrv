#!/usr/bin/env bash
# BahariMRV — Fly.io + Neon deployment script
#
# Deploys the BahariMRV API (Go) and Executor (Node.js) to Fly.io, wired to a
# Neon PostgreSQL database.
#
# Prerequisites:
#   - flyctl installed and authenticated (flyctl auth login)
#   - Neon account with a project created
#   - IOTA conservation_chain already deployed (see deploy/deploy-chain.sh)
#     so object IDs / sponsor key are available.
#
# Usage:
#   ./deploy/fly-deploy.sh            # guided deployment
#   ./deploy/fly-deploy.sh --checks   # validate local builds only
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
EXECUTOR_DIR="$REPO_ROOT/executor"

API_APP="baharimrv-api"
EXEC_APP="baharimrv-executor"

info()  { printf "\033[1;34m[info]\033[0m %s\n" "$*"; }
ok()    { printf "\033[1;32m[ok]\033[0m  %s\n" "$*"; }
warn()  { printf "\033[1;33m[warn]\033[0m %s\n" "$*"; }
fatal() { printf "\033[1;31m[fatal]\033[0m %s\n" "$*" >&2; exit 1; }

# ── 0. Prerequisites ──────────────────────────────────────────────────────────
check_prereqs() {
  info "Checking prerequisites…"
  command -v flyctl >/dev/null 2>&1 || fatal "flyctl not found — install: https://fly.io/docs/hands-on/install/"
  command -v go >/dev/null 2>&1 || fatal "go not found"
  command -v node >/dev/null 2>&1 || fatal "node not found"
  command -v npm >/dev/null 2>&1 || fatal "npm not found"
  flyctl auth whoami >/dev/null 2>&1 || fatal "Not logged into Fly.io — run: flyctl auth login"
  ok "Prerequisites satisfied"
}

# ── 1. Validate local builds ──────────────────────────────────────────────────
run_checks() {
  info "Validating local builds…"

  info "Backend: go build ./cmd/server"
  (cd "$BACKEND_DIR" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /tmp/baharimrv-server-check ./cmd/server) \
    || fatal "Backend build failed"
  rm -f /tmp/baharimrv-server-check
  ok "Backend builds"

  info "Executor: npm run build"
  (cd "$EXECUTOR_DIR" && npm run build) || fatal "Executor build failed"
  ok "Executor builds"

  ok "All local builds valid"
}
# ── 2. Create Fly.io apps ─────────────────────────────────────────────────────
create_apps() {
  info "Ensuring Fly.io apps exist…"

  if ! flyctl apps list --json 2>/dev/null | grep -q "\"$API_APP\""; then
    info "Creating API app: $API_APP"
    flyctl apps create "$API_APP" --org personal >/dev/null 2>&1 || \
      flyctl apps create "$API_APP" >/dev/null 2>&1 || \
      fatal "Failed to create Fly.io app $API_APP"
  else
    ok "API app $API_APP already exists"
  fi

  if ! flyctl apps list --json 2>/dev/null | grep -q "\"$EXEC_APP\""; then
    info "Creating Executor app: $EXEC_APP"
    flyctl apps create "$EXEC_APP" --org personal >/dev/null 2>&1 || \
      flyctl apps create "$EXEC_APP" >/dev/null 2>&1 || \
      fatal "Failed to create Fly.io app $EXEC_APP"
  else
    ok "Executor app $EXEC_APP already exists"
  fi
}

# ── 3. Neon database instructions ─────────────────────────────────────────────
neon_instructions() {
  cat <<EOF

──────────────────────────────────────────────────────────────────────────────
  MANUAL STEP — Neon PostgreSQL
──────────────────────────────────────────────────────────────────────────────

The BahariMRV API requires a PostgreSQL database with SSL. Neon provides a
serverless Postgres with SSL enabled by default.

1. Log in to https://console.neon.tech
2. Create a new project (e.g. "baharimrv-prod").
3. In the project dashboard, copy the connection string:
     postgresql://<user>:<password>@<endpoint>.aws.<region>.neon.tech/<db>?sslmode=require
4. Run the Postgres migrations against Neon (from backend/):
     DATABASE_URL="<neon connection string>" go run ./cmd/server   # runs migrations on startup
   or apply migrations manually from backend/internal/database/migrations/postgres/
5. Set the DATABASE_URL Fly.io secret:
     flyctl secrets set DATABASE_URL="<neon connection string>" -a baharimrv-api

──────────────────────────────────────────────────────────────────────────────
EOF
}

# ── 4. Set Fly.io secrets ─────────────────────────────────────────────────────
set_secrets() {
  info "Setting base Fly.io secrets…"

  flyctl secrets set \
    APP_ENV=production \
    PORT=8080 \
    CHAIN_MODE=iota \
    SEED_DEMO_DATA=false \
    -a "$API_APP" >/dev/null 2>&1 || warn "Could not set base API secrets (app may not be deployed yet)"

  flyctl secrets set \
    PORT=9000 \
    -a "$EXEC_APP" >/dev/null 2>&1 || warn "Could not set base Executor secrets"

  ok "Base secrets set"
  warn "NOTE: You must still set the following secrets manually with real values:"
  echo "  API (baharimrv-api):"
  echo "    flyctl secrets set DATABASE_URL=\"<neon postgres sslmode=require>\" -a baharimrv-api"
  echo "    flyctl secrets set KMS_KEK=\"<base64 32-byte key>\" -a baharimrv-api"
  echo "    flyctl secrets set OIDC_SIGNING_PRIVATE_KEY=\"<PEM or base64>\" -a baharimrv-api"
  echo "    flyctl secrets set OIDC_SIGNING_KID=\"baharimrv-prod-<year>\" -a baharimrv-api"
  echo "    flyctl secrets set PUBLIC_BASE_URL=\"https://baharimrv-api.fly.dev\" -a baharimrv-api"
  echo "    flyctl secrets set CORS_ORIGINS=\"https://<your-frontend>.fly.dev\" -a baharimrv-api"
  echo "    flyctl secrets set IOTA_EXECUTOR_URL=\"https://baharimrv-executor.fly.dev\" -a baharimrv-api"
  echo "    flyctl secrets set IOTA_EXECUTOR_API_KEY=\"<shared secret>\" -a baharimrv-api"
  echo "    flyctl secrets set IOTA_PACKAGE_ID=\"<0x...>\" -a baharimrv-api"
  echo "    flyctl secrets set IOTA_EXPLORER_URL=\"https://explorer.rebased.iota.org/testnet\" -a baharimrv-api"
  echo "  Executor (baharimrv-executor):"
  echo "    flyctl secrets set API_KEY=\"<same shared secret>\" -a baharimrv-executor"
  echo "    flyctl secrets set SPONSOR_PRIVATE_KEY=\"<iotaprivkey... or base64>\" -a baharimrv-executor"
  echo "    flyctl secrets set IOTA_PACKAGE_ID=\"<0x...>\" -a baharimrv-executor"
  echo "    flyctl secrets set BAHARI_IDENTITY_PACKAGE_ID=\"<0x...>\" -a baharimrv-executor"
  echo "    flyctl secrets set ANCHOR_REGISTRY_OBJECT_ID=\"<0x...>\" -a baharimrv-executor"
  echo "    flyctl secrets set IDENTITY_REGISTRY_OBJECT_ID=\"<0x...>\" -a baharimrv-executor"
  echo "    flyctl secrets set MRV_REGISTRY_OBJECT_ID=\"<0x...>\" -a baharimrv-executor"
  echo "    flyctl secrets set CARBON_CREDIT_REGISTRY_OBJECT_ID=\"<0x...>\" -a baharimrv-executor"
  echo "    flyctl secrets set ADMIN_CAP_OBJECT_ID=\"<0x...>\" -a baharimrv-executor"
}

# ── 5. Deploy ─────────────────────────────────────────────────────────────────
deploy_apps() {
  info "Deploying API to Fly.io…"
  (cd "$BACKEND_DIR" && flyctl deploy --app "$API_APP") || fatal "API deploy failed"
  ok "API deployed"

  info "Deploying Executor to Fly.io…"
  (cd "$EXECUTOR_DIR" && flyctl deploy --app "$EXEC_APP") || fatal "Executor deploy failed"
  ok "Executor deployed"
}

# ── 6. Health checks ──────────────────────────────────────────────────────────
run_health_checks() {
  API_URL="https://$API_APP.fly.dev"
  EXEC_URL="https://$EXEC_APP.fly.dev"

  info "Running health checks…"

  info "API health: $API_URL/api/v1/health"
  if curl -sf "$API_URL/api/v1/health" >/dev/null 2>&1; then
    ok "API is healthy"
  else
    warn "API health check failed — check logs: flyctl logs -a $API_APP"
  fi

  info "Executor health: $EXEC_URL/health"
  if curl -sf "$EXEC_URL/health" >/dev/null 2>&1; then
    ok "Executor is healthy"
  else
    warn "Executor health check failed — check logs: flyctl logs -a $EXEC_APP"
  fi

  info "Verify API → Executor wiring (executor /health is public):"
  echo "  curl $EXEC_URL/health"
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  case "${1:-deploy}" in
    --checks|checks)
      check_prereqs
      run_checks
      ;;
    deploy)
      check_prereqs
      run_checks
      create_apps
      neon_instructions
      set_secrets
      deploy_apps
      run_health_checks
      ;;
    *)
      echo "Usage: $0 [deploy | --checks]"
      exit 1
      ;;
  esac
}

main "$@"
