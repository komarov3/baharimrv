# BahariMRV SSO Integration Verification Test Plan

**Version:** 1.0  
**Date:** December 2024  
**Purpose:** Verify Jahazi SSO integration works correctly before production release

---

## Test Environment Setup

### Prerequisites
- [ ] Backend running with `FEDERATION_CLIENTS` configured
- [ ] Android app built with SSO configuration in `local.properties`
- [ ] Jahazi OAuth2 client registered and credentials obtained
- [ ] Test Jahazi accounts available
- [ ] IOTA testnet accessible

### Configuration Checklist

**Backend (.env):**
```bash
APP_ENV=staging
FEDERATION_CLIENTS=https://auth.jahazi.ke=baharimrv_jahazi_client
CHAIN_MODE=iota
IOTA_NETWORK=testnet
DATABASE_URL=postgresql://...?sslmode=require
```

**Android (local.properties):**
```properties
BAHARIMRV_API_BASE_URL=https://api-staging.baharimrv.example.com
JAHAZI_ISSUER_URL=https://auth.jahazi.ke
JAHAZI_CLIENT_ID=baharimrv_jahazi_client
JAHAZI_REDIRECT_URI=baharimrv://auth/callback
GOOGLE_MAPS_API_KEY=AIza...
```

---

## Test Cases

### TC-001: Native Registration (Baseline)

**Objective:** Verify native registration still works

**Steps:**
1. Open BahariMRV app
2. Tap "Sign Up"
3. Enter email: `test.native@example.com`
4. Enter password: `SecurePassword123`
5. Enter name: `Test Native User`
6. Enter phone: `+254700123456`
7. Select role: `Buyer`
8. Tap "Create Account"

**Expected Results:**
- [ ] Loading indicator shown
- [ ] "Account created, provisioning identity..." message shown (if async)
- [ ] OR immediate login if identity provisioned synchronously
- [ ] User redirected to home screen
- [ ] Profile shows user name and role
- [ ] Backend logs show IOTA identity provisioned

**Pass/Fail:** ___________

---

### TC-002: SSO Button Visibility

**Objective:** Verify SSO UI appears when configured

**Steps:**
1. Open BahariMRV app (not logged in)
2. Observe login screen

**Expected Results:**
- [ ] "Continue with Jahazi" button visible
- [ ] OR similar SSO option clearly displayed
- [ ] Button has Jahazi branding/logo (if applicable)

**Pass/Fail:** ___________

---

### TC-003: SSO Authorization Request

**Objective:** Verify OAuth flow starts correctly

**Steps:**
1. Tap "Continue with Jahazi"
2. Observe what happens

**Expected Results:**
- [ ] Browser or Custom Tab opens
- [ ] Jahazi login page loads
- [ ] URL contains correct parameters:
  - `client_id=baharimrv_jahazi_client`
  - `redirect_uri=baharimrv://auth/callback`
  - `scope=openid profile email`
  - `code_challenge` present (PKCE)
  - `code_challenge_method=S256`
  - `state` present
  - `nonce` present

**Pass/Fail:** ___________

---

### TC-004: SSO Login - First Time User

**Objective:** Verify new Jahazi user can sign into BahariMRV

**Steps:**
1. Start SSO flow (TC-003)
2. On Jahazi page, login with test account: `jahazi.test1@example.com`
3. Grant permissions when prompted
4. Wait for redirect

**Expected Results:**
- [ ] Redirected back to BahariMRV app
- [ ] "Provisioning identity..." message shown (if async)
- [ ] User logged into BahariMRV
- [ ] Profile shows:
  - Name from Jahazi account
  - Email: `fed-...@federated.invalid` (or similar)
  - Role: `Buyer` (default for federated users)
  - IOTA identity provisioned
- [ ] Backend database has:
  - User record created
  - `external_identities` record linking to Jahazi
  - `identity_provisions` record with `state=active`

**Pass/Fail:** ___________

---

### TC-005: SSO Login - Returning User

**Objective:** Verify existing Jahazi-linked user can login

**Steps:**
1. Logout from BahariMRV
2. Tap "Continue with Jahazi"
3. Login to Jahazi with same account as TC-004
4. Grant permissions (if prompted)

**Expected Results:**
- [ ] Redirected back to BahariMRV
- [ ] Logged in immediately (no re-provisioning)
- [ ] Profile shows same user data as before
- [ ] No duplicate user created

**Pass/Fail:** ___________

---

### TC-006: Native User Account Linking

**Objective:** Verify native user can link Jahazi account

**Steps:**
1. Login natively (TC-001 account)
2. Navigate to Profile
3. Tap "Link Jahazi Account" (or similar)
4. Complete Jahazi OAuth flow with `jahazi.test2@example.com`
5. Return to BahariMRV

**Expected Results:**
- [ ] Profile updated to show linked Jahazi account
- [ ] No logout occurred
- [ ] Session still valid
- [ ] Backend `external_identities` table has new record for this user

**Pass/Fail:** ___________

---

### TC-007: Login After Linking

**Objective:** Verify linked user can login either way

**Steps:**
1. Logout from BahariMRV
2. Login natively with `test.native@example.com` / `SecurePassword123`
3. Verify success, then logout
4. Login via SSO with `jahazi.test2@example.com`
5. Verify success

**Expected Results:**
- [ ] Both login methods work
- [ ] Same user account accessed both times
- [ ] No duplicate sessions or accounts

**Pass/Fail:** ___________

---

### TC-008: SSO User Cancellation

**Objective:** Verify graceful handling of cancelled SSO

**Steps:**
1. Logout from BahariMRV
2. Tap "Continue with Jahazi"
3. On Jahazi page, tap "Cancel" or close browser

**Expected Results:**
- [ ] Returned to BahariMRV login screen
- [ ] Error message shown: "Login cancelled" or similar
- [ ] No crash
- [ ] Can retry SSO or login natively

**Pass/Fail:** ___________

---

### TC-009: SSO User Denies Permissions

**Objective:** Verify handling of permission denial

**Steps:**
1. Start SSO flow
2. Login to Jahazi successfully
3. On permission screen, tap "Deny" or "Don't Allow"

**Expected Results:**
- [ ] Redirected back with error
- [ ] App shows: "Permission denied" or "Access denied"
- [ ] No account created
- [ ] Can retry

**Pass/Fail:** ___________

---

### TC-010: Invalid/Expired Authorization Code

**Objective:** Verify error handling for expired codes

**Steps:**
1. Start SSO flow
2. Complete Jahazi login successfully
3. Wait >10 minutes before returning to app (to expire code)
4. OR manually craft callback with old code

**Expected Results:**
- [ ] Backend rejects expired/invalid code
- [ ] App shows error: "Authorization expired, please try again"
- [ ] User can retry from beginning

**Pass/Fail:** ___________

---

### TC-011: State Parameter Mismatch (CSRF Protection)

**Objective:** Verify CSRF protection works

**Steps:**
1. Start SSO flow, note the `state` parameter in URL
2. Manually craft callback URL with different state:
   ```
   adb shell am start -W -a android.intent.action.VIEW \
     -d "baharimrv://auth/callback?code=test&state=wrong_state"
   ```

**Expected Results:**
- [ ] App rejects callback
- [ ] Error: "SSO state did not match"
- [ ] No session created

**Pass/Fail:** ___________

---

### TC-012: Deep Link Handling

**Objective:** Verify deep link correctly routes to OAuth handler

**Steps:**
1. Close BahariMRV app completely
2. Trigger deep link:
   ```bash
   adb shell am start -W -a android.intent.action.VIEW \
     -d "baharimrv://auth/callback?code=test_code&state=test_state"
   ```

**Expected Results:**
- [ ] App opens
- [ ] OAuth callback handler invoked
- [ ] Appropriate error shown (invalid code expected)
- [ ] No crash

**Pass/Fail:** ___________

---

### TC-013: Identity Provisioning Success

**Objective:** Verify IOTA identity provisioned for SSO users

**Steps:**
1. Complete SSO login as new user (TC-004)
2. Check backend logs
3. Query database:
   ```sql
   SELECT * FROM identity_provisions 
   WHERE user_id = (SELECT id FROM users WHERE email LIKE 'fed-%');
   ```

**Expected Results:**
- [ ] `identity_provisions` record exists
- [ ] `state = 'active'`
- [ ] `did` field populated
- [ ] `iota_address` field populated
- [ ] Backend logs show successful IOTA transaction

**Pass/Fail:** ___________

---

### TC-014: Identity Provisioning Failure Handling

**Objective:** Verify graceful handling of IOTA provisioning failure

**Steps:**
1. Stop IOTA RPC or set invalid `IOTA_RPC_URL`
2. Attempt SSO login as new user
3. Observe behavior

**Expected Results:**
- [ ] Backend returns 202 or appropriate status
- [ ] App shows: "Account created, identity provisioning in progress"
- [ ] User NOT logged in immediately
- [ ] Can retry login after IOTA fixed
- [ ] No crash or confusing error

**Pass/Fail:** ___________

---

### TC-015: Duplicate Email Prevention

**Objective:** Verify same Jahazi user can't create multiple BahariMRV accounts

**Steps:**
1. Complete SSO login with `jahazi.test3@example.com`
2. Logout
3. Login with same Jahazi account again

**Expected Results:**
- [ ] Same BahariMRV account accessed
- [ ] No duplicate user created
- [ ] User ID unchanged

**Pass/Fail:** ___________

---

### TC-016: Already Linked Account Protection

**Objective:** Verify can't link Jahazi account already linked to another user

**Steps:**
1. Login as User A natively
2. Link to `jahazi.test4@example.com`
3. Logout
4. Login as User B natively
5. Attempt to link same `jahazi.test4@example.com`

**Expected Results:**
- [ ] Error shown: "This Jahazi account is already linked"
- [ ] No link created
- [ ] User B remains without Jahazi link

**Pass/Fail:** ___________

---

### TC-017: Backend Token Validation

**Objective:** Verify backend properly validates ID tokens

**Steps:**
1. Capture ID token from successful SSO flow (network logs)
2. Attempt to replay it:
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/federation/exchange \
     -H "Content-Type: application/json" \
     -d '{"issuer":"https://auth.jahazi.ke","code":"old_code",...}'
   ```

**Expected Results:**
- [ ] Expired/reused code rejected
- [ ] Error: "Invalid authorization code"
- [ ] No session created

**Pass/Fail:** ___________

---

### TC-018: Offline SSO Behavior

**Objective:** Verify SSO gracefully fails when offline

**Steps:**
1. Enable Airplane mode
2. Tap "Continue with Jahazi"

**Expected Results:**
- [ ] Browser fails to load Jahazi page
- [ ] Network error shown
- [ ] User returned to app
- [ ] Can retry when online

**Pass/Fail:** ___________

---

### TC-019: Cross-Platform Consistency

**Objective:** Verify SSO user has consistent identity across sessions

**Steps:**
1. Login via SSO on Device A
2. Capture assigned role, projects, etc.
3. Logout
4. Login via SSO on Device B (or web if available)

**Expected Results:**
- [ ] Same user account accessed
- [ ] Same role, name, email
- [ ] Same data visible (projects, wallet, etc.)

**Pass/Fail:** ___________

---

### TC-020: Production Configuration Enforcement

**Objective:** Verify production safety checks work

**Steps:**
1. Set backend `APP_ENV=production`
2. Set `CHAIN_MODE=sandbox`
3. Attempt to start backend

**Expected Results:**
- [ ] Backend refuses to start
- [ ] Error: "production requires CHAIN_MODE=iota"

**Repeat for other validations:**
- [ ] PostgreSQL required (not SQLite)
- [ ] SSL required (not sslmode=disable)
- [ ] KMS_KEK required
- [ ] OIDC signing key required
- [ ] DEMO_SEED_DATA forbidden

**Pass/Fail:** ___________

---

## Performance Tests

### PT-001: SSO Flow Timing

**Objective:** Measure end-to-end SSO performance

**Steps:**
1. Start timer
2. Tap "Continue with Jahazi"
3. Complete login on Jahazi (use saved credentials for consistency)
4. Wait for BahariMRV home screen
5. Stop timer

**Expected Results:**
- [ ] Total time < 10 seconds (excluding Jahazi page load)
- [ ] No noticeable UI freezing
- [ ] Smooth transitions

**Measured Time:** ___________

**Pass/Fail:** ___________

---

### PT-002: Identity Provisioning Timing

**Objective:** Measure IOTA identity provisioning time

**Steps:**
1. Register new user (native or SSO)
2. Measure time from submission to `identity_provisions.state = 'active'`

**Expected Results:**
- [ ] Provisioning completes in < 60 seconds
- [ ] User receives feedback during wait
- [ ] No timeout errors

**Measured Time:** ___________

**Pass/Fail:** ___________

---

## Security Tests

### ST-001: Token Storage Security

**Objective:** Verify tokens stored securely

**Steps:**
1. Login via SSO
2. Inspect Android device storage:
   ```bash
   adb shell
   run-as ke.co.baharilink.baharimrv
   cat shared_prefs/*.xml
   ```

**Expected Results:**
- [ ] No plaintext tokens visible
- [ ] Tokens in `EncryptedSharedPreferences`
- [ ] No PKCE verifier persisted after exchange

**Pass/Fail:** ___________

---

### ST-002: HTTPS Enforcement

**Objective:** Verify HTTPS required in production

**Steps:**
1. Set Android `JAHAZI_ISSUER_URL=http://auth.jahazi.ke` (HTTP not HTTPS)
2. Attempt SSO login

**Expected Results:**
- [ ] App rejects HTTP issuer
- [ ] Error: "Jahazi endpoints must use HTTPS"

**Pass/Fail:** ___________

---

### ST-003: Backend CORS Protection

**Objective:** Verify CORS properly configured

**Steps:**
1. From unauthorized origin (e.g., `http://evil.com`), attempt API call:
   ```bash
   curl -H "Origin: http://evil.com" \
     http://api.baharimrv.example.com/api/v1/users/me
   ```

**Expected Results:**
- [ ] Request blocked (no CORS headers returned)
- [ ] OR 403/401 error

**Pass/Fail:** ___________

---

## Regression Tests

### RT-001: Native Auth Still Works

**Objective:** Verify SSO doesn't break native login

**Steps:**
1. Login natively (email/password)
2. Use app normally (create project, capture record, etc.)
3. Logout and login again natively

**Expected Results:**
- [ ] All native auth flows work as before
- [ ] No SSO-related errors in native flow

**Pass/Fail:** ___________

---

### RT-002: Existing Data Preserved

**Objective:** Verify SSO deployment doesn't affect existing accounts

**Steps:**
1. Query database for pre-existing users
2. Login as pre-existing user
3. Verify data intact

**Expected Results:**
- [ ] All old accounts still exist
- [ ] Projects, monitoring records, etc. unchanged
- [ ] Can login normally

**Pass/Fail:** ___________

---

## Test Summary

**Total Test Cases:** 25  
**Passed:** _______  
**Failed:** _______  
**Blocked:** _______  
**Not Tested:** _______

### Critical Issues Found
1. _______________________________________________
2. _______________________________________________
3. _______________________________________________

### Recommendations
1. _______________________________________________
2. _______________________________________________
3. _______________________________________________

### Sign-Off

**Tested By:** ___________________  
**Date:** ___________________  
**Environment:** ___________________  
**BahariMRV Version:** ___________________  
**Jahazi Integration Version:** ___________________

**Approved for Production:** [ ] Yes  [ ] No

**Approver:** ___________________  
**Date:** ___________________

---

## Rollback Plan

If critical issues found in production:

1. **Immediate:** Remove SSO buttons from UI (release hotfix)
2. **Backend:** Set `FEDERATION_CLIENTS=` (empty) to disable federation
3. **Database:** No rollback needed (additive schema changes only)
4. **Communication:** Notify users native login still works
5. **Fix:** Address issues, re-test, re-deploy

**Rollback triggers:**
- >5% login failure rate
- Critical security vulnerability discovered
- IOTA identity provisioning failing >50% of time
- Data corruption or account duplication observed
