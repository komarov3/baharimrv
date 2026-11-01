# BahariMRV Production Deployment Guide

**Last Updated:** December 2024  
**Target:** Production-ready deployment with Jahazi SSO integration

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Backend Deployment](#backend-deployment)
3. [Android App Deployment](#android-app-deployment)
4. [Jahazi SSO Setup](#jahazi-sso-setup)
5. [Database Setup](#database-setup)
6. [IOTA Configuration](#iota-configuration)
7. [SSL/TLS Certificates](#ssltls-certificates)
8. [Health Checks & Monitoring](#health-checks--monitoring)
9. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Services
- PostgreSQL 14+ with SSL enabled
- IOTA Rebased testnet access (or mainnet for production)
- Jahazi platform account for SSO integration
- Google Maps API key (Android)
- Domain with SSL certificate
- Linux server or container runtime (Docker recommended)

### Required Tools
- Go 1.21+
- Android Studio (for app builds)
- OpenSSL (for key generation)
- PostgreSQL client tools

---

## Backend Deployment

### 1. Environment Configuration

Copy the example configuration:

```bash
cd backend
cp .env.example .env
```

Edit `.env` and configure all required variables:

#### Essential Production Settings

```bash
# Environment
APP_ENV=production
PORT=8080
PUBLIC_BASE_URL=https://api.baharimrv.example.com

# Database (PostgreSQL with SSL)
DATABASE_URL=postgresql://username:password@db.example.com:5432/baharimrv?sslmode=require

# Security - Generate encryption key
KMS_KEK=$(openssl rand -base64 32)

# OIDC Signing Key
# Generate keypair:
openssl ecparam -genkey -name prime256v1 -noout -out oidc-signing-key.pem
# Set the key (can be PEM content or base64):
OIDC_SIGNING_PRIVATE_KEY=$(cat oidc-signing-key.pem)
OIDC_SIGNING_KID=baharimrv-prod-$(date +%Y)

# CORS
CORS_ORIGINS=https://app.baharimrv.example.com,https://admin.baharimrv.example.com

# IOTA (production must use real chain)
CHAIN_MODE=iota
IOTA_NETWORK=testnet
IOTA_RPC_URL=https://api.testnet.iota.cafe

# Jahazi Federation
FEDERATION_CLIENTS=https://auth.jahazi.ke=baharimrv_jahazi_client_id

# Demo data (MUST be false in production)
SEED_DEMO_DATA=false
```

#### Integration Modes

Set adapter modes based on your deployment stage:

- **sandbox**: Fake success responses (development only)
- **unavailable**: Honest unavailable state (staging without credentials)
- **integrated**: Real provider integration (production with credentials)

```bash
SATELLITE_MODE=unavailable
REGISTRY_MODE=unavailable
PAYMENT_MODE=sandbox
```

### 2. Build the Backend

```bash
cd backend
go mod download
go build -o baharimrv-server ./cmd/server
```

For production optimized build:

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o baharimrv-server ./cmd/server
```

### 3. Run Database Migrations

The server automatically runs migrations on startup, but you can test them manually:

```bash
# Set DATABASE_URL environment variable
export DATABASE_URL="postgresql://user:pass@host:5432/baharimrv?sslmode=require"

# Run the server in migration-only mode (if supported) or start and check logs
./baharimrv-server
```

Check logs for migration success messages.

### 4. Start the Server

```bash
# Load environment variables
source .env

# Run the server
./baharimrv-server
```

For production, use a process manager:

**systemd service** (`/etc/systemd/system/baharimrv.service`):

```ini
[Unit]
Description=BahariMRV API Server
After=network.target postgresql.service

[Service]
Type=simple
User=baharimrv
WorkingDirectory=/opt/baharimrv
EnvironmentFile=/opt/baharimrv/.env
ExecStart=/opt/baharimrv/baharimrv-server
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable baharimrv
sudo systemctl start baharimrv
sudo systemctl status baharimrv
```

**Docker Compose** (`docker-compose.yml`):

```yaml
version: '3.8'

services:
  api:
    build: ./backend
    ports:
      - "8080:8080"
    env_file:
      - ./backend/.env
    depends_on:
      - db
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: baharimrv
      POSTGRES_USER: baharimrv
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  postgres_data:
```

---

## Android App Deployment

### 1. Configure Build

Copy configuration template:

```bash
cd android
cp local.properties.example local.properties
```

Edit `local.properties`:

```properties
# Backend API
BAHARIMRV_API_BASE_URL=https://api.baharimrv.example.com

# Jahazi SSO (must match backend FEDERATION_CLIENTS)
JAHAZI_ISSUER_URL=https://auth.jahazi.ke
JAHAZI_CLIENT_ID=baharimrv_android_client
JAHAZI_REDIRECT_URI=baharimrv://auth/callback

# Google Maps
GOOGLE_MAPS_API_KEY=AIza...your-key-here
```

### 2. Build Release APK

```bash
cd android
./gradlew assembleRelease
```

The APK will be at: `app/build/outputs/apk/release/app-release-unsigned.apk`

### 3. Sign the APK

#### Generate Keystore (first time only)

```bash
keytool -genkey -v -keystore baharimrv-release.keystore \
  -alias baharimrv -keyalg RSA -keysize 2048 -validity 10000
```

Store the keystore securely and **never commit it to git**.

#### Sign the APK

```bash
# Using apksigner
apksigner sign --ks baharimrv-release.keystore \
  --out app-release-signed.apk \
  app/build/outputs/apk/release/app-release-unsigned.apk

# Or configure signing in build.gradle.kts
```

Add to `app/build.gradle.kts`:

```kotlin
android {
    signingConfigs {
        create("release") {
            storeFile = file(System.getenv("KEYSTORE_FILE") ?: "release.keystore")
            storePassword = System.getenv("KEYSTORE_PASSWORD")
            keyAlias = System.getenv("KEY_ALIAS")
            keyPassword = System.getenv("KEY_PASSWORD")
        }
    }
    buildTypes {
        release {
            signingConfig = signingConfigs.getByName("release")
            // ... other config
        }
    }
}
```

### 4. Test the Release Build

Install on a physical device:

```bash
adb install app-release-signed.apk
```

Test critical flows:
- Native registration with IOTA provisioning
- Jahazi SSO login
- Offline field capture
- Evidence upload
- Map rendering with real API key

### 5. Distribute

Options:
- **Google Play Store**: Follow [Android publishing guide](https://developer.android.com/studio/publish)
- **Internal distribution**: Use Firebase App Distribution or host APK directly
- **Enterprise MDM**: Deploy through your organization's MDM solution

---

## Jahazi SSO Setup

### 1. Register OAuth2 Client with Jahazi

Contact Jahazi platform administrators or use their developer portal to register BahariMRV:

**Required Information:**
- **Client Name**: BahariMRV
- **Client Type**: Public (PKCE-enabled, no client secret)
- **Redirect URIs**:
  - Android: `baharimrv://auth/callback`
  - Web (if applicable): `https://app.baharimrv.example.com/auth/callback`
- **Scopes**: `openid profile email`
- **Grant Types**: `authorization_code`
- **Token Endpoint Auth Method**: `none` (PKCE only)

**Outputs:**
- `client_id`: Use as `JAHAZI_CLIENT_ID` (Android) and in `FEDERATION_CLIENTS` (backend)
- `issuer_url`: Use as `JAHAZI_ISSUER_URL`

### 2. Register BahariMRV as Federation Partner

Similarly, BahariMRV can act as an identity provider for Jahazi users.

On the BahariMRV backend, the OIDC endpoints are automatically exposed:

- **Issuer**: `https://api.baharimrv.example.com`
- **Discovery**: `https://api.baharimrv.example.com/.well-known/openid-configuration`
- **Authorization**: `https://api.baharimrv.example.com/oauth2/authorize`
- **Token**: `https://api.baharimrv.example.com/oauth2/token`
- **UserInfo**: `https://api.baharimrv.example.com/oauth2/userinfo`
- **JWKS**: `https://api.baharimrv.example.com/.well-known/jwks.json`

Provide these to Jahazi for their federation configuration.

### 3. Test SSO Integration

#### Android Flow Test

1. Open BahariMRV app
2. Tap "Continue with Jahazi"
3. Redirected to Jahazi login (browser or Custom Tab)
4. Sign in with Jahazi credentials
5. Consent to share profile
6. Redirected back to BahariMRV
7. Account created automatically, IOTA identity provisioned
8. Logged into BahariMRV

#### Cross-Platform Test

1. Register natively on BahariMRV
2. Go to Profile → Link Jahazi Account
3. Complete Jahazi OAuth flow
4. Now can login to BahariMRV with either method
5. Test Jahazi platform accessing BahariMRV user data (if integrated)

### 4. Troubleshooting SSO

See [SSO_INTEGRATION.md](./SSO_INTEGRATION.md) for detailed troubleshooting.

---

## Database Setup

### 1. PostgreSQL Installation

**Ubuntu/Debian:**

```bash
sudo apt update
sudo apt install postgresql-14 postgresql-contrib
```

**Docker:**

```bash
docker run -d \
  --name baharimrv-db \
  -e POSTGRES_DB=baharimrv \
  -e POSTGRES_USER=baharimrv \
  -e POSTGRES_PASSWORD=secure_password \
  -v baharimrv_data:/var/lib/postgresql/data \
  -p 5432:5432 \
  postgres:16-alpine
```

### 2. Create Database and User

```sql
CREATE DATABASE baharimrv;
CREATE USER baharimrv_user WITH ENCRYPTED PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE baharimrv TO baharimrv_user;
```

### 3. Enable SSL

Edit `postgresql.conf`:

```conf
ssl = on
ssl_cert_file = '/etc/ssl/certs/server.crt'
ssl_key_file = '/etc/ssl/private/server.key'
```

Restart PostgreSQL:

```bash
sudo systemctl restart postgresql
```

### 4. Connection String

```bash
DATABASE_URL=postgresql://baharimrv_user:secure_password@localhost:5432/baharimrv?sslmode=require
```

For cloud providers:
- AWS RDS: Use provided connection string, ensure SSL certificates
- Google Cloud SQL: Use Cloud SQL Proxy or direct SSL connection
- Azure Database: Enable SSL, download certificate bundle

### 5. Backup Strategy

**Automated daily backups:**

```bash
#!/bin/bash
# /opt/baharimrv/backup.sh

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR=/opt/backups/baharimrv
mkdir -p $BACKUP_DIR

pg_dump -h localhost -U baharimrv_user -d baharimrv \
  | gzip > $BACKUP_DIR/baharimrv_$DATE.sql.gz

# Retain last 30 days
find $BACKUP_DIR -name "baharimrv_*.sql.gz" -mtime +30 -delete
```

Add to cron:

```bash
0 2 * * * /opt/baharimrv/backup.sh
```

---

## IOTA Configuration

### 1. Network Selection

**Testnet** (recommended for initial production):
- Lower gas costs
- Safe for testing
- Still provides real blockchain verification

```bash
CHAIN_MODE=iota
IOTA_NETWORK=testnet
IOTA_RPC_URL=https://api.testnet.iota.cafe
IOTA_EXPLORER_URL=https://explorer.rebased.iota.org/testnet
```

**Mainnet** (for full production):
- Real value transactions
- Requires mainnet tokens for gas
- Production-grade reliability

```bash
CHAIN_MODE=iota
IOTA_NETWORK=mainnet
IOTA_RPC_URL=https://api.iota.cafe
IOTA_EXPLORER_URL=https://explorer.rebased.iota.org
```

### 2. Gas Sponsorship Setup

For user identity provisioning without requiring users to hold IOTA tokens:

1. Create a sponsor wallet with sufficient IOTA tokens
2. Configure the executor service (if using centralized sponsorship)
3. Set `IOTA_EXECUTOR_URL` if using external transaction building

### 3. Deploy Move Contracts

If using custom Move contracts for provenance:

```bash
# Build contracts
cd contracts
iota move build

# Deploy to testnet
iota client publish --gas-budget 100000000

# Note the package ID
IOTA_PACKAGE_ID=0x...
```

Update backend `.env` with the package ID.

### 4. Monitor Chain Health

Check RPC connectivity:

```bash
curl -X POST https://api.testnet.iota.cafe \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"rpc.discover","params":[]}'
```

---

## SSL/TLS Certificates

### 1. Obtain Certificates

**Let's Encrypt (recommended for HTTP services):**

```bash
sudo apt install certbot
sudo certbot certonly --standalone -d api.baharimrv.example.com
```

Certificates will be at:
- `/etc/letsencrypt/live/api.baharimrv.example.com/fullchain.pem`
- `/etc/letsencrypt/live/api.baharimrv.example.com/privkey.pem`

**Auto-renewal:**

```bash
sudo certbot renew --dry-run
```

Add to cron for automatic renewal.

### 2. Configure Reverse Proxy

**Nginx** (`/etc/nginx/sites-available/baharimrv`):

```nginx
server {
    listen 80;
    server_name api.baharimrv.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.baharimrv.example.com;

    ssl_certificate /etc/letsencrypt/live/api.baharimrv.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.baharimrv.example.com/privkey.pem;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    client_max_body_size 10M;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable and reload:

```bash
sudo ln -s /etc/nginx/sites-available/baharimrv /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

---

## Health Checks & Monitoring

### 1. Health Check Endpoint

The backend exposes a health check at:

```bash
curl https://api.baharimrv.example.com/health
```

Expected response:

```json
{
  "status": "healthy",
  "database": "connected",
  "iota": "reachable",
  "version": "0.1.0"
}
```

### 2. Monitoring Setup

**Prometheus metrics** (if implemented):

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'baharimrv'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

**Uptime monitoring:**

Use services like:
- UptimeRobot
- Pingdom
- StatusCake

Monitor:
- API health endpoint
- Database connectivity
- IOTA RPC responsiveness
- SSO authentication flow

### 3. Logging

**Structured logging to file:**

```bash
# systemd service
StandardOutput=append:/var/log/baharimrv/server.log
StandardError=append:/var/log/baharimrv/error.log
```

**Log rotation** (`/etc/logrotate.d/baharimrv`):

```conf
/var/log/baharimrv/*.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    create 0640 baharimrv baharimrv
    postrotate
        systemctl reload baharimrv
    endscript
}
```

**Centralized logging:**

Forward logs to:
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Grafana Loki
- Cloud provider logging (CloudWatch, Stackdriver)

### 4. Alerting

Set up alerts for:
- API downtime (>1 minute)
- Database connection failures
- IOTA RPC failures
- High error rates (>5% of requests)
- Disk space low (<10% free)
- SSL certificate expiring (<30 days)

---

## Troubleshooting

### Backend Won't Start

**Error: "production requires CHAIN_MODE=iota"**
- Solution: Set `CHAIN_MODE=iota` in `.env`

**Error: "production DATABASE_URL must use PostgreSQL"**
- Solution: Change SQLite to PostgreSQL connection string

**Error: "KMS_KEK is required in production"**
- Solution: Generate and set encryption key: `openssl rand -base64 32`

**Error: "production OIDC signing key and kid are required"**
- Solution: Generate and set OIDC signing keypair (see configuration above)

### Android App Shows "SSO Not Configured"

- Check `local.properties` has all Jahazi variables set
- Rebuild the app: `./gradlew clean assembleDebug`
- Verify `BuildConfig.AUTH_CONFIGURED` is `true` in logs

### SSO Redirect Fails

**"No pending authorization"**
- The OAuth state was lost or expired
- Try sign-in flow again from the start

**"SSO state did not match"**
- Possible CSRF attack or app reinstall between authorize and callback
- Restart the sign-in flow

**"Jahazi sign-in was denied: access_denied"**
- User cancelled or denied consent
- Check OAuth client configuration on Jahazi side

### IOTA Identity Provisioning Fails

**"Account is recorded, but no session was issued"**
- Identity provisioning is still in progress (async operation)
- Wait 30-60 seconds and try logging in again
- Check backend logs for IOTA RPC errors

**"IOTA RPC unreachable"**
- Check `IOTA_RPC_URL` is correct
- Verify network connectivity to IOTA network
- Check if testnet/mainnet is experiencing downtime

### Maps Not Loading

**Blank map or "Error loading map"**
- Verify `GOOGLE_MAPS_API_KEY` is set in `local.properties`
- Check API key is enabled for Maps SDK for Android
- Verify API key restrictions allow your app's package name and SHA-1

### Database Connection Issues

**"connection refused"**
- Check PostgreSQL is running: `sudo systemctl status postgresql`
- Verify connection string host and port

**"SSL required"**
- Remove `sslmode=disable` from `DATABASE_URL`
- Use `sslmode=require` or `sslmode=verify-full`

### Evidence Upload Failures

**"File too large"**
- Check `MAX_REQUEST_BYTES` in backend config
- Increase Nginx `client_max_body_size` if using reverse proxy

**"Storage path not writable"**
- Check `EVIDENCE_ROOT` directory exists and has write permissions
- `sudo chown baharimrv:baharimrv /opt/baharimrv/data/evidence`

---

## Security Checklist

Before going live:

- [ ] `APP_ENV=production` in backend
- [ ] All URLs use HTTPS (no HTTP except localhost in debug builds)
- [ ] PostgreSQL SSL enabled and enforced
- [ ] `KMS_KEK` generated and secured
- [ ] OIDC signing key generated and secured
- [ ] `CORS_ORIGINS` limited to your domains (no wildcards)
- [ ] Release APK signed with secure keystore
- [ ] Keystore password stored securely (not in git)
- [ ] `SEED_DEMO_DATA=false` in production
- [ ] Google Maps API key has usage restrictions
- [ ] Database backups automated
- [ ] Monitoring and alerting configured
- [ ] SSL certificates auto-renewal enabled

---

## Support

For issues or questions:
- GitHub Issues: [BahariLink/BahariMRV](https://github.com/baharilink/baharimrv/issues)
- Email: support@baharilink.co.ke
- Documentation: [docs.baharilink.co.ke](https://docs.baharilink.co.ke)

---

**Next Steps:**
- [DEVELOPMENT.md](./DEVELOPMENT.md) - Local development setup
- [SSO_INTEGRATION.md](./SSO_INTEGRATION.md) - Jahazi SSO integration guide
- [SPEC.md](./SPEC.md) - Product specification
