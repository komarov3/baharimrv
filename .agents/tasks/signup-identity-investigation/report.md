# BahariMRV Signup, Identity Provisioning, and Project Creation — Investigation Report

**Date:** 2026-11-02  
**Scope:** backend/internal/auth, backend/internal/identity, backend/internal/mrv, backend/internal/chain, backend/internal/app, backend/internal/seed, live database baharimrv-local.db  
**Mode:** Read-only. No code was changed.

---

## Summary (answer-first)

The core flows are working as intended for the paths exercised so far. Every registered user has a corresponding `identity_provisions` row in `active` state with a sandbox DID. No user can log in without an active identity. Project creation correctly restricts to the `project_manager` and `platform_administrator` roles. The IOTA adapter in dev/test mode is an explicit in-memory sandbox that always succeeds — it does not call the real testnet. All 28 users and 10 projects in the live DB are in consistent state.

Two specific gaps exist:

1. **`identity_provisions` schema is missing the `role_cap_object_id` column** that the model defines — the migration has not run, so cap persistence silently fails.
2. **Seed path (`seed.go`) calls `Initialize()` + `EnsureProvision()` separately**, producing a redundant initial `IdentityProvision` row creation, while the registration path (`auth/service.go`) creates the row inline in a transaction and then also calls `EnsureProvision()`. This is not a correctness bug but it is an inconsistency in the pattern.

Neither gap blocks the flows that have actually run. No user or project is in inconsistent state.

---

## Evidence

### 1. Signup / Registration Flow

**File:** `backend/internal/auth/service.go` — `func (s *Service) Register()`  
**File:** `backend/internal/auth/handlers.go` — `func (h *Handler) Register()`

The full registration path is:

```
POST /api/v1/auth/register
  → auth.Handler.Register (handlers.go:34)
    → auth.Service.Register (service.go:52)
      1. Validates email, password (12–128 chars), display name, role
      2. Role whitelist: project_manager, field_monitor, independent_verifier, carbon_buyer
         (admin and finance_operator cannot self-register — correct)
      3. Bcrypt password hash (cost 12)
      4. DB Transaction: creates users row (status=identity_pending) + identity_provisions row (state=identity_pending)
      5. Calls identity.EnsureProvision(ctx, user.ID) — BLOCKS registration
      6. If EnsureProvision returns error → returns HTTP 202 (accepted, not active)
      7. If EnsureProvision succeeds → launches goroutine: IssueCapForUser (non-blocking)
      8. Creates session token → returns HTTP 201 with token, user, identity
```

**Is provisioning called on every registration path?** Yes, unconditionally. There is no early-return between the transaction commit and the `EnsureProvision` call at `service.go:74`. The `EnsureProvision` call is synchronous — if it fails, the user gets HTTP 202, not a session, and cannot log in.

**Can a user reach `active` without successful identity?**

The `Authenticate` method (`service.go:113`) queries:
```go
s.db.Where("id = ? AND status = ?", session.UserID, models.IdentityActive).First(&user)
```
Session creation only happens after `EnsureProvision` succeeds (line 78) and only after `user.Status = models.IdentityActive` is set in the `EnsureProvision` transaction. **Login is blocked unless `user.status = 'active'` in the DB.**

The `Login` method also checks:
```go
if err != nil || provision.State != models.IdentityActive || user.Status != models.IdentityActive {
    return Result{...}, identity.ErrProvisioning
}
```
There is a dual guard: both `users.status` and `identity_provisions.state` must be `active`.

**Verdict: No path can produce an active user without a completed identity provision.** ✓

---

### 2. Identity Provisioning — `identity/service.go`

**State machine:**

```
identity_pending (initial)
  → EnsureProvision()
    → generates key material via EnvelopeCustody
    → calls chain.ProvisionIdentity(ctx, userID, publicKey)
      [sandbox: deterministic DID from SHA-256 of userID+publicKey — always succeeds]
      [iota: real testnet call via executor]
    → validates result fields (DID, DIDDocumentRef, Address, TransactionRef, Network all non-empty)
    → DB Transaction:
        UPDATE identity_provisions SET state='active', did=..., activated_at=..., ...
        UPDATE users SET status='active', ...
  → state: active

  → on any failure in key gen / chain call / result validation / DB write:
    → fail() called: sets identity_provisions.state = 'identity_failed'
                     sets users.status = 'identity_failed'
    → returns ErrProvisioning
```

**Is `Initialize()` still used?**

`Initialize()` (`identity/service.go:56`) simply inserts a row with `state=identity_pending`. It is called only from `seed.go:account()` (line ~80). The registration path in `auth/service.go` does **not** call `Initialize()` — it creates the provision row inline in its own transaction (lines 66–72), then calls `EnsureProvision()`. This is slightly inconsistent: seed uses a two-step create-then-provision, while registration uses a single transaction for create + then a separate `EnsureProvision` call. There is no functional bug — `EnsureProvision` is idempotent and checks for an existing row first.

**SPEC §6.1 requirement 7:** _"The account becomes active only when wallet, DID document, and required role provisioning succeed."_

The DID document and wallet (key) are part of `EnsureProvision`. Role cap issuance (`IssueCapForUser`) is launched in a background goroutine and is **non-blocking** — it does not gate activation. This means a user becomes `active` without confirmed cap issuance. This is a partial divergence from the spec letter, but the code comments explicitly document this as intentional ("non-blocking: registration succeeds even if cap issuance is temporarily unavailable"). The spec says "required role provisioning succeed" — the degree to which cap issuance is "required" is an open design question.

**Schema gap — `role_cap_object_id` column missing:**

The model defines:
```go
// models/models.go:106
RoleCapObjectID string `gorm:"column:role_cap_object_id" json:"roleCapObjectId,omitempty"`
```

But the live DB schema for `identity_provisions` is:
```sql
CREATE TABLE identity_provisions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL UNIQUE ..., 
  state TEXT, did TEXT, ..., last_provisioned_at TIMESTAMP);
```

The column `role_cap_object_id` is **absent** from the schema. Any `IssueCapForUser` call that tries to persist the cap object ID via:
```go
_ = s.db.Model(&models.IdentityProvision{}).Where("id = ?", provision.ID).
    Update("role_cap_object_id", capRes.CapObjectID)
```
will silently fail (the `_` discards the error). This means cap assignments are never persisted to the DB. The cap is issued on the sandbox adapter (and would be on the real chain), but the reference is lost at restart.

**Verdict:** Identity provisioning works correctly for DID/wallet. Cap object IDs are not persisted due to missing migration. No user is stuck in `identity_pending` or `identity_failed`.

---

### 3. Project Creation — `mrv/project.go`

**File:** `backend/internal/mrv/project.go` — `func (s *Service) CreateProject()`

```go
func (s *Service) CreateProject(userID string, input ProjectInput, sandboxDemo bool) (models.ConservationProject, error) {
    var user models.User
    if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
        return ..., ErrForbidden  // user not found → forbidden
    }
    if user.Role != models.RoleManager && user.Role != models.RoleAdmin {
        return ..., ErrForbidden  // role check
    }
    ...
```

**Does it enforce active identity?** The role check gates on `user.Role` but **does NOT separately check `user.Status == active`**. However, this is not an exploitable gap: a user with `status != active` cannot have a valid session (see `Authenticate` above which enforces `status = active`). Any bearer token reaching `CreateProject` belongs to an active-status user by construction. The middleware check (`RequireLocal` in `handlers.go:115`) verifies the session token, which is only issued after provisioning completes.

**Validation applied:**
- Role whitelist: `project_manager` or `platform_administrator` only
- Non-empty project name
- `communityShareBPS` 0–10,000
- Methodology must exist
- Sites and plots created atomically in a DB transaction
- Each plot: site must be named in the `sites` list, valid area (numeric), coordinates in range (lat −90..90, lon −180..180)

**Atomicity:** All sites and plots are created in a single `s.db.Transaction(...)`. A failure in any plot creation rolls back the entire project + sites. ✓

**Verdict:** Project creation is correctly role-gated. Identity is implicitly enforced through session middleware. Plots and sites are atomic.

---

### 4. IOTA Adapter in Test/Dev Mode

**File:** `backend/internal/chain/sandbox.go`

The sandbox adapter is an in-memory implementation. Its `ProvisionIdentity` method:
```go
func (s *Sandbox) ProvisionIdentity(_ context.Context, userID string, publicKey []byte) (IdentityResult, error) {
    // ...
    digest := sha256.Sum256(append([]byte(userID), publicKey...))
    identifier := hex.EncodeToString(digest[:16])
    result := IdentityResult{
        DID:            "did:iota:sandbox:" + identifier,
        DIDDocumentRef: "sandbox-document-" + identifier,
        Address:        "sandbox-address-" + identifier,
        TransactionRef: "sandbox-identity-transaction-" + identifier,
        Network:        "sandbox",
    }
```

This is deterministic (same userID + key → same DID), idempotent (checks if already exists), and **never calls the real IOTA testnet**. All DIDs in the DB have the `did:iota:sandbox:` prefix, confirming only the sandbox adapter has ever run.

**Is it possible to accidentally run in live mode?** `app.go:ChainAdapter()` enforces:
```go
if cfg.ChainMode != "iota" {
    return chain.NewSandbox(), nil
}
if cfg.IOTAExecutorURL == "" || cfg.IOTAPackageID == "" {
    return nil, errors.New("CHAIN_MODE=iota requires IOTA_EXECUTOR_URL and IOTA_PACKAGE_ID")
}
if cfg.IOTANetwork != "testnet" {
    return nil, errors.New("this release anchors on IOTA Rebased testnet only")
}
```
The test `TestLiveChainModeNeverSilentlyDowngrades` in `flow_test.go` validates this guard. **No silent downgrade is possible.** ✓

**Verdict:** Sandbox mode is explicit, deterministic, non-network, and guarded against accidental live mode. Real testnet calls are only made when `CHAIN_MODE=iota` with both executor URL and package ID configured.

---

### 5. Live Database State Consistency

**Database:** `backend/baharimrv-local.db`

```
users:               28 total — 28 active, 0 pending, 0 failed
identity_provisions: 28 total — 28 active, 0 pending, 0 failed
users without provision row: 0
users where status ≠ provision.state: 0
projects: 10 total
projects.sandbox_demo = 0 (live): 0 — all 10 are sandbox/demo
projects where manager lacks active identity: 0
```

**User breakdown by role:**
- `project_manager`: 22
- `field_monitor`: 2
- `independent_verifier`: 1
- `carbon_buyer`: 1
- `finance_operator`: 1
- `platform_administrator`: 1

**Project managers without any project:** 12 out of 22. These are test/probe accounts (e.g. `probe.1793567122@test.local`, `route.test.1793568149@test.local`) created by E2E test runs. Their identities are fully provisioned (all `active`). They registered successfully and have DIDs — they simply never created a project. This is expected behavior.

**All 10 projects have `sandbox_demo = 1`**, indicating the DB has been populated by seeded demo data and test runs, not production data.

**All identity_provisions rows use `network = 'sandbox'`**, consistent with only the sandbox adapter having ever run.

**Verdict:** The database is in fully consistent state. No orphaned users, no stuck provisions, no projects owned by users with inactive identities.

---

### 6. Auth Flow — SSO/OIDC Path

**File:** `backend/internal/auth/federation.go` — `func (s *FederationService) createFederatedAccount()`  
**File:** `backend/internal/auth/federation.go` — `func (s *FederationService) Login()`

The federation/SSO flow (for BahariLink users signing in via OIDC) also wires up identity provisioning:

```go
func (s *FederationService) createFederatedAccount(ctx context.Context, claims ExternalClaims) (models.ExternalIdentity, error) {
    // ...
    provision := models.IdentityProvision{..., State: models.IdentityPending, ...}
    link := models.ExternalIdentity{...}
    if err := s.db.Transaction(func(tx *gorm.DB) error {
        tx.Create(&user); tx.Create(&provision); tx.Create(&link)
    }); err != nil { ... }
    if _, err := s.identities.EnsureProvision(ctx, user.ID); err != nil {
        return link, err  // ← provisioning failure propagates
    }
    return link, nil
}
```

The federation login path also guards the same way:
```go
var user models.User
if err := s.db.Where("id = ? AND status = ?", link.UserID, models.IdentityActive).First(&user).Error; err != nil {
    return Result{}, identity.ErrProvisioning
}
provision, err := s.identities.ForUser(user.ID)
if err != nil || provision.State != models.IdentityActive {
    return Result{}, identity.ErrProvisioning
}
```

Federated accounts are assigned `role = RoleBuyer` by default. The role can be updated via hub role-healing on next login (`service.go:Login()` launches a goroutine to call `ResolveRoleFromHub`).

**Verdict:** The SSO path is consistent with the direct registration path — provisioning is mandatory and any failure blocks login.

---

### 7. E2E Test Coverage — `flow_test.go`

The test suite covers:

| Test | Coverage |
|---|---|
| `TestRegistrationProvisionsIdentityWithoutExposingKeyMaterial` | Full registration → DID issued → `identity.state = active` → DID prefix check → no key leakage |
| `TestRegistrationRefusesRolesThePublicMayNotClaim` | Admin role cannot self-register |
| `TestSessionIsRequiredForConservationRecords` | Unauthenticated requests blocked |
| `TestCompletionIsRefusedUntilItsEvidenceExists` | Evidence gate before completion claim |
| `TestSyncReplaysAndConflictsWithoutOverwritingEvidence` | Sync idempotency |
| `TestForgedMonitorSignatureIsRejected` | Attestation DID must match caller |
| `TestIndependentVerificationCreatesASecondMilestone` | Two-milestone provenance |
| `TestProofStatesStayHonestAcrossBothMilestones` | Proof type matches milestone |
| `TestStepUpTicketsAreActionBoundAndSingleUse` | Step-up cannot be reused or cross-used |
| `TestFederationRefusesAnIssuerThatWasNeverConfigured` | Federation issuer guard |
| `TestAnchorRetryPreservesTheOriginalCommitment` | Anchor retry idempotency |
| `TestLiveChainModeNeverSilentlyDowngrades` | Live chain requires config |
| `TestContextCancellationDoesNotFabricateAReceipt` | Cancelled request safety |

**Gaps in test coverage:**
- No test verifies that `Login` is blocked for a user whose `status = identity_pending` (it is guarded, but not explicitly tested via a scenario where provisioning was partially completed).
- No test covers the federation SSO path's provisioning requirement end-to-end (only that unknown issuers are rejected).
- No test verifies that `role_cap_object_id` is actually persisted after cap issuance (this gap is hidden because the column doesn't exist in the DB yet).
- No test exercises the `RetryIdentity` or `RecoverIdentity` endpoints.

---

## Conclusions

| Question | Answer |
|---|---|
| Is provisioning called on every registration path? | **Yes** — both direct (`auth/service.go`) and federated (`auth/federation.go`) paths call `EnsureProvision()` synchronously |
| Can a user reach `active` status without successful identity provision? | **No** — `EnsureProvision` sets `users.status` to `active` in the same transaction as the provision row, and `Authenticate()` enforces `status = active` |
| Does project creation gate on identity status? | **Implicitly yes** — the route middleware blocks sessions that don't belong to active users; `CreateProject` itself only checks role |
| Is the IOTA adapter doing real work or simulating success? | **Simulating** — explicit in-memory sandbox, deterministic, no network calls |
| Are there any users or projects in inconsistent state? | **No** — 28/28 users active, 28/28 provisions active, all 10 projects have active-identity managers |
| Does the E2E test cover these paths end-to-end? | **Partially** — registration+provisioning is well covered; retry/recover endpoints and federated provisioning are not |

---

## Recommendations

### Fix: Apply migration to add `role_cap_object_id` column

The `identity_provisions` table is missing the `role_cap_object_id` column defined in `models.go:106`. Calls to persist cap object IDs silently fail. This should be added via a migration:

```sql
ALTER TABLE identity_provisions ADD COLUMN role_cap_object_id TEXT NOT NULL DEFAULT '';
```

The consequence today is minor (sandbox caps are not persisted, but the in-memory sandbox re-issues them consistently). On a real chain, cap object IDs would be lost on restart, breaking the idempotency guard in `IssueMonitorCapForUser` (`if provision.RoleCapObjectID != "" { return }`).

### Clarify: Spec §6.1 requirement 7 vs. non-blocking cap issuance

The spec says _"The account becomes active only when wallet, DID document, and required role provisioning succeed."_ But cap issuance is non-blocking (`go s.identity.IssueCapForUser(...)`). This is intentional per the code comment but diverges from the strict spec reading. Consider either updating the spec to say "DID and wallet provisioning are required; role capability issuance is best-effort" or making cap issuance part of the activation transaction.

### Consider: Unify seed and registration provisioning pattern

`seed.go` calls `identities.Initialize(user.ID)` + `identities.EnsureProvision(ctx, user.ID)` (two steps). The registration path creates the `identity_provisions` row inline in its own transaction and then calls `EnsureProvision`. The `Initialize()` function is now only used by seed — consider removing it and having seed use the same single-transaction pattern as registration, to reduce the risk of divergence.

### Add tests for:
- Login rejection for a user whose provision is in `identity_pending` state
- `RetryIdentity` and `RecoverIdentity` handler paths
- That `role_cap_object_id` is persisted after the migration is applied
