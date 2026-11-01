# Jahazi SSO Integration Guide

**Last Updated:** December 2024  
**Integration Type:** OAuth 2.0 with PKCE (Public Client)  
**Flow:** Authorization Code with Proof Key for Code Exchange

---

## Table of Contents

1. [Overview](#overview)
2. [Integration Architecture](#integration-architecture)
3. [OAuth2 Client Registration](#oauth2-client-registration)
4. [Backend Configuration](#backend-configuration)
5. [Android Configuration](#android-configuration)
6. [SSO Flow Walkthrough](#sso-flow-walkthrough)
7. [Account Linking](#account-linking)
8. [Testing the Integration](#testing-the-integration)
9. [Security Considerations](#security-considerations)
10. [Troubleshooting](#troubleshooting)

---

## Overview

BahariMRV integrates with Jahazi using **federated authentication**, allowing users to:

1. **Sign up/Login to BahariMRV with Jahazi credentials** (Jahazi → BahariMRV)
2. **Access BahariMRV from Jahazi** (BahariMRV as identity provider)
3. **Link existing accounts** between platforms
4. **Single identity** across both ecosystems

### Key Features

- **PKCE Flow**: No client secrets (secure for mobile apps)
- **Automatic Account Creation**: First-time Jahazi users get a BahariMRV account automatically
- **IOTA Identity Provisioning**: All federated users receive blockchain identity
- **Account Linking**: Existing users can connect their Jahazi identity
- **Bidirectional**: Both platforms can authenticate each other's users

---

## Integration Architecture

```
┌─────────────────┐                    ┌─────────────────┐
│  Jahazi Platform│                    │BahariMRV Backend│
│  (Auth Server)  │                    │  (Relying Party)│
└────────┬────────┘                    └────────┬────────┘
         │                                      │
         │  1. Authorization Request            │
         │      (with PKCE challenge)           │
         │◄─────────────────────────────────────┤
         │                                      │
         │  2. User authenticates               │
         │     & grants consent                 │
         │                                      │
         │  3. Authorization Code               │
         ├─────────────────────────────────────►│
         │                                      │
         │  4. Token Exchange                   │
         │     (with PKCE verifier)             │
         │◄─────────────────────────────────────┤
         │                                      │
         │  5. ID Token + Access Token          │
         ├─────────────────────────────────────►│
         │                                      │
         │  6. UserInfo (optional)              │
         │◄─────────────────────────────────────┤
         │                                      │
         │  7. Session created                  │
         │     IOTA identity provisioned        │
         └──────────────────────────────────────┘
```

### Flow Summary

1. **Android app** generates PKCE code verifier and challenge
2. **Browser/Custom Tab** opens Jahazi authorization endpoint
3. **User** signs in to Jahazi and grants permission
4. **Jahazi** redirects back with authorization code
5. **Android app** receives redirect via deep link
6. **Android app** sends code + verifier to BahariMRV backend
7. **Backend** exchanges code for tokens with Jahazi
8. **Backend** validates ID token, creates/finds user, provisions IOTA identity
9. **Backend** issues BahariMRV session token
10. **Android app** stores session and navigates to home

---

## OAuth2 Client Registration

### Registering BahariMRV with Jahazi

Contact Jahazi administrators or use their developer portal to register BahariMRV as an OAuth2 client.

#### Registration Information

| Field | Value |
|-------|-------|
| **Client Name** | BahariMRV |
| **Client Type** | Public (Native Mobile App) |
| **Grant Types** | `authorization_code` |
| **Response Types** | `code` |
| **Token Endpoint Auth Method** | `none` (PKCE only, no client secret) |
| **PKCE** | Required (S256) |
| **Redirect URIs** | `baharimrv://auth/callback` |
| **Scopes** | `openid profile email` |
| **Logo** | [BahariMRV logo URL] |
| **Privacy Policy** | `https://baharimrv.ke/privacy` |
| **Terms of Service** | `https://baharimrv.ke/terms` |

#### Outputs You'll Receive

- **Client ID**: `baharimrv_jahazi_android` (example)
- **Issuer URL**: `https://auth.jahazi.ke`
- **Discovery URL**: `https://auth.jahazi.ke/.well-known/openid-configuration`

### Registering Jahazi as Federated Client in BahariMRV

Similarly, Jahazi can register to use BahariMRV as an identity provider.

BahariMRV automatically exposes OIDC endpoints:

- **Issuer**: `https://api.baharimrv.example.com`
- **Discovery**: `https://api.baharimrv.example.com/.well-known/openid-configuration`
- **Authorization**: `https://api.baharimrv.example.com/oauth2/authorize`
- **Token**: `https://api.baharimrv.example.com/oauth2/token`
- **UserInfo**: `https://api.baharimrv.example.com/oauth2/userinfo`
- **JWKS**: `https://api.baharimrv.example.com/.well-known/jwks.json`

Jahazi will need to:
1. Register their client
2. Configure their backend to trust BahariMRV's issuer
3. Add BahariMRV to their federation allowlist

---

## Backend Configuration

### Environment Variables

Add to `backend/.env`:

```bash
# Jahazi Federation
# Format: issuer_url=client_id (comma-separated for multiple)
FEDERATION_CLIENTS=https://auth.jahazi.ke=baharimrv_jahazi_android

# If multiple clients or platforms:
# FEDERATION_CLIENTS=https://auth.jahazi.ke=client1,https://other.platform=client2
```

### How It Works

1. **Trust Establishment**: `FEDERATION_CLIENTS` maps trusted issuer URLs to expected client IDs
2. **Token Validation**: Backend validates ID tokens from Jahazi using OIDC discovery
3. **Subject Mapping**: Backend links Jahazi's `sub` claim to BahariMRV user account
4. **Account Creation**: First-time users get auto-created accounts with IOTA identities

### Code Reference

See `backend/internal/auth/federation.go`:

- `FederationService.Login()`: Handles SSO login
- `FederationService.Link()`: Handles account linking
- `ExternalValidator`: Validates ID tokens from trusted issuers

---

## Android Configuration

### local.properties

```properties
# Jahazi SSO Configuration
JAHAZI_ISSUER_URL=https://auth.jahazi.ke
JAHAZI_CLIENT_ID=baharimrv_jahazi_android
JAHAZI_REDIRECT_URI=baharimrv://auth/callback
```

### Build Configuration

The `build.gradle.kts` automatically:
1. Reads these values from `local.properties`
2. Injects them into `BuildConfig`
3. Sets up deep link handling in `AndroidManifest.xml`

**Automatic behavior:**

- `BuildConfig.AUTH_CONFIGURED = true` when all four SSO variables are set
- SSO UI shown only when `AuthRepository.isConfigured == true`
- Deep link registered for `baharimrv://auth/callback`

### Deep Link Handling

`AndroidManifest.xml` already contains:

```xml
<intent-filter android:autoVerify="true">
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data
        android:host="${ssoHost}"
        android:path="${ssoPath}"
        android:scheme="${ssoScheme}" />
</intent-filter>
```

This resolves to `baharimrv://auth/callback` when configured.

### Code Reference

See `android/app/src/main/java/ke/co/baharilink/baharimrv/data/repository/AuthRepository.kt`:

- `beginAuthorization()`: Starts OAuth flow
- `completeAuthorization()`: Handles callback
- `linkJahazi()`: Links Jahazi account to existing user

---

## SSO Flow Walkthrough

### 1. User Clicks "Continue with Jahazi"

**UI Layer** (`AuthScreen.kt` or equivalent):

```kotlin
if (authRepository.isConfigured) {
    Button(onClick = { viewModel.signInWithJahazi() }) {
        Text("Continue with Jahazi")
    }
}
```

**ViewModel**:

```kotlin
fun signInWithJahazi() {
    viewModelScope.launch {
        val authUri = authRepository.beginAuthorization(MODE_LOGIN)
        // Launch browser with authUri
        _ssoAuthUri.value = authUri
    }
}
```

### 2. AuthRepository Builds Authorization URL

```kotlin
suspend fun beginAuthorization(mode: String = MODE_LOGIN): Uri {
    val issuer = BuildConfig.JAHAZI_ISSUER_URL.trimEnd('/')
    val pending = PendingAuthorization(
        state = Pkce.generateState(),
        verifier = Pkce.generateVerifier(),
        nonce = Pkce.generateNonce(),
        mode = mode,
    )
    secureStore.savePendingAuthorization(pending)
    
    return Uri.parse("$issuer/authorize").buildUpon()
        .appendQueryParameter("response_type", "code")
        .appendQueryParameter("client_id", BuildConfig.JAHAZI_CLIENT_ID)
        .appendQueryParameter("redirect_uri", BuildConfig.JAHAZI_REDIRECT_URI)
        .appendQueryParameter("scope", "openid profile email")
        .appendQueryParameter("state", pending.state)
        .appendQueryParameter("nonce", pending.nonce)
        .appendQueryParameter("code_challenge", Pkce.s256Challenge(pending.verifier))
        .appendQueryParameter("code_challenge_method", "S256")
        .build()
}
```

**Generated URL example:**

```
https://auth.jahazi.ke/authorize
  ?response_type=code
  &client_id=baharimrv_jahazi_android
  &redirect_uri=baharimrv://auth/callback
  &scope=openid+profile+email
  &state=abc123...
  &nonce=xyz789...
  &code_challenge=def456...
  &code_challenge_method=S256
```

### 3. Browser Opens Jahazi Login

**UI launches Custom Tab:**

```kotlin
val customTabsIntent = CustomTabsIntent.Builder()
    .setShowTitle(true)
    .build()
customTabsIntent.launchUrl(context, authUri)
```

User sees Jahazi's login page, signs in, grants consent.

### 4. Jahazi Redirects Back

**Redirect URL:**

```
baharimrv://auth/callback
  ?code=AUTH_CODE_HERE
  &state=abc123...
```

**Android catches deep link** in `MainActivity`:

```kotlin
override fun onNewIntent(intent: Intent?) {
    super.onNewIntent(intent)
    intent?.data?.let { uri ->
        if (uri.scheme == "baharimrv" && uri.host == "auth") {
            handleAuthCallback(uri)
        }
    }
}
```

### 5. App Completes Authorization

```kotlin
fun handleAuthCallback(uri: Uri) {
    viewModelScope.launch {
        try {
            authRepository.completeAuthorization(uri)
            // Navigate to home
        } catch (e: Exception) {
            // Show error
        }
    }
}
```

**AuthRepository.completeAuthorization():**

```kotlin
suspend fun completeAuthorization(callback: Uri) {
    val pending = requirePending(callback)  // Validates state
    try {
        val response = api.exchangeFederation(
            FederationExchangeRequest(
                issuer = BuildConfig.JAHAZI_ISSUER_URL,
                code = callback.getQueryParameter("code")!!,
                codeVerifier = pending.verifier,
                nonce = pending.nonce,
                redirectUri = BuildConfig.JAHAZI_REDIRECT_URI,
            )
        )
        storeSession(response, federatedIssuer(), null)
    } finally {
        secureStore.clearPendingAuthorization()
    }
}
```

### 6. Backend Exchanges Code for Tokens

**API endpoint:** `POST /api/v1/auth/federation/exchange`

**Backend flow:**

```go
func (h *FederationHandler) Exchange(c *gin.Context) {
    var req ExchangeRequest
    c.BindJSON(&req)
    
    // 1. Exchange authorization code with Jahazi
    tokenResp := exchangeCodeWithIssuer(req.Issuer, req.Code, req.CodeVerifier, req.RedirectUri)
    
    // 2. Validate ID token
    claims := validateIDToken(tokenResp.IDToken, req.Nonce)
    
    // 3. Find or create user
    user := findOrCreateFederatedUser(claims)
    
    // 4. Provision IOTA identity (if needed)
    ensureIdentityProvisioned(user.ID)
    
    // 5. Create session
    session := createSession(user.ID)
    
    c.JSON(200, LoginResponse{User: user, SessionToken: session.Token})
}
```

### 7. App Stores Session

Session stored in `EncryptedSharedPreferences`, user redirected to home.

---

## Account Linking

Users who signed up natively can link their Jahazi account later.

### UI Flow

**Profile Screen:**

```kotlin
if (authRepository.isConfigured && !user.hasJahaziLink) {
    Button(onClick = { viewModel.linkJahaziAccount() }) {
        Text("Link Jahazi Account")
    }
}
```

### Link Flow

**Same OAuth flow, different mode:**

```kotlin
val authUri = authRepository.beginAuthorization(MODE_LINK)
// Launch browser

// On callback:
authRepository.linkJahazi(callbackUri)
```

**Backend creates link:**

```go
func (s *FederationService) Link(ctx context.Context, userID, rawIDToken, expectedNonce string) {
    claims := s.validator.Validate(ctx, rawIDToken, expectedNonce)
    
    // Check if already linked
    existing := findExternalIdentity(claims.Issuer, claims.Subject)
    if existing != nil && existing.UserID != userID {
        return error("This Jahazi account is already linked to another user")
    }
    
    // Create link
    createExternalIdentity(userID, claims.Issuer, claims.Subject)
}
```

**Result:**
- User can now login with Jahazi credentials
- Both login methods work (native email/password AND Jahazi SSO)

---

## Testing the Integration

### 1. Test Native Registration

```bash
# Register natively
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "securepassword123",
    "name": "Test User",
    "role": "buyer"
  }'
```

### 2. Test SSO Login (Android)

1. Open app
2. Tap "Continue with Jahazi"
3. Sign in to Jahazi (or use demo account)
4. Grant permissions
5. Redirected back to app
6. Check: User is logged in, profile shows Jahazi identity

### 3. Test Account Linking

1. Login natively
2. Go to Profile
3. Tap "Link Jahazi Account"
4. Complete OAuth flow
5. Check: Profile shows linked Jahazi account

### 4. Test Cross-Platform Login

1. Register on BahariMRV Android
2. Link Jahazi account
3. On Jahazi web/app, try to access BahariMRV resources
4. Should work with BahariMRV identity

### 5. Test Token Validation

```bash
# Get ID token from Jahazi
JAHAZI_ID_TOKEN="eyJhbGc..."

# Exchange with BahariMRV
curl -X POST http://localhost:8080/api/v1/auth/federation/exchange \
  -H "Content-Type: application/json" \
  -d '{
    "issuer": "https://auth.jahazi.ke",
    "code": "AUTH_CODE",
    "code_verifier": "VERIFIER",
    "nonce": "NONCE",
    "redirect_uri": "baharimrv://auth/callback"
  }'
```

### 6. Test Error Scenarios

**User denies consent:**
- Expected: Redirect with `error=access_denied`
- App should show "Login cancelled" message

**Invalid state:**
- Expected: App rejects callback
- Error: "SSO state did not match"

**Expired authorization code:**
- Expected: Backend returns 400
- User should retry from start

**Already linked account:**
- Expected: Backend returns error
- User informed to unlink first or use existing link

---

## Security Considerations

### PKCE (Proof Key for Code Exchange)

**Why PKCE?**
- No client secret in mobile app (can't be kept secret)
- Protects against authorization code interception

**How it works:**
1. Generate random `code_verifier` (43-128 chars)
2. Compute `code_challenge = BASE64URL(SHA256(code_verifier))`
3. Send challenge in authorization request
4. Send verifier in token exchange
5. Server validates: `SHA256(verifier) == challenge`

### State Parameter

**Prevents CSRF attacks:**
- Random state generated and stored locally
- Included in authorization request
- Validated on callback
- Mismatched state rejected

### Nonce

**Prevents replay attacks:**
- Random nonce generated and stored
- Included in authorization request
- Must match `nonce` claim in ID token
- Jahazi binds nonce to issued ID token

### HTTPS Enforcement

**Production requirements:**
- All OAuth endpoints MUST use HTTPS
- Redirect URIs SHOULD use custom scheme (safer than Universal Links)
- Backend validates issuer URLs are HTTPS

### Token Storage

**Android:**
- Session tokens in `EncryptedSharedPreferences`
- PKCE verifier in secure storage (cleared after use)
- Never log tokens or verifiers

**Backend:**
- Session tokens hashed before database storage
- Federated tokens never persisted (validated and discarded)
- Refresh tokens encrypted with KMS_KEK

### Issuer Validation

**Backend checks:**
- Issuer in `FEDERATION_CLIENTS` (allowlist)
- ID token signature valid (JWKS from issuer)
- `aud` claim matches expected client ID
- `exp` claim not expired
- `iss` claim matches trusted issuer

---

## Troubleshooting

### "SSO Not Configured" in App

**Cause:** `BuildConfig.AUTH_CONFIGURED == false`

**Solutions:**
1. Check `local.properties` has all four variables set:
   ```
   JAHAZI_ISSUER_URL=https://auth.jahazi.ke
   JAHAZI_CLIENT_ID=baharimrv_jahazi_android
   JAHAZI_REDIRECT_URI=baharimrv://auth/callback
   BAHARIMRV_API_BASE_URL=http://10.0.2.2:8080
   ```
2. Rebuild: `./gradlew clean assembleDebug`
3. Check logcat for `AUTH_CONFIGURED` value

### "No pending authorization was found"

**Cause:** App was killed or restarted between authorize and callback

**Solutions:**
- Don't kill app during OAuth flow
- Increase `PendingAuthorization` persistence (use DataStore instead of memory)
- Ask user to retry sign-in

### "SSO state did not match"

**Cause:** State parameter mismatch (possible CSRF attack or corrupted state)

**Solutions:**
- Restart sign-in flow
- Check no other app is intercepting deep links
- Verify `state` in callback URL matches saved `pending.state`

### "Jahazi sign-in was denied: access_denied"

**Cause:** User clicked "Cancel" or denied permissions

**Solutions:**
- Normal user action, just inform user
- Offer to retry or use native registration

### "Federation client not trusted"

**Cause:** Backend doesn't trust Jahazi issuer

**Solutions:**
1. Add to backend `.env`:
   ```
   FEDERATION_CLIENTS=https://auth.jahazi.ke=baharimrv_jahazi_android
   ```
2. Restart backend
3. Verify issuer URL exactly matches (no trailing slash differences)

### "ID token validation failed"

**Causes:**
- Expired token
- Invalid signature
- Nonce mismatch
- Audience mismatch

**Solutions:**
- Check clock sync (backend and Jahazi)
- Verify `FEDERATION_CLIENTS` has correct client ID
- Check nonce is passed correctly in token exchange
- Enable debug logs to see exact validation error

### "Account is recorded, but no session was issued"

**Cause:** IOTA identity provisioning is async and not yet complete

**Solutions:**
- Wait 30-60 seconds
- Try logging in again
- Check backend logs for provisioning errors
- Verify IOTA RPC is reachable

### Deep Link Not Working

**Android:**
1. Verify intent filter in `AndroidManifest.xml`
2. Test deep link:
   ```bash
   adb shell am start -W -a android.intent.action.VIEW \
     -d "baharimrv://auth/callback?code=test&state=test"
   ```
3. Check if another app claimed the scheme
4. Try Universal Links as alternative

**iOS (if applicable):**
- Configure Associated Domains
- Verify `apple-app-site-association` file

### Backend Can't Reach Jahazi

**Causes:**
- Network firewall blocks outbound HTTPS
- Jahazi issuer URL incorrect
- DNS resolution failure

**Solutions:**
- Test from backend server:
  ```bash
  curl https://auth.jahazi.ke/.well-known/openid-configuration
  ```
- Check firewall allows outbound 443
- Verify DNS resolves correctly
- Use IP whitelisting if Jahazi provides

---

## Advanced Topics

### Custom Scopes

Request additional scopes:

```kotlin
.appendQueryParameter("scope", "openid profile email conservation:read")
```

Backend validates scope in access token before granting permissions.

### Refresh Tokens

If Jahazi issues refresh tokens:

```go
// Store encrypted refresh token
// Use to obtain new access tokens without re-authentication
```

### Multiple Identity Providers

Support multiple federations:

```bash
FEDERATION_CLIENTS=https://auth.jahazi.ke=client1,https://auth.other.ke=client2
```

Each provider gets own login button, own client ID.

### Single Logout

Implement logout propagation:

```kotlin
// Logout from BahariMRV
authRepository.signOut()

// Optionally logout from Jahazi
val logoutUri = "$issuer/logout?post_logout_redirect_uri=$appUri"
launchBrowser(logoutUri)
```

---

## Resources

- [OAuth 2.0 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [PKCE RFC 7636](https://tools.ietf.org/html/rfc7636)
- [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html)
- [Android Custom Tabs](https://developer.chrome.com/docs/android/custom-tabs/)
- [App Links (Deep Linking)](https://developer.android.com/training/app-links)

---

## Support

For SSO integration help:
- GitHub Issues: [BahariLink/BahariMRV](https://github.com/baharilink/baharimrv/issues)
- Jahazi Support: support@jahazi.ke
- BahariMRV Support: support@baharilink.co.ke
