#!/usr/bin/env zsh
# BahariMRV — one-stop dev server startup
#
# Starts three services in order:
#   1. IOTA Executor  (Node.js  — port 9000)
#   2. BahariMRV API  (Go       — port 8080)
#   3. ngrok tunnel   (HTTPS    — rattly-superacutely-angelita.ngrok-free.dev → :8080)
#
# Usage:
#   ./start.sh              # full stack, seed demo data on first run
#   ./start.sh --no-seed    # skip demo seeding (use after first run)
#   ./start.sh --rebuild    # force recompile Go binary
#   ./start.sh --no-chain   # skip executor + ngrok (offline / sandbox mode)

set -euo pipefail

SCRIPT_DIR="${0:A:h}"
BACKEND_DIR="$SCRIPT_DIR/backend"
EXECUTOR_DIR="$SCRIPT_DIR/executor"
BINARY="$BACKEND_DIR/bin/mrv-server"
NGROK_DOMAIN="rattly-superacutely-angelita.ngrok-free.dev"
BACKEND_PORT=8080
EXECUTOR_PORT=9000
LOG_DIR="/tmp/baharimrv-logs"

# ── colours ──────────────────────────────────────────────────────────────────
info()    { print -P "%F{cyan}▸ $*%f" }
success() { print -P "%F{green}✔ $*%f" }
warn()    { print -P "%F{yellow}⚠ $*%f" }
fatal()   { print -P "%F{red}✖ $*%f"; exit 1 }
header()  { print -P "\n%B%F{white}$*%f%b" }

# ── args ─────────────────────────────────────────────────────────────────────
SEED=true
REBUILD=false
WITH_CHAIN=true
for arg in "$@"; do
  case $arg in
    --no-seed)   SEED=false ;;
    --rebuild)   REBUILD=true ;;
    --no-chain)  WITH_CHAIN=false ;;
    *)           fatal "Unknown argument: $arg (valid: --no-seed --rebuild --no-chain)" ;;
  esac
done

mkdir -p "$LOG_DIR"

# ── preamble ──────────────────────────────────────────────────────────────────
print ""
print "╔══════════════════════════════════════════╗"
print "║       BahariMRV — Dev Stack              ║"
print "╚══════════════════════════════════════════╝"

# ── cleanup on exit ──────────────────────────────────────────────────────────
PIDS=()
cleanup() {
  print ""
  info "Shutting down all services…"
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  # also kill by port in case of orphans
  lsof -ti ":$BACKEND_PORT"  2>/dev/null | xargs kill -9 2>/dev/null || true
  lsof -ti ":$EXECUTOR_PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
  pkill -f "ngrok.*$NGROK_DOMAIN" 2>/dev/null || true
  success "All services stopped."
}
trap cleanup INT TERM EXIT

# ═══════════════════════════════════════════════════════════════════════════════
# 1. CHECKS
# ═══════════════════════════════════════════════════════════════════════════════
header "── Preflight checks ─────────────────────────"

# .env
[[ -f "$BACKEND_DIR/.env" ]] || fatal "backend/.env not found — copy .env.example and fill in values"
success "backend/.env found"

# node
command -v node &>/dev/null || fatal "node not found — install Node.js 18+"
success "node $(node --version)"

# ngrok
if [[ "$WITH_CHAIN" == true ]]; then
  command -v ngrok &>/dev/null || fatal "ngrok not found — install via: brew install ngrok/ngrok/ngrok"
  success "ngrok $(ngrok version 2>&1 | head -1 | awk '{print $NF}')"
fi

# executor node_modules
if [[ "$WITH_CHAIN" == true ]]; then
  if [[ ! -d "$EXECUTOR_DIR/node_modules" ]]; then
    info "Installing executor dependencies…"
    (cd "$EXECUTOR_DIR" && npm install --silent) || fatal "npm install failed in executor/"
    success "Executor deps installed"
  else
    success "Executor node_modules present"
  fi

  # tsx (used for dev mode)
  EXECUTOR_TSX="$EXECUTOR_DIR/node_modules/.bin/tsx"
  if [[ ! -f "$EXECUTOR_TSX" ]]; then
    info "Installing tsx…"
    (cd "$EXECUTOR_DIR" && npm install --save-dev tsx --silent) \
      || fatal "Failed to install tsx"
  fi
  success "tsx available"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# 2. BUILD GO BINARY
# ═══════════════════════════════════════════════════════════════════════════════
header "── Backend build ────────────────────────────"

NEEDS_BUILD=false
if [[ "$REBUILD" == true ]] || [[ ! -f "$BINARY" ]]; then
  NEEDS_BUILD=true
elif [[ -n "$(find "$BACKEND_DIR" -name '*.go' -newer "$BINARY" 2>/dev/null)" ]]; then
  info "Source changes detected"
  NEEDS_BUILD=true
fi

if [[ "$NEEDS_BUILD" == true ]]; then
  info "Compiling Go backend…"
  (cd "$BACKEND_DIR" && go build -o bin/mrv-server ./cmd/server) \
    || fatal "Go build failed — run 'cd backend && go build ./cmd/server' for details"
  success "Binary compiled → backend/bin/mrv-server"
else
  success "Binary up-to-date (pass --rebuild to force)"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# 3. KILL STALE PROCESSES ON SAME PORTS
# ═══════════════════════════════════════════════════════════════════════════════
header "── Port cleanup ──────────────────────────────"

for port in $BACKEND_PORT $EXECUTOR_PORT; do
  if lsof -ti ":$port" &>/dev/null; then
    warn "Port $port in use — killing stale process…"
    lsof -ti ":$port" | xargs kill -9 2>/dev/null || true
    sleep 0.5
  fi
done
success "Ports $BACKEND_PORT and $EXECUTOR_PORT are free"

# ═══════════════════════════════════════════════════════════════════════════════
# 4. START EXECUTOR (Node.js IOTA transaction service)
# ═══════════════════════════════════════════════════════════════════════════════
if [[ "$WITH_CHAIN" == true ]]; then
  header "── Executor (port $EXECUTOR_PORT) ─────────────────────"

  [[ -f "$EXECUTOR_DIR/.env" ]] || fatal "executor/.env not found — check executor/ directory"

  EXECUTOR_LOG="$LOG_DIR/executor.log"
  (cd "$EXECUTOR_DIR" && node_modules/.bin/tsx src/index.ts >> "$EXECUTOR_LOG" 2>&1) &
  EXECUTOR_PID=$!
  PIDS+=($EXECUTOR_PID)

  # wait up to 8 s for executor to be ready
  info "Waiting for executor on :$EXECUTOR_PORT…"
  for i in {1..16}; do
    if curl -sf "http://localhost:$EXECUTOR_PORT/health" &>/dev/null \
       || curl -sf "http://localhost:$EXECUTOR_PORT/" &>/dev/null; then
      break
    fi
    if ! kill -0 $EXECUTOR_PID 2>/dev/null; then
      fatal "Executor exited early — check $EXECUTOR_LOG"
    fi
    sleep 0.5
  done
  success "Executor running (PID $EXECUTOR_PID) — logs: $EXECUTOR_LOG"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# 5. START BACKEND (Go API)
# ═══════════════════════════════════════════════════════════════════════════════
header "── Backend API (port $BACKEND_PORT) ──────────────────────"

EVIDENCE_DIR="$BACKEND_DIR/data/evidence"
[[ -d "$EVIDENCE_DIR" ]] || mkdir -p "$EVIDENCE_DIR"

# build env from .env file, optionally override SEED_DEMO_DATA
ENV_VARS=($(grep -v '^#' "$BACKEND_DIR/.env" | grep -v '^$'))
[[ "$SEED" == false ]] && ENV_VARS+=("SEED_DEMO_DATA=false")

BACKEND_LOG="$LOG_DIR/backend.log"
(cd "$BACKEND_DIR" && env "${ENV_VARS[@]}" "$BINARY" >> "$BACKEND_LOG" 2>&1) &
BACKEND_PID=$!
PIDS+=($BACKEND_PID)

# wait up to 15 s for the backend health endpoint
info "Waiting for backend on :$BACKEND_PORT…"
for i in {1..30}; do
  if curl -sf "http://localhost:$BACKEND_PORT/api/v1/health" &>/dev/null; then
    break
  fi
  if ! kill -0 $BACKEND_PID 2>/dev/null; then
    fatal "Backend exited early — check $BACKEND_LOG"
  fi
  sleep 0.5
done
success "Backend running (PID $BACKEND_PID) — logs: $BACKEND_LOG"

# ═══════════════════════════════════════════════════════════════════════════════
# 6. START NGROK TUNNEL
# ═══════════════════════════════════════════════════════════════════════════════
if [[ "$WITH_CHAIN" == true ]]; then
  header "── ngrok tunnel ──────────────────────────────"

  NGROK_LOG="$LOG_DIR/ngrok.log"
  ngrok http "$BACKEND_PORT" \
    --domain="$NGROK_DOMAIN" \
    --log=stdout \
    --log-format=json \
    > "$NGROK_LOG" 2>&1 &
  NGROK_PID=$!
  PIDS+=($NGROK_PID)

  # wait up to 8 s for ngrok to establish
  info "Waiting for ngrok tunnel…"
  for i in {1..16}; do
    if curl -sf "https://$NGROK_DOMAIN/api/v1/health" &>/dev/null; then
      break
    fi
    if ! kill -0 $NGROK_PID 2>/dev/null; then
      fatal "ngrok exited early — check $NGROK_LOG"
    fi
    sleep 0.5
  done
  success "ngrok tunnel active (PID $NGROK_PID) — logs: $NGROK_LOG"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# 7. READY
# ═══════════════════════════════════════════════════════════════════════════════
print ""
print "╔══════════════════════════════════════════════════════════════╗"
print "║  ✔  BahariMRV dev stack is running                          ║"
print "╠══════════════════════════════════════════════════════════════╣"
printf "║  Local API    http://localhost:%-31s║\n" "$BACKEND_PORT"
if [[ "$WITH_CHAIN" == true ]]; then
printf "║  Public URL   https://%-39s║\n" "$NGROK_DOMAIN"
printf "║  Executor     http://localhost:%-31s║\n" "$EXECUTOR_PORT"
fi
print "╠══════════════════════════════════════════════════════════════╣"
print "║  Demo accounts (password in backend/.env DEMO_SEED_PASSWORD) ║"
print "║    Halima Rasid    manager@demo.baharimrv.local              ║"
print "║    Hamisi Mwangi   hamisi.manager@demo.baharimrv.local       ║"
print "╠══════════════════════════════════════════════════════════════╣"
print "║  Logs: /tmp/baharimrv-logs/                                  ║"
print "║    backend.log  executor.log  ngrok.log                      ║"
print "╠══════════════════════════════════════════════════════════════╣"
print "║  Press Ctrl-C to stop all services                           ║"
print "╚══════════════════════════════════════════════════════════════╝"
print ""

# ── tail logs to show activity ─────────────────────────────────────────────
tail -f "$LOG_DIR/backend.log" &
TAIL_PID=$!
PIDS+=($TAIL_PID)

# wait for any service to die unexpectedly
WATCH_PIDS=($BACKEND_PID)
[[ "$WITH_CHAIN" == true ]] && WATCH_PIDS+=($EXECUTOR_PID $NGROK_PID)
while true; do
  for pid in "${WATCH_PIDS[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      warn "A service (PID $pid) exited unexpectedly — check logs in $LOG_DIR"
    fi
  done
  sleep 5
done
