# BahariMRV TODO & Enhancement Tracker

**Last Updated:** December 2024 (Production Release)  
**Source:** Comprehensive Gap & Bug Assessment + Production Release Requirements

---

## 🚀 Production Release Completed (December 2024)

### Configuration & Documentation ✅
- [x] Backend `.env.example` created with comprehensive production configuration
- [x] Android `local.properties.example` updated with all required variables
- [x] Google Maps API key injection configured in `build.gradle.kts`
- [x] ProGuard rules completed for release builds
- [x] `DEPLOYMENT.md` - Production deployment guide created
- [x] `DEVELOPMENT.md` - Local development setup guide created
- [x] `SSO_INTEGRATION.md` - Jahazi SSO integration guide created

### Jahazi SSO Integration ✅
- [x] Backend federation configuration via `FEDERATION_CLIENTS` environment variable
- [x] Android BuildConfig properly injects SSO configuration
- [x] Deep link handling configured for OAuth callbacks
- [x] PKCE flow implemented (no client secrets)
- [x] Account linking support (native + SSO)
- [x] Bidirectional federation (BahariMRV ↔ Jahazi)
- [x] Automatic IOTA identity provisioning for federated users

### Demo Data & Onboarding ✅
- [x] Comprehensive seed data script exists with realistic placeholder data
- [x] 7 demo accounts across all roles (Manager, Monitors, Verifier, Buyer, Finance, Admin)
- [x] Demo project with 2 sites, 2 plots, GPS coordinates
- [x] Monitoring records with evidence
- [x] Marketplace listing with sandbox credits
- [x] Financial imports (tourism revenue)
- [x] Demo data forbidden in production (`SEED_DEMO_DATA=false` enforced)

### Production Readiness ✅
- [x] Environment-based configuration with strict production validation
- [x] PostgreSQL with SSL enforced in production
- [x] HTTPS required for all production endpoints
- [x] CHAIN_MODE=iota enforced in production (no sandbox)
- [x] Security: KMS_KEK required, OIDC signing keys required
- [x] No wildcard CORS origins allowed
- [x] Comprehensive health checks and monitoring guidance

### Known Limitations & Next Steps
- [ ] Google Maps API key must be obtained by each developer/organization
- [ ] Jahazi OAuth2 client registration required (manual coordination)
- [ ] IOTA mainnet deployment pending (currently testnet-ready)
- [ ] External integrations (satellite, registry, payment) in sandbox/unavailable mode
- [ ] Release build minification testing needed (ProGuard rules ready)

---

## 🔴 Must Do (Before Production)

### 1. Configure Google Maps API Key ✅ COMPLETED
**Priority:** High  
**Component:** Android  
**File:** `android/app/src/main/AndroidManifest.xml`

**Current State:**
```xml
<meta-data android:name="com.google.android.geo.API_KEY" android:value="${googleMapsApiKey}" />
```

**Actions Completed:**
- [x] Add `GOOGLE_MAPS_API_KEY` to `local.properties.example` with documentation
- [x] Update `build.gradle.kts` to inject key:
  ```kotlin
  manifestPlaceholders["googleMapsApiKey"] = config("GOOGLE_MAPS_API_KEY")
  ```
- [x] Update manifest to use placeholder:
  ```xml
  <meta-data android:name="com.google.android.geo.API_KEY" 
             android:value="${googleMapsApiKey}" />
  ```

**Remaining Action:**
- [ ] Obtain Google Maps API key from Google Cloud Console (developer-specific)

**Impact:** Map features will fail without valid key

---

### 2. Complete ProGuard Rules (If Minification Enabled) ✅ COMPLETED
**Priority:** Medium-High  
**Component:** Android  
**File:** `android/app/proguard-rules.pro`

**Current State:** Comprehensive ProGuard rules configured

**Actions Completed:**
- [x] Added comprehensive ProGuard rules for Retrofit, Room, Hilt, Gson, OkHttp
- [x] Added rules for BahariMRV DTOs and models
- [x] Added rules for Kotlin coroutines, Google Play Services, Maps
- [x] Documented each section for maintainability

**Remaining Actions:**
- [ ] Verify if `isMinifyEnabled = true` in release build (currently false)
- [ ] Test release build thoroughly after enabling minification
- [ ] Verify API calls work in release mode
- [ ] Check database queries function correctly

**Impact:** Potential runtime crashes in release builds if minification breaks reflection

---

## 🟡 Should Do (Post-Launch Priority)

### 3. Increase Test Coverage
**Priority:** Medium  
**Component:** Android  
**Current Coverage:** 2 test files (SyncMappingTest, FormattingTest)

**Action Required:**

#### ViewModels Tests
- [ ] `HomeViewModelTest` - test project loading, dashboard state
- [ ] `MonitorViewModelTest` - test GPS capture, evidence attachment, validation
- [ ] `AuthViewModelTest` - test login, registration, SSO flow
- [ ] `CreditsViewModelTest` - test marketplace, purchase flow
- [ ] `WalletViewModelTest` - test wallet loading, document download
- [ ] `ProfileViewModelTest` - test identity provisioning, link management

#### Repository Tests
- [ ] `MonitoringRepositoryTest` - test sync logic, conflict resolution, retry policies
- [ ] `AuthRepositoryTest` - test token refresh, session management
- [ ] `ProjectRepositoryTest` - test project creation, plot management

#### Database Tests
- [ ] `MonitoringDaoTest` - test CRUD operations, queries
- [ ] `EvidenceDaoTest` - test evidence linking, upload state transitions

#### Integration Tests
- [ ] End-to-end field capture flow
- [ ] Offline-first sync scenario (capture offline → sync when online)
- [ ] Conflict resolution scenario (device A and B capture same plot)

**Target:** 60%+ code coverage on critical paths

---

### 4. Add Crash Reporting
**Priority:** Medium  
**Component:** Android

**Action Required:**
- [ ] Choose crash reporting solution:
  - Option A: Firebase Crashlytics (recommended - free, Google integration)
  - Option B: Sentry (open source friendly)
- [ ] Add dependency to `build.gradle.kts`:
  ```kotlin
  implementation("com.google.firebase:firebase-crashlytics:18.6.0")
  implementation("com.google.firebase:firebase-analytics:21.5.0")
  ```
- [ ] Add Firebase config file `google-services.json`
- [ ] Initialize in `BahariMrvApplication.onCreate()`
- [ ] Configure ProGuard mapping upload for symbolicated stack traces
- [ ] Test crash reporting in staging environment
- [ ] Set up alerting for critical crashes

**Benefits:**
- Production crash visibility
- User impact metrics
- Stack traces for debugging
- Trend analysis

---

### 5. Review WorkManager Retry Strategy
**Priority:** Medium  
**Component:** Android  
**File:** `android/app/src/main/java/ke/co/baharilink/baharimrv/sync/SyncWorker.kt`

**Current Implementation:**
```kotlin
SyncRunResult.RETRY -> if (runAttemptCount < 20) Result.retry() else Result.failure()
```

**Action Required:**
- [ ] Review retry behavior under production load
- [ ] Consider adding jitter to prevent thundering herd:
  ```kotlin
  .setBackoffCriteria(
      BackoffPolicy.EXPONENTIAL,
      WorkRequest.MIN_BACKOFF_MILLIS + Random.nextLong(0, 10000),
      TimeUnit.MILLISECONDS
  )
  ```
- [ ] Consider max backoff time (e.g., cap at 15 minutes)
- [ ] Add monitoring for failed sync attempts
- [ ] Document retry policy in code comments

**Current Assessment:** Working well, but could be optimized for scale

---

## 🟢 Nice to Have (Future Enhancements)

### 6. Add Analytics
**Priority:** Low  
**Component:** Android

**Action Required:**
- [ ] Choose analytics platform:
  - Google Analytics 4
  - Mixpanel
  - Amplitude
- [ ] Track key events:
  - User registration/login
  - Field capture completion
  - Evidence upload success/failure
  - Credit purchase flow
  - Verification completion
- [ ] Track screen views
- [ ] Set up conversion funnels:
  - Registration → First Project → First Capture
  - Credit Generation → Listing → Sale
- [ ] Add user properties (role, project count, etc.)

**Benefits:**
- Feature usage insights
- User journey optimization
- Drop-off point identification
- A/B testing capability

---

### 7. Internationalization (i18n) Preparation
**Priority:** Low  
**Component:** Android

**Action Required:**
- [ ] Audit hardcoded strings in Composables
- [ ] Extract strings to `strings.xml`:
  - Success/error messages
  - Button labels
  - Form labels
  - Empty state messages
- [ ] Use `stringResource(R.string.key)` in Compose
- [ ] Add plurals support for count-based strings
- [ ] Prepare for locale-specific date/number formatting
- [ ] Consider RTL layout support

**Target Locales (Future):**
- English (en) - primary
- Swahili (sw) - community language
- French (fr) - regional language

---

### 8. Performance Enhancements
**Priority:** Low  
**Component:** Android

**Action Required:**
- [ ] Add Compose performance monitoring
- [ ] Profile recomposition frequency in complex screens
- [ ] Add `remember` for expensive calculations
- [ ] Consider `derivedStateOf` for computed values
- [ ] Profile database query performance
- [ ] Add indexes to frequently queried fields (already good, but review)
- [ ] Consider pagination for large lists (credits marketplace, wallet entries)

---

### 9. Accessibility Audit
**Priority:** Low  
**Component:** Android

**Action Required:**
- [ ] Test with TalkBack screen reader
- [ ] Verify all interactive elements have contentDescription
- [ ] Check touch target sizes (48dp minimum - already implemented)
- [ ] Test with large font sizes
- [ ] Add semantic labels for images/icons
- [ ] Test form field navigation with external keyboard
- [ ] Verify color contrast ratios meet WCAG AA standards
- [ ] Add haptic feedback for important actions

---

### 10. Animations & Polish
**Priority:** Low  
**Component:** Android

**Action Required:**
- [ ] Add Lottie for empty states
- [ ] Add loading animations
- [ ] Add success/celebration animations (e.g., after field capture)
- [ ] Add pull-to-refresh animations
- [ ] Add page transition animations
- [ ] Add micro-interactions (button press effects, etc.)
- [ ] Consider skeleton screens for loading states

**Library:** `implementation("com.airbnb.android:lottie-compose:6.3.0")`

---

## 📊 Backend Enhancements (Lower Priority)

### 11. Increase Backend Test Coverage
**Priority:** Low  
**Component:** Go Backend  
**Current:** 4 test files (flow_test, schema_test, money_test, database_test)

**Action Required:**
- [ ] Add handler tests for each API endpoint
- [ ] Add middleware tests (auth, rate limiting, CORS)
- [ ] Add integration tests with test database
- [ ] Add end-to-end API tests
- [ ] Set up test coverage reporting (go test -cover)

**Target:** 70%+ coverage

---

### 12. API Documentation
**Priority:** Low  
**Component:** Backend

**Action Required:**
- [ ] Add OpenAPI/Swagger documentation
- [ ] Document authentication flow
- [ ] Document webhook payloads (if any)
- [ ] Document rate limits per endpoint
- [ ] Add example requests/responses
- [ ] Consider API versioning strategy

---

## 🎯 Admin Dashboard Assessment (Not Yet Done)

**Component:** Web Admin (`/admin-dashboard`)  
**Status:** Deferred for separate assessment

**When Needed:**
- [ ] TypeScript strict mode audit
- [ ] API error handling review
- [ ] Authentication token management
- [ ] Form validation completeness
- [ ] Accessibility compliance check
- [ ] Browser compatibility testing
- [ ] Performance optimization (bundle size, lazy loading)

---

## ✅ Already Complete (For Reference)

- ✅ Network timeout configuration (30s connect, 45s read/write)
- ✅ API interface definition (`Api.kt` with comprehensive DTOs)
- ✅ Backend security (CORS, rate limiting, SQL injection protection)
- ✅ Android offline-first architecture
- ✅ Database transaction safety
- ✅ Error handling with user-friendly messages
- ✅ Empty state handling across all screens
- ✅ GPS validation and accuracy checks
- ✅ SHA-256 hash verification for evidence uploads
- ✅ NetworkMonitor with capability validation
- ✅ Sync conflict resolution
- ✅ BuildConfig version display in Profile
- ✅ Professional UI redesign (wireframes C1-C4)
- ✅ Custom BahariMRV logo
- ✅ Professional login/profile screens
- ✅ Privacy policy & terms integration
- ✅ "Powered by BahariLink" branding

---

## 📝 Notes

### Testing Strategy
- Unit tests for business logic (ViewModels, Repositories)
- Integration tests for database operations
- UI tests for critical user flows (capture, sync, purchase)
- Manual testing for GPS accuracy, camera, audio recording

### Release Checklist Template
- [ ] All "Must Do" items complete
- [ ] ProGuard rules tested in release build
- [ ] Maps API key configured
- [ ] Crash reporting enabled
- [ ] Version numbers updated
- [ ] Release notes prepared
- [ ] Staged rollout plan (5% → 20% → 50% → 100%)

---

**Next Review:** After completing "Must Do" items
