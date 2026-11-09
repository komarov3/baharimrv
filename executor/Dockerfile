# syntax=docker/dockerfile:1
# BahariMRV Executor (Node.js) — Fly.io production image
# Builds the TypeScript source and runs the compiled Express service.

# ── Build stage ───────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

# Install dependencies (executor is a standalone workspace)
COPY package.json ./
RUN npm install

# Compile TypeScript to dist/
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# ── Run stage ─────────────────────────────────────────────────────────────────
FROM node:22-alpine

WORKDIR /app

# Copy only production artifacts needed to run
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY package.json ./

ENV NODE_ENV=production \
    PORT=9000

EXPOSE 9000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:9000/health || exit 1

CMD ["node", "dist/index.js"]