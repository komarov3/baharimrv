# syntax=docker/dockerfile:1
# BahariMRV API (Go) — Fly.io production image
# Builds a statically-linked Go binary and runs it in a minimal Alpine image.

# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# CGO is needed by some indirect deps (go-sqlite3) but we target Postgres in prod;
# we still install the toolchain so `go mod download` succeeds for all modules.
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked, stripped binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /baharimrv-server ./cmd/server

# ── Run stage ─────────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget && \
    addgroup -S baharimrv && adduser -S baharimrv -G baharimrv && \
    mkdir -p /data/evidence && chown -R baharimrv:baharimrv /data

WORKDIR /

COPY --from=builder /baharimrv-server /baharimrv-server

# Non-root runtime user
USER baharimrv

EXPOSE 8080

# Production defaults (overridden by Fly.io secrets / env)
ENV APP_ENV=production \
    PORT=8080 \
    CHAIN_MODE=iota \
    SEED_DEMO_DATA=false \
    EVIDENCE_ROOT=/data/evidence

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/v1/health || exit 1

CMD ["/baharimrv-server"]