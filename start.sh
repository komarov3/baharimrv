#!/usr/bin/env zsh
# BahariMRV — one-stop dev server startup
# Usage: ./start.sh [--no-seed] [--rebuild]
#
#   --no-seed   skip demo data seeding (useful after first run)
#   --rebuild   force recompile even if binary is up-to-date

set -euo pipefail

SCRIPT_DIR="${0:A:h}"
BACKEND_DIR="$SCRIPT_DIR/backend"
BINARY="$BACKEND_DIR/bin/mrv-server"

# ── colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

info()    { print -P "%F{cyan}▸ $*%f" }
success() { print -P "%F{green}✔ $*%f" }
warn()    { print -P "%F{yellow}⚠ $*%f" }
fatal()   { print -P "%F{red}✖ $*%f"; exit 1 }

# ── args ─────────────────────────────────────────────────────────────────────
SEED=true
REBUILD=false
for arg in "$@"; do
  case $arg in
    --no-seed)  SEED=false ;;
    --rebuild)  REBUILD=true ;;
    *)          fatal "Unknown argument: $arg" ;;
  esac
done

print ""
print "${BOLD}╔══════════════════════════════════════╗${RESET}"
print "${BOLD}║        BahariMRV Dev Server          ║${RESET}"
print "${BOLD}╚══════════════════════════════════════╝${RESET}"
print ""

# ── check .env ────────────────────────────────────────────────────────────────
if [[ ! -f "$BACKEND_DIR/.env" ]]; then
  fatal ".env not found at $BACKEND_DIR/.env — copy .env.example and fill in values"
fi
success ".env found"

# ── build ─────────────────────────────────────────────────────────────────────
NEEDS_BUILD=false
if [[ "$REBUILD" == true ]]; then
  NEEDS_BUILD=true
elif [[ ! -f "$BINARY" ]]; then
  NEEDS_BUILD=true
else
  # rebuild if any .go file is newer than the binary
  if [[ -n "$(find "$BACKEND_DIR" -name '*.go' -newer "$BINARY" 2>/dev/null)" ]]; then
    NEEDS_BUILD=true
  fi
fi

if [[ "$NEEDS_BUILD" == true ]]; then
  info "Building backend…"
  (cd "$BACKEND_DIR" && go build -o bin/mrv-server ./cmd/server) \
    || fatal "Build failed — run 'cd backend && go build ./cmd/server' for details"
  success "Build complete → bin/mrv-server"
else
  success "Binary up-to-date (use --rebuild to force)"
fi

# ── seed flag ─────────────────────────────────────────────────────────────────
# Override SEED_DEMO_DATA from .env if --no-seed was passed
SEED_OVERRIDE=""
if [[ "$SEED" == false ]]; then
  SEED_OVERRIDE="SEED_DEMO_DATA=false"
  warn "Demo seeding disabled (--no-seed)"
else
  info "Demo seeding enabled — Mikoko Park + Gazi Bay projects will be created on first run"
fi

# ── evidence dir ──────────────────────────────────────────────────────────────
EVIDENCE_DIR="$BACKEND_DIR/data/evidence"
if [[ ! -d "$EVIDENCE_DIR" ]]; then
  mkdir -p "$EVIDENCE_DIR"
  info "Created evidence storage dir: $EVIDENCE_DIR"
fi

# ── port ──────────────────────────────────────────────────────────────────────
PORT=$(grep -E '^PORT=' "$BACKEND_DIR/.env" | head -1 | cut -d= -f2 | tr -d '[:space:]')
PORT="${PORT:-8080}"

# check if port is already in use
if lsof -ti ":$PORT" &>/dev/null; then
  warn "Port $PORT is already in use — killing existing process…"
  lsof -ti ":$PORT" | xargs kill -9 2>/dev/null || true
  sleep 1
fi

# ── start ─────────────────────────────────────────────────────────────────────
print ""
info "Starting BahariMRV backend on port ${BOLD}$PORT${RESET}…"
print ""
print "  API:      ${CYAN}http://localhost:$PORT/api/v1/health${RESET}"
print "  Admin:    ${CYAN}http://localhost:$PORT${RESET}"
print "  DB:       ${CYAN}$BACKEND_DIR/baharimrv-local.db${RESET}"
print ""
print "  Demo manager:   manager@demo.baharimrv.local"
print "  Gazi Bay mgr:   hamisi.manager@demo.baharimrv.local"
print "  Password:       (see DEMO_SEED_PASSWORD in backend/.env)"
print ""
print "  Press ${BOLD}Ctrl-C${RESET} to stop"
print ""

# run from backend dir so relative paths (DB, evidence) resolve correctly
cd "$BACKEND_DIR"
if [[ -n "$SEED_OVERRIDE" ]]; then
  env $(cat .env | grep -v '^#' | grep -v '^$' | xargs) $SEED_OVERRIDE "$BINARY"
else
  env $(cat .env | grep -v '^#' | grep -v '^$' | xargs) "$BINARY"
fi
