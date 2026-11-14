# Silent session refresh spec

Status: implemented on all three layers, tests green (Android + web, 7 day rotating grants, deploy held for confirmation)
Scope: BahariMRV backend session lifecycle + Android and web session handling

## Problem

Local sessions are DB rows (`LocalSession`) with a one hour TTL (`SESSION_TTL`,
backend/internal/config/config.go:126). No renewal or refresh credential exists.
When the hour passes, every request 401s: Android drops to the "Load by ID"
fallback home, web throws the user back to sign-in. Both happen mid-task, with
no warning and no recovery short of a full re-login (Jahazi SSO custom tab on
Android). That is the experience to remove.

## Design

One rotating refresh credential per sign-in, accepted only at a dedicated
endpoint. Access sessions stay short lived; the refresh grant is what survives
idle time.

### Backend

- New model `RefreshGrant`: ID, TokenHash (unique index), UserID (index),
  ExpiresAt, RevokedAt, CreatedAt. Registered in models.All() so AutoMigrate
  creates it. Only the SHA-256 hash is stored, same as LocalSession.
- Config `REFRESH_TTL`, default 168h (7 days).
- Every place that issues a local session also issues a refresh grant:
  password login, registration, Jahazi federation exchange. `auth.Result`
  gains `refreshToken` and `refreshExpiresAt` (omitempty).
- `POST /v1/auth/refresh` (authLimit + bodyLimit, no bearer required):
  body `{refreshToken}`. Validates hash, unrevoked, unexpired, user active.
  In one transaction: revoke the presented grant, issue a rotated grant,
  issue a new local session. Returns the same shape as login.
- Reuse detection: presenting an already revoked grant revokes every refresh
  grant for that user (stolen-token signal) and returns 401.
- Logout revokes the user's refresh grants as well as the access session.
- Errors use the existing client-safe error envelope; 401 for any invalid,
  expired, revoked or reused grant. No grant material in logs.

### Android

- `LoginResponse` gains refreshToken/refreshExpiresAt; `AuthRepository.storeSession`
  persists the refresh token in the existing EncryptedSharedPreferences slot
  (`TokenSet.refreshToken`, today always null).
- New OkHttp `Authenticator` on both the JSON and tiles clients: on a 401 for a
  bearer-carrying request, refresh once (single flight, synchronized), retry
  the original request with the new token. The refresh call uses a separate
  client without the authenticator so it can never recurse.
- Refresh failure clears tokens and session metadata, so the existing auth
  gate shows sign-in. No silent retry loops, no token in logs.
- No proactive timer: the 401 path is the trigger and is invisible to the user
  when it succeeds.

### Web

- `AuthSession` gains refreshToken/refreshExpiresAt; AuthContext keeps the
  refresh token in sessionStorage beside the access token (same exposure
  class as today's token, no new storage surface).
- ApiClient gains a single-flight unauthorized handler: on 401 it asks
  AuthContext to refresh, then retries the request once with the new token.
  The refresh POST bypasses the handler so it cannot recurse. Failure signs
  the tab out through the existing gate.

## Tests

- Go: rotation rejects the old grant; reuse revokes the family; expired grant
  rejected; logout revokes grants; refreshed session authenticates; federated
  sign-in receives a usable grant.
- Android: MockWebServer tests for 401-refresh-retry, single flight under
  concurrent 401s, and session clear on refresh failure.
- Web: vitest for 401-refresh-retry and sign-out on refresh failure.

## Verification and rollout

- go build + go test, Gradle assembleDebug + unit tests, web typecheck +
  vitest + build, all green before anything ships.
- Backend deploy to Fly only after explicit confirmation. Until deployed,
  clients keep today's behaviour (no refresh token issued, 401 path unchanged).
- On-device proof after deploy: session expiry no longer ejects to the
  fallback home; a revoked grant still ejects (honest failure).

## Out of scope

- Changing SESSION_TTL or the Jahazi issuer's own tokens.
- The web dashboard basemap replacement (paused separately).
- Biometric or step-up changes; step-up tickets stay as they are.