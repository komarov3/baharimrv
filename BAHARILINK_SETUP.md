# BahariLink Family — Consolidated Setup Requirements

**Covers:** Nanga, BahariMRV, Jahazi, EcoTrack  
**Date:** 2026-11  
**Purpose:** Everything needed to run all four projects locally on a macOS laptop.

---

## 1. System-Wide Prerequisites

### Runtimes

| Tool | Version | Install |
|---|---|---|
| Node.js | 22+ | `brew install node` or nvm |
| npm | 10+ | ships with Node |
| Go | 1.25+ | `brew install go` |
| Java (JDK) | 17+ | `brew install openjdk@17` |
| Python | 3.12+ | `brew install python@3.12` (used by EcoTrack utility scripts only) |
| Kotlin | 2.1 | bundled via Gradle (no separate install needed) |
| Android SDK | API 35 | via Android Studio or `sdkmanager` |

### CLI Tools

| Tool | Purpose | Install |
|---|---|---|
| `iota` CLI | Publish Move packages, interact with IOTA testnet | `brew install iotaledger/tap/iota` (currently 1.19.1 — upgrade to 1.35.1-rc before deploying) |
| `docker` | Postgres, Redis, Caddy for local dev | Docker Desktop for Mac |
| `docker compose` | Orchestrate local stacks | ships with Docker Desktop |
| `gradle` | Android builds | use `gradlew` wrapper per project (no global install) |
| `prisma` CLI | Jahazi database migrations | `npm install -g prisma` (or use `npx prisma`) |

### IOTA Testnet

- Active env: `iota client switch --env testnet`
- RPC: `https://api.testnet.iota.cafe`
- Faucet: `https://faucet.testnet.iota.cafe/gas`
- Explorer: `https://explorer.testnet.iota.cafe`
- Fund your wallet: `iota client faucet`
- Active address check: `iota client active-address`
- Balance check: `iota client balance`

---

## 2. Shared Infrastructure

All four projects use PostgreSQL and Redis. You can run separate databases on one Postgres instance or separate instances.

### PostgreSQL

| Project | Database name | Connection string |
|---|---|---|
| Nanga | `nanga` | `postgres://nanga:nanga@localhost:5432/nanga` |
| BahariMRV | `baharimrv` | `postgresql://baharimrv_user:changeme@localhost:5432/baharimrv?sslmode=require` |
| Jahazi | `baharilink` | `postgresql://baharilink:baharilink@localhost:5432/baharilink` |
| EcoTrack | `ecotrack` | `postgresql://ecotrack_user:CHANGE_ME@localhost:5432/ecotrack?sslmode=disable` |

**Quick start (single Postgres, separate DBs):**
```bash
docker run -d --name baharilink-pg \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:16

# Then create each database:
psql -h localhost -U postgres -c "CREATE DATABASE nanga;"
psql -h localhost -U postgres -c "CREATE DATABASE baharimrv;"
psql -h localhost -U postgres -c "CREATE DATABASE baharilink;"
psql -h localhost -U postgres -c "CREATE DATABASE ecotrack;"
```

Or use Nanga's docker-compose which brings up PostGIS + Redis:
```bash
cd /Users/Komarov/Documents/Nanga
docker compose up -d db redis
```

### Redis

| Project | Redis URL |
|---|---|
| Nanga | `redis://localhost:6379` |
| BahariMRV | (not required — uses in-memory) |
| Jahazi | `redis://localhost:6379` |
| EcoTrack | `redis://localhost:6379/0` |

```bash
docker run -d --name baharilink-redis -p 6379:6379 redis:7-alpine
```

---

## 3. Project Requirements

### 3.1 Nanga

**Location:** `/Users/Komarov/Documents/Nanga/`

**Stack:**
- Backend: Node.js / TypeScript / Express 4 / raw `pg`
- Mobile: React Native 0.74 / Redux Toolkit / SQLite
- Web: Next.js (county dashboard, partner portal)
- Chain: IOTA Rebased Testnet (`fisheries_chain` Move package)
- Legacy chain: Hyperledger Fabric (deprecated, retained during transition)

**Backend dependencies** (from `backend/package.json`):

| Package | Version | Purpose |
|---|---|---|
| express | ^4.22.3 | HTTP server |
| pg | 8.11.5 | PostgreSQL driver (raw queries, no ORM) |
| redis | 4.6.14 | Caching, session store |
| jsonwebtoken | 9.0.2 | JWT auth |
| bcryptjs | ^2.4.3 | Password hashing |
| africastalking | ^0.6.3 | SMS gateway (Safaricom/AT) |
| firebase-admin | ^12.7.0 | FCM push notifications |
| socket.io | ^4.7.5 | Real-time web updates |
| multer | 1.4.5-lts.1 | File upload middleware |
| sharp | ^0.35.5 | Image processing |
| qrcode | ^1.5.4 | QR code generation (proof docs) |
| zod | ^3.25.76 | Input validation |
| uuid | 9.0.1 | UUID generation |
| @aws-sdk/client-s3 | ^3.637.0 | S3 photo storage |
| nodemailer | ^10.0.13 | Email (county digest) |
| dotenv | 16.4.5 | Env var loading |
| helmet | 7.1.0 | HTTP security headers |
| cors | 2.8.5 | CORS middleware |
| express-rate-limit | 7.3.1 | Rate limiting |

Dev: typescript ^5.3.3, jest 29.7.0, ts-jest 29.1.4, ts-node 10.9.2, eslint 8.57.0, supertest 7.0.0

**Mobile dependencies:** React Native 0.74.6, React 18.2.0, Redux Toolkit, React Navigation, Keychain (credential storage), SQLite (offline queue).

**Docker:** `docker-compose.yml` provides PostGIS, Redis, API, county dashboard, partner portal, Caddy TLS.

**Environment variables** (`backend/.env.example`):

```
# Core
NODE_ENV=development
PORT=3000
ALLOWED_ORIGINS=http://localhost:3001
DATABASE_URL=postgres://nanga:nanga@localhost:5432/nanga
PG_POOL_MAX=10
PG_QUERY_LOG=false
REDIS_URL=redis://localhost:6379

# Auth
JWT_SECRET=change-me
JWT_ACCESS_TTL=3600
JAHAZI_OIDC_ISSUER=http://localhost:4000/api/v1/oidc
BAHARIMRV_OIDC_ISSUER=http://localhost:8080/api/v1/oidc

# SMS (Africa's Talking)
AT_API_KEY=
AT_USERNAME=sandbox
AT_SENDER_ID=

# M-Pesa
MPESA_CONSUMER_KEY=
MPESA_CONSUMER_SECRET=
MPESA_PASSKEY=
MPESA_SHORTCODE=
MPESA_CALLBACK_URL=https://api.nanga.co.ke/v1/callbacks/mpesa
MPESA_VERIFY_CALLBACKS=true
MPESA_CALLBACK_TOKEN=

# Storage
S3_BUCKET=nanga-photos
S3_REGION=af-south-1
S3_ENDPOINT=
CDN_BASE_URL=https://cdn.nanga.co.ke

# Push (Firebase FCM)
FIREBASE_SERVICE_ACCOUNT_JSON=

# Email
SMTP_HOST=
SMTP_PORT=587
SMTP_SECURE=false
SMTP_USER=
SMTP_PASS=
SMTP_FROM=digest@nanga.co.ke
DIGEST_EMAILS=

# Ocean data
COPERNICUS_MARINE_TOKEN=
COPERNICUS_SALINITY_URL=

# IOTA (new — from elevation spec)
IOTA_EXECUTOR_URL=
IOTA_EXECUTOR_API_KEY=
IOTA_PACKAGE_ID=
IOTA_EXPLORER_URL=https://explorer.testnet.iota.cafe
GAS_THRESHOLD=100000000
IDENTITY_KEK=

# Legacy Fabric (deprecated)
FABRIC_CHANNEL=fisheries-channel
FABRIC_CONNECTION_PROFILE=
FABRIC_MSP_ID=
```

**Run:**
```bash
cd /Users/Komarov/Documents/Nanga/backend
npm install
npm run migrate          # apply SQL migrations
npm run dev              # start API (ts-node-dev)
npm test                 # run Jest suite (384 tests)

cd /Users/Komarov/Documents/Nanga/mobile
npm install
npx react-native run-android   # mobile app
```

---

### 3.2 BahariMRV

**Location:** `/Users/Komarov/Documents/BahariMRV/`

**Stack:**
- Backend: Go 1.25 / Gin / GORM / PostgreSQL
- Android: Kotlin 2.1 / Compose / Hilt / KSP (compileSdk 35, minSdk 26)
- Chain: IOTA Rebased Testnet (`conservation_chain` Move package)
- Executor: Node.js / TypeScript / Express (serves PTB transactions to IOTA)
- SSO: Authorization-code PKCE with Jahazi

**Backend Go dependencies** (from `backend/go.mod`):

| Module | Version | Purpose |
|---|---|---|
| github.com/gin-gonic/gin | v1.10.0 | HTTP server |
| gorm.io/gorm | v1.25.12 | ORM |
| gorm.io/driver/postgres | v1.5.11 | Postgres driver |
| gorm.io/driver/sqlite | v1.5.7 | SQLite for local/test |
| github.com/golang-jwt/jwt/v5 | v5.2.2 | JWT auth |
| github.com/google/uuid | v1.6.0 | UUID generation |
| golang.org/x/crypto | v0.35.0 | Crypto (Ed25519, AES-GCM) |
| golang.org/x/time | v0.10.0 | Rate limiting |
| gorm.io/datatypes | v1.2.5 | JSONB support for GORM |
| github.com/jung-kurt/gofpdf | v1.16.2 | PDF generation (proof docs) |
| github.com/shopspring/decimal | v1.4.0 | Decimal math (carbon credits) |
| github.com/skip2/go-qrcode | — | QR codes |

**Executor dependencies** (from `executor/package.json`):

| Package | Version | Purpose |
|---|---|---|
| @iota/iota-sdk | ^0.5.0 | IOTA SDK for PTB construction + signing |
| express | ^4.18.3 | HTTP server |

**Android:** Kotlin 2.1, Compose, Hilt 2.54, KSP, compileSdk 35, minSdk 26. Build via `./gradlew assembleDebug` from `android/`.

> **Required secret — `android/app/google-services.json`:** Firebase requires this file at Gradle build time for `assembleDebug` to succeed. It is excluded from version control (`.gitignore`). Obtain it from the Firebase console (project `eco-msck` / BahariMRV Android app) and place it at `android/app/google-services.json` before building. CI/CD pipelines must inject this file as a secret before running the Gradle build step.

**Environment variables** (`backend/.env.example`):

```
APP_ENV=production
PORT=8080
PUBLIC_BASE_URL=https://api.baharimrv.example.com
DATABASE_URL=postgresql://baharimrv_user:changeme@localhost:5432/baharimrv?sslmode=require
KMS_KEK=
SESSION_TTL=1h

# SSO / OIDC
OIDC_SIGNING_PRIVATE_KEY=
OIDC_SIGNING_KID=baharimrv-prod-2026
OIDC_ISSUER=https://api.baharimrv.example.com
CORS_ORIGINS=https://app.baharimrv.example.com,https://admin.baharimrv.example.com
AUTH_RATE_PER_MINUTE=10
PUBLIC_RATE_PER_MINUTE=30
MAX_REQUEST_BYTES=2097152
FEDERATION_CLIENTS=https://auth.jahazi.ke=baharimrv_jahazi_client

# IOTA
CHAIN_MODE=sandbox
IOTA_NETWORK=testnet
IOTA_RPC_URL=https://api.testnet.iota.cafe
IOTA_EXECUTOR_URL=
IOTA_PACKAGE_ID=
IOTA_EXPLORER_URL=https://explorer.rebased.iota.org/testnet

# Services
SATELLITE_MODE=sandbox
REGISTRY_MODE=sandbox
PAYMENT_MODE=sandbox
EVIDENCE_ROOT=data/evidence

# Jahazi integration
JAHAZI_ENABLED=false
JAHAZI_WEBHOOK_URL=
JAHAZI_WEBHOOK_SECRET=

# Seed
SEED_DEMO_DATA=false
DEMO_SEED_PASSWORD=
```

**Run:**
```bash
cd /Users/Komarov/Documents/BahariMRV/backend
go mod download
go run cmd/server/main.go    # start API
go test ./...                # run Go tests

cd /Users/Komarov/Documents/BahariMRV/executor
npm install
npm run dev                   # start executor service

cd /Users/Komarov/Documents/BahariMRV/android
./gradlew assembleDebug        # build APK
```

---

### 3.3 Jahazi

**Location:** `/Users/Komarov/Documents/Turing/mazingira/baharilink/`

**Stack:**
- Backend: Node.js / TypeScript / NestJS 11 / Prisma 5
- Chain: IOTA Rebased Testnet (uses BahariMRV's `conservation_chain` via executor — no separate Move package)
- Database: PostgreSQL (Prisma ORM)

**Backend dependencies** (from `package.json`):

| Package | Version | Purpose |
|---|---|---|
| @nestjs/common | ^11.2.8 | NestJS framework |
| @nestjs/core | ^11.2.8 | NestJS core |
| @nestjs/platform-express | ^11.2.8 | Express adapter |
| @nestjs/config | ^4.0.4 | Config module |
| @nestjs/schedule | ^6.1.3 | Cron jobs (platform DID, gas station) |
| @nestjs/swagger | ^11.4.7 | API docs |
| @prisma/client | ^5.8.0 | Prisma ORM |
| class-transformer | ^0.5.1 | DTO transformation |
| class-validator | ^0.14.1 | DTO validation |
| helmet | ^7.1.0 | HTTP security |
| reflect-metadata | ^0.2.1 | NestJS decorators |
| rxjs | ^7.8.1 | Reactive extensions |
| uuid | ^9.0.0 | UUID generation |

Dev: @nestjs/cli ^11.0.24, prisma ^5.8.0, typescript ^5.3.3, jest 29.7.0, ts-jest 29.4.12

**Environment variables** (`.env`):

```
NODE_ENV=development
PORT=4000
DATABASE_URL=postgresql://baharilink:baharilink@localhost:5432/baharilink
BAHARILINK_WEBHOOK_SECRET=
BAHARILINK_MODULE_KEYS=nanga:dev-nanga-key,jahazi:dev-jahazi-key,ecotrack:dev-ecotrack-key
REDIS_URL=redis://localhost:6379
```

Jahazi also needs BahariMRV's executor URL for IOTA operations (configured via `IOTA_EXECUTOR_URL` in the NestJS config).

**Run:**
```bash
cd /Users/Komarov/Documents/Turing/mazingira/baharilink
npm install
npx prisma generate         # generate Prisma client
npx prisma migrate dev       # apply migrations
npm run start:dev            # start NestJS API
npm test                     # run Jest
```

---

### 3.4 EcoTrack

**Location:** `/Users/Komarov/Documents/Ecotrack-v4 copy 20/`

**Stack:**
- Backend: Go 1.25 / Gin / GORM / PostgreSQL
- Mobile: Kotlin 2.1 / Compose / Hilt 2.55 / KSP (Android 8.8.2)
- Chain: IOTA Rebased Testnet (`plastic_chain` Move package — already deployed)
- Auth: Firebase (phone + email)
- Notifications: Firebase FCM

**Backend Go dependencies** (from `backend-go/go.mod`):

| Module | Version | Purpose |
|---|---|---|
| github.com/gin-gonic/gin | v1.12.0 | HTTP server |
| gorm.io/gorm | v1.31.1 | ORM |
| gorm.io/driver/postgres | v1.6.0 | Postgres driver |
| gorm.io/driver/sqlite | v1.6.0 | SQLite for local/test |
| firebase.google.com/go/v4 | v4.19.0 | Firebase Admin SDK |
| github.com/coming-chat/go-sui/v2 | v2.0.1 | IOTA/Sui SDK (direct RPC) |
| github.com/golang-migrate/migrate/v4 | v4.17.1 | DB migrations |
| github.com/tyler-smith/go-bip39 | v1.1.0 | Mnemonic generation |
| github.com/fardream/go-bcs | v0.2.1 | BCS serialization |
| github.com/gorilla/websocket | v1.5.3 | WebSocket support |
| github.com/joho/godotenv | v1.5.1 | Env var loading |
| github.com/jung-kurt/gofpdf | v1.16.2 | PDF generation |
| github.com/xuri/excelize/v2 | v2.10.1 | Excel export |
| github.com/disintegration/imaging | v1.6.2 | Image processing |
| github.com/chai2010/webp | v1.4.0 | WebP image support |
| google.golang.org/api | v0.273.0 | Google APIs |
| golang.ngrok.com/ngrok | v1.13.0 | Ngrok tunneling |
| github.com/stretchr/testify | v1.11.1 | Test assertions |
| golang.org/x/crypto | v0.49.0 | Crypto |
| golang.org/x/sync | v0.20.0 | Concurrency |
| golang.org/x/time | v0.15.0 | Rate limiting |

**Move package:** `plastic_chain` — already deployed to testnet.

**Environment variables** (`backend-go/.env.example`):

```
# Core
APP_NAME=EcoTrack
APP_ENV=production
APP_PORT=8080
APP_URL=https://api.mirasiprovisions.com

# Database
DATABASE_URL=postgresql://ecotrack_user:CHANGE_ME@localhost:5432/ecotrack?sslmode=disable

# Firebase
FIREBASE_PROJECT_ID=eco-msck
FIREBASE_CLIENT_EMAIL=firebase-adminsdk@eco-msck.iam.gserviceaccount.com
FIREBASE_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----...

# IOTA
IOTA_PACKAGE_ID=0x966f0ea1f7e2946f0a5dbc2ee3b5dab6465276abab16ded5a3dde4ae2abc93f4
IOTA_ADMIN_CAP_ID=0xfb556ed47bfeae853d569b8f65d253e6216ba725db13b5f819f56b4f094a951c
IOTA_NODE_URL=https://api.testnet.iota.cafe

# Auth
JWT_SECRET=CHANGE_ME
JWT_EXPIRY=24h
MNEMONIC_ENCRYPTION_KEY=CHANGE_ME_32_BYTE_HEX

# M-Pesa
M-PESA_CONSUMER_KEY=
M-PESA_CONSUMER_SECRET=
M-PESA_SHORTCODE=174379
M-PESA_PASSKEY=
M-PESA_ENVIRONMENT=sandbox

# Storage
CLOUD_STORAGE_PROVIDER=gcs
GCS_BUCKET_NAME=ecotrack-quality-photos
GCS_PROJECT_ID=eco-msck

# Email
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=noreply@ecotrack.ke
SMTP_PASSWORD=

# Other
REDIS_URL=redis://localhost:6379/0
CORS_ALLOWED_ORIGINS=https://mirasiprovisions.com
LOG_LEVEL=info
SENTRY_DSN=
```

**Run:**
```bash
cd "/Users/Komarov/Documents/Ecotrack-v4 copy 20/backend-go"
go mod download
go run cmd/server/main.go     # or find the main entry point
go test ./...

cd "/Users/Komarov/Documents/Ecotrack-v4 copy 20"
./gradlew assembleDebug        # build Android APK
```

---

## 4. Move Packages on IOTA Testnet

| Package | Location | Status |
|---|---|---|
| `conservation_chain` | `/Users/Komarov/Documents/BahariMRV/chain/` | ✅ Built, tested locally. ⏳ Not deployed to testnet yet. |
| `fisheries_chain` | `/Users/Komarov/Documents/Nanga/chain/` | ✅ Built, tested locally (11 tests pass). ⏳ Not deployed to testnet yet. |
| `plastic_chain` | `/Users/Komarov/Documents/Ecotrack-v4 copy 20/plastic_chain/` | ✅ Deployed to testnet (`0x966f...`) |

### Deploying `conservation_chain` and `fisheries_chain`

Both use the same IOTA framework dependency and publish independently:

```bash
# 0. Upgrade iota CLI first (1.19.1 → 1.35.1-rc)
brew upgrade iota

# 1. Swap local dep to git pin in both Move.toml files
# In both Move.toml, replace:
#   Iota = { local = "/tmp/iota-framework-v1.19.1/..." }
# with:
#   Iota = { git = "https://github.com/iotaledger/iota.git",
#            subdir = "crates/iota-framework/packages/iota-framework",
#            rev = "v1.19.1" }

# 2. Publish conservation_chain (BahariMRV)
cd /Users/Komarov/Documents/BahariMRV/chain
iota move build
iota client publish --gas-budget 200000000
# Note the package ID from output → set as IOTA_PACKAGE_ID in BahariMRV .env

# 3. Publish fisheries_chain (Nanga)
cd /Users/Komarov/Documents/Nanga/chain
iota move build
iota client publish --gas-budget 200000000
# Note the package ID from output → set as IOTA_PACKAGE_ID in Nanga .env
```

Both share the same wallet address (`0xbf07c...`) and the same testnet instance. No conflicts.

---

## 5. Inter-Project Integration Points

```
                    ┌──────────┐
                    │  Jahazi  │ (NestJS, port 4000)
                    │  tourism │
                    └────┬─────┘
                         │ SSO (OIDC PKCE)
                         │ Webhook (restoration claims)
                    ┌────▼─────┐
  ┌─────────┐       │ BahariMRV│ (Go/Gin, port 8080)
  │ EcoTrack│       │   MRV    │
  │ plastic │       │ verifier │
  │ trace   │       └────┬─────┘
  └─────────┘            │ SSO (OIDC PKCE)
                         │ Webhook (MRV link signing)
                    ┌────▼─────┐
                    │  Nanga   │ (Node/Express, port 3000)
                    │ fishery  │
                    └──────────┘
```

### SSO / OIDC
- Jahazi is the identity provider (port 4000)
- BahariMRV and Nanga are relying parties (consume Jahazi's OIDC issuer)
- Nanga env: `JAHAZI_OIDC_ISSUER=http://localhost:4000/api/v1/oidc`
- BahariMRV env: `FEDERATION_CLIENTS=https://auth.jahazi.ke=baharimrv_jahazi_client`

### Webhooks
- BahariMRV → Jahazi: restoration claim notifications (`JAHAZI_WEBHOOK_URL`, `JAHAZI_WEBHOOK_SECRET`)
- Jahazi → BahariMRV: MRV link signing

### IOTA Executor
- BahariMRV owns the executor service (`/executor/`, Node.js/Express)
- Nanga points its `IOTA_EXECUTOR_URL` at the same executor (or a second instance)
- Jahazi also uses the BahariMRV executor for `conservation_chain` operations
- EcoTrack uses direct RPC (`go-sui` SDK), not the executor

### Shared module keys
- Jahazi configures cross-project module access: `BAHARILINK_MODULE_KEYS=nanga:dev-nanga-key,jahazi:dev-jahazi-key,ecotrack:dev-ecotrack-key`

---

## 6. Quick Start — All Four Projects

```bash
# 1. Start infrastructure
docker run -d --name baharilink-pg -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:16
docker run -d --name baharilink-redis -p 6379:6379 redis:7-alpine

# Create databases
psql -h localhost -U postgres <<EOF
CREATE DATABASE nanga;
CREATE DATABASE baharimrv;
CREATE DATABASE baharilink;
CREATE DATABASE ecotrack;
EOF

# 2. Start BahariMRV (Go backend + executor)
cd /Users/Komarov/Documents/BahariMRV/backend && go run cmd/server/main.go &
cd /Users/Komarov/Documents/BahariMRV/executor && npm install && npm run dev &

# 3. Start Jahazi (NestJS)
cd /Users/Komarov/Documents/Turing/mazingira/baharilink
npm install && npx prisma migrate dev && npm run start:dev &

# 4. Start Nanga (Express)
cd /Users/Komarov/Documents/Nanga/backend
npm install && npm run migrate && npm run dev &

# 5. Start EcoTrack (Go backend)
cd "/Users/Komarov/Documents/Ecotrack-v4 copy 20/backend-go"
go mod download && go run cmd/server/main.go &

# 6. Deploy Move packages to testnet (one-time)
brew upgrade iota
cd /Users/Komarov/Documents/BahariMRV/chain && iota client publish --gas-budget 200000000
cd /Users/Komarov/Documents/Nanga/chain && iota client publish --gas-budget 200000000
```

---

## 7. Port Assignments

| Project | Port | Service |
|---|---|---|
| Nanga | 3000 | Backend API |
| Nanga | 3001 | County dashboard (Next.js) |
| Nanga | 3002 | Partner portal (Next.js) |
| BahariMRV | 8080 | Backend API (Go/Gin) |
| BahariMRV | — | Executor (port from executor config) |
| Jahazi | 4000 | Backend API (NestJS) |
| EcoTrack | 8080 | Backend API (Go/Gin) — change if running alongside BahariMRV |

**Conflict:** BahariMRV and EcoTrack both default to port 8080. Change one:
```bash
# EcoTrack: set APP_PORT=8081 in .env
# or BahariMRV: set PORT=8082 in .env
```

---

## 8. Current Status Summary

| Project | Code | Tests | IOTA Deployed | Notes |
|---|---|---|---|---|
| Nanga | ✅ IOTA elevation complete (10 items) | 384 backend, 146 mobile | ⏳ `fisheries_chain` not yet on testnet | Needs `brew upgrade iota` + env vars |
| BahariMRV | ✅ Production-ready with SSO | Go tests | ⏳ `conservation_chain` not yet on testnet | Executor ready, needs package deploy |
| Jahazi | ✅ Phases 1–3 complete | NestJS tests | Uses BahariMRV executor | Depends on BahariMRV deployment |
| EcoTrack | ✅ Fully deployed | Go tests | ✅ `plastic_chain` on testnet (`0x966f...`) | Live, using direct RPC not executor |
