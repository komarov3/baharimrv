# BahariMRV Development Setup Guide

**Last Updated:** December 2024  
**Target:** Local development environment with demo data

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Prerequisites](#prerequisites)
3. [Backend Setup](#backend-setup)
4. [Android Setup](#android-setup)
5. [Running with Demo Data](#running-with-demo-data)
6. [Testing SSO Locally](#testing-sso-locally)
7. [Development Workflow](#development-workflow)
8. [Troubleshooting](#troubleshooting)

---

## Quick Start

Get up and running in 5 minutes:

```bash
# 1. Clone and navigate
git clone https://github.com/baharilink/baharimrv.git
cd baharimrv

# 2. Start backend with demo data
cd backend
cp .env.example .env
# Edit .env: Set SEED_DEMO_DATA=true and DEMO_SEED_PASSWORD=demo123
go run cmd/server/main.go

# 3. In another terminal, run Android app
cd ../android
cp local.properties.example local.properties
# Edit local.properties: Set BAHARIMRV_API_BASE_URL=http://10.0.2.2:8080
# Open in Android Studio and run
```

---

## Prerequisites

### Required Software

**Backend:**
- Go 1.21 or later ([install](https://go.dev/doc/install))
- PostgreSQL 14+ (optional, SQLite used by default for development)
- Git

**Android:**
- Android Studio Hedgehog (2023.1.1) or later
- JDK 17
- Android SDK with API level 35
- Android device or emulator (API 26+)

**Tools:**
- curl or Postman (API testing)
- ngrok or localhost.run (for testing OAuth callbacks)

### System Setup

**macOS:**

```bash
# Install Homebrew if needed
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install Go
brew install go

# Install PostgreSQL (optional)
brew install postgresql@14
brew services start postgresql@14
```

**Ubuntu/Debian:**

```bash
# Install Go
sudo add-apt-repository ppa:longsleep/golang-backports
sudo apt update
sudo apt install golang-go

# Install PostgreSQL (optional)
sudo apt install postgresql postgresql-contrib

# Install JDK 17
sudo apt install openjdk-17-jdk
```

**Windows:**

- Install [Go for Windows](https://go.dev/dl/)
- Install [Android Studio](https://developer.android.com/studio)
- Install [PostgreSQL for Windows](https://www.postgresql.org/download/windows/) (optional)

---

## Backend Setup

### 1. Clone Repository

```bash
git clone https://github.com/baharilink/baharimrv.git
cd baharimrv/backend
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` for local development:

```bash
# Development defaults
APP_ENV=development
PORT=8080
PUBLIC_BASE_URL=http://localhost:8080

# SQLite for development (no PostgreSQL needed)
DATABASE_URL=file:baharimrv-local.db?_busy_timeout=5000&_journal_mode=WAL

# CORS for local web clients
CORS_ORIGINS=http://localhost:3000,http://localhost:5173

# Sandbox modes (no real integrations needed)
CHAIN_MODE=sandbox
SATELLITE_MODE=sandbox
REGISTRY_MODE=sandbox
PAYMENT_MODE=sandbox

# Demo data
SEED_DEMO_DATA=true
DEMO_SEED_PASSWORD=demo123

# KEK auto-generated in development if not set
# No OIDC signing key required for local development
```

### 4. Run the Server

```bash
go run cmd/server/main.go
```

You should see:

```
Starting BahariMRV API server...
Environment: development
Database: SQLite (file:baharimrv-local.db)
Seeding demo data...
Demo accounts created (password: demo123):
  - manager@demo.baharimrv.local (Manager)
  - amina@demo.baharimrv.local (Monitor)
  - juma@demo.baharimrv.local (Monitor)
  - verifier@demo.baharimrv.local (Verifier)
  - buyer@demo.baharimrv.local (Buyer)
  - finance@demo.baharimrv.local (Finance)
  - admin@demo.baharimrv.local (Admin)
Server listening on :8080
```

### 5. Test the API

```bash
# Health check
curl http://localhost:8080/health

# Login with demo account
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"manager@demo.baharimrv.local","password":"demo123"}'
```

---

## Android Setup

### 1. Open Project

```bash
cd android
```

Open Android Studio → Open → Select the `android` folder.

### 2. Configure local.properties

```bash
cp local.properties.example local.properties
```

Edit `local.properties`:

```properties
# For Android Emulator (10.0.2.2 maps to host's localhost)
BAHARIMRV_API_BASE_URL=http://10.0.2.2:8080

# For Physical Device on same network
# BAHARIMRV_API_BASE_URL=http://192.168.1.100:8080

# SSO (leave empty for development, native login only)
JAHAZI_ISSUER_URL=
JAHAZI_CLIENT_ID=
JAHAZI_REDIRECT_URI=

# Google Maps (get free key from Google Cloud Console)
GOOGLE_MAPS_API_KEY=AIza...your-key-here
```

**Finding your local IP (for physical device testing):**

```bash
# macOS/Linux
ifconfig | grep "inet " | grep -v 127.0.0.1

# Windows
ipconfig | findstr IPv4
```

### 3. Sync Gradle

Android Studio will prompt you to sync. Click "Sync Now".

If there are issues:

```bash
./gradlew clean
./gradlew build --refresh-dependencies
```

### 4. Run the App

1. Create an Android emulator (if you don't have one):
   - Tools → Device Manager → Create Device
   - Choose Pixel 5 or similar
   - System Image: API 34 (UpsideDownCake)

2. Click Run (green play button) or press `Shift+F10`

3. Login with demo account:
   - Email: `manager@demo.baharimrv.local`
   - Password: `demo123`

---

## Running with Demo Data

### What Demo Data Includes

When `SEED_DEMO_DATA=true`, the backend creates:

**Users (all password: `demo123`):**
- `manager@demo.baharimrv.local` - Project Manager
- `amina@demo.baharimrv.local` - Field Monitor (Plot 1)
- `juma@demo.baharimrv.local` - Field Monitor (Plot 2)
- `verifier@demo.baharimrv.local` - Independent Verifier
- `buyer@demo.baharimrv.local` - Carbon Credit Buyer
- `finance@demo.baharimrv.local` - Finance Operator
- `admin@demo.baharimrv.local` - Platform Administrator

**Project:**
- "Mikoko Park mangrove restoration (demo)"
- 2 sites, 2 plots with GPS coordinates (Kenyan coast)
- Demo methodology with survival threshold

**Monitoring Records:**
- Plot 1: 388/420 alive, Rhizophora mucronata
- Plot 2: 302/310 alive, Avicennia marina
- Photo evidence (placeholder images)
- GPS coordinates with realistic accuracy

**Activities:**
- Plot 1: Completed and awaiting verification
- Plot 2: Still under monitoring

**Marketplace:**
- 1 sandbox credit batch (100 tCO2e, unverified)
- Listed at 35 KES per tonne

**Wallet:**
- 2 tourism revenue imports (27,740.50 KES total)

### Exploring Demo Data

**As Manager:**
1. Login: `manager@demo.baharimrv.local` / `demo123`
2. View dashboard with project metrics
3. Navigate to Monitor tab
4. See monitoring records from field staff
5. Go to Credits tab → Verification section
6. Review the completed activity
7. Check Wallet for tourism revenue

**As Monitor:**
1. Login: `amina@demo.baharimrv.local` / `demo123`
2. View assigned plots
3. Capture new monitoring record
4. Test offline capture (enable Airplane mode)
5. Test sync when back online

**As Verifier:**
1. Login: `verifier@demo.baharimrv.local` / `demo123`
2. Review evidence packages
3. Approve or request corrections
4. Sign attestation

**As Buyer:**
1. Login: `buyer@demo.baharimrv.local` / `demo123`
2. Browse marketplace listings
3. Purchase credits
4. Retire credits for offset claims
5. Download retirement certificate

### Resetting Demo Data

```bash
# Stop the backend
# Delete the database
rm backend/baharimrv-local.db

# Restart the backend
cd backend
go run cmd/server/main.go
# Demo data will be re-seeded
```

---

## Testing SSO Locally

OAuth redirects require publicly reachable URLs, which localhost isn't. Use a tunneling service:

### Option 1: ngrok (Recommended)

1. Install ngrok: [https://ngrok.com/download](https://ngrok.com/download)

2. Start ngrok tunnel:

```bash
ngrok http 8080
```

You'll get a URL like: `https://abc123.ngrok.io`

**Note:** The Android app is configured to use HTTP/1.1 for ngrok compatibility (some ngrok versions have HTTP/2 framing issues). This is set in `AppModule.kt` with `.protocols(listOf(Protocol.HTTP_1_1))`.

3. Update backend `.env`:

```bash
PUBLIC_BASE_URL=https://abc123.ngrok.io
OIDC_ISSUER=https://abc123.ngrok.io
```

4. Restart backend

5. Update Android `local.properties`:

```properties
BAHARIMRV_API_BASE_URL=https://abc123.ngrok.io
JAHAZI_ISSUER_URL=https://abc123.ngrok.io  # Or real Jahazi URL
JAHAZI_CLIENT_ID=dev_client_id
JAHAZI_REDIRECT_URI=baharimrv://auth/callback
```

6. Register redirect URI with Jahazi (or your test OAuth server)

7. Test SSO flow in the app

### Option 2: localhost.run

```bash
ssh -R 80:localhost:8080 localhost.run
```

Similar process as ngrok.

### Mock SSO Server

For testing without real Jahazi integration:

```bash
# Use a mock OAuth2 server
docker run -p 9000:9000 ghcr.io/oauth2-proxy/mockoidc:latest
```

Configure to use `http://localhost:9000` as issuer.

---

## Development Workflow

### Backend Development

**Project Structure:**

```
backend/
├── cmd/server/          # Main entry point
├── internal/
│   ├── auth/            # Authentication & federation
│   ├── identity/        # IOTA identity provisioning
│   ├── mrv/             # Monitoring, Reporting, Verification
│   ├── models/          # Database models
│   ├── config/          # Configuration loading
│   └── seed/            # Demo data seeding
└── migrations/          # Database migrations (if separate)
```

**Hot Reload:**

```bash
# Install air for hot reload
go install github.com/cosmtrek/air@latest

# Run with hot reload
air
```

**Testing:**

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test ./internal/auth -v
```

### Android Development

**Project Structure:**

```
android/app/src/main/java/ke/co/baharilink/baharimrv/
├── data/
│   ├── local/           # Room database, DataStore
│   ├── remote/          # Retrofit API
│   └── repository/      # Repository layer
├── domain/              # Business logic, models
├── ui/
│   ├── auth/            # Login, registration screens
│   ├── home/            # Dashboard
│   ├── monitor/         # Field capture
│   ├── credits/         # Marketplace, verification
│   ├── wallet/          # Financial tracking
│   └── profile/         # User profile, settings
└── sync/                # WorkManager sync
```

**Run Tests:**

```bash
# Unit tests
./gradlew test

# Connected tests (requires device/emulator)
./gradlew connectedAndroidTest

# Specific test
./gradlew test --tests SyncMappingTest
```

**Lint and Format:**

```bash
# Kotlin lint
./gradlew lintDebug

# Format code (if ktlint configured)
./gradlew ktlintFormat
```

### Database Inspection

**SQLite (development):**

```bash
# Install sqlite3
brew install sqlite  # macOS
sudo apt install sqlite3  # Ubuntu

# Open database
sqlite3 backend/baharimrv-local.db

# List tables
.tables

# Query users
SELECT id, email, role, status FROM users;

# Exit
.quit
```

**Room Database (Android):**

```bash
# Pull from emulator
adb exec-out run-as ke.co.baharilink.baharimrv cat \
  /data/data/ke.co.baharilink.baharimrv/databases/baharimrv.db \
  > baharimrv-android.db

# Open with sqlite3
sqlite3 baharimrv-android.db
```

### API Testing

**Postman Collection:**

Create a collection with common requests:

```json
{
  "info": { "name": "BahariMRV API" },
  "item": [
    {
      "name": "Login",
      "request": {
        "method": "POST",
        "url": "{{baseUrl}}/api/v1/auth/login",
        "body": {
          "mode": "raw",
          "raw": "{\"email\":\"manager@demo.baharimrv.local\",\"password\":\"demo123\"}"
        }
      }
    },
    {
      "name": "Get Profile",
      "request": {
        "method": "GET",
        "url": "{{baseUrl}}/api/v1/users/me",
        "header": [{"key": "Authorization", "value": "Bearer {{token}}"}]
      }
    }
  ]
}
```

**curl Examples:**

```bash
# Login and save token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"manager@demo.baharimrv.local","password":"demo123"}' \
  | jq -r '.sessionToken')

# Get user profile
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/users/me

# Get project dashboard
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/projects/{project-id}/dashboard
```

---

## Troubleshooting

### Backend Issues

**"Port 8080 already in use"**

```bash
# Find process using port 8080
lsof -i :8080  # macOS/Linux
netstat -ano | findstr :8080  # Windows

# Kill the process
kill -9 <PID>  # macOS/Linux
taskkill /PID <PID> /F  # Windows

# Or use a different port
PORT=8081 go run cmd/server/main.go
```

**"Database locked"**

SQLite is single-writer. Close other connections or use PostgreSQL:

```bash
# Install PostgreSQL
brew install postgresql@14

# Create database
createdb baharimrv-dev

# Update .env
DATABASE_URL=postgresql://localhost:5432/baharimrv-dev?sslmode=disable
```

**"Demo seed password required"**

Set in `.env`:

```bash
DEMO_SEED_PASSWORD=demo123
```

### Android Issues

**"BuildConfig.AUTH_CONFIGURED is false"**

Check `local.properties` has all four SSO variables set (or leave all empty for native-only auth).

**"API call failed: Unable to resolve host"**

- Emulator: Use `http://10.0.2.2:8080`, not `localhost`
- Physical device: Use your computer's local IP, ensure same WiFi network
- Check backend is running: `curl http://localhost:8080/health`

**"Map not loading"**

- Set `GOOGLE_MAPS_API_KEY` in `local.properties`
- Get a free key: [Google Maps Platform](https://console.cloud.google.com/)
- Enable "Maps SDK for Android"

**"Room migration error"**

Schema changed. Uninstall app and reinstall:

```bash
adb uninstall ke.co.baharilink.baharimrv
# Then run from Android Studio again
```

**"WorkManager sync not triggering"**

Force sync from UI or manually:

```bash
adb shell am broadcast \
  -a androidx.work.impl.background.systemjob.SystemJobService.ACTION_SCHEDULE_WORK \
  -n ke.co.baharilink.baharimrv/androidx.work.impl.background.systemjob.SystemJobService
```

### Common Build Errors

**"Cannot resolve symbol 'BuildConfig'"**

```bash
./gradlew clean
./gradlew build
# Or Build → Clean Project → Rebuild Project in Android Studio
```

**"Execution failed for task ':app:kspDebugKotlin'"**

Room schema issue. Check entity classes for errors.

```bash
./gradlew clean
# Fix entity annotations
./gradlew build
```

---

## Next Steps

- Read [DEPLOYMENT.md](./DEPLOYMENT.md) for production deployment
- Read [SSO_INTEGRATION.md](./SSO_INTEGRATION.md) for Jahazi SSO setup
- Read [SPEC.md](./SPEC.md) for product specification
- Check [TODO.md](./TODO.md) for enhancement roadmap

---

## Support

For development questions:
- GitHub Issues: [BahariLink/BahariMRV](https://github.com/baharilink/baharimrv/issues)
- Dev chat: [Slack/Discord link]
- Email: dev@baharilink.co.ke
