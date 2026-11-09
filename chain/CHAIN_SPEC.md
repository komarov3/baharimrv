# conservation_chain — Full On-Chain Design Spec
# BahariMRV IOTA Rebased Testnet Package

**Status:** ✅ Implemented — November 2026  
**Test result:** 15/15 Move unit tests passing  
**Build:** `iota move build --skip-fetch-latest-git-deps` clean

---

## 1. Design principles

1. **Privacy first.** No PII on-chain. Tourist identities are DIDs. GPS coordinates are hashed. Evidence is Merkle roots. Individual contribution amounts are not on-chain — only Merkle roots of contribution breakdowns.
2. **Corrections are additive.** Frozen objects are never deleted or mutated. Corrections create new objects that reference old ones via `previousId` fields and emit supersession events.
3. **Proof chain completeness.** A verifier with only a tourist's ConservationCertificate object ID can traverse to PlantingRecord → ProvenanceAnchor → RestorationClaimAnchor → ProjectImpactLedger without touching any database.
4. **Shared ledger, capability-gated writes.** ProjectImpactLedger is defined in `conservation_chain` (BahariMRV) but Jahazi can write to it via a `ContributionCap` issued by BahariMRV's AdminCap.
5. **No floats on-chain.** All monetary values in cents (u64). CO₂ in milligrams (× 1000). Area in millionths of hectares (× 1_000_000). Survival as integer percentage × 100.

---

## 2. Object graph

```
ProjectImpactLedger (shared, mutable)
  ├── PlantingLedgerEntry × N (frozen, appended by BahariMRV)
  ├── ContributionLedgerEntry × N (frozen, appended by Jahazi via ContributionCap)
  │
  ↑ referenced by:
  ├── PlantingRecord × N (frozen)
  │     ├── → ProvenanceAnchor (from notary.move)
  │     └── → RestorationClaimAnchor (optional, if Jahazi-funded)
  │
  └── ContributionCap (held by Jahazi executor address)

RestorationClaimAnchor (frozen, created by BahariMRV when Jahazi claim arrives)
  └── → matched PlantingRecord (via matchedPlantingRecordId, set on fulfillment)

RestorationFulfillmentRecord (frozen, created when planting matches claim)
  ├── → RestorationClaimAnchor
  ├── → PlantingRecord
  └── → ProvenanceAnchor

ConservationCertificate (soul-bound, one per tourist, created by Jahazi)
  ├── → PlantingRecord
  ├── → RestorationClaimAnchor
  └── → ProvenanceAnchor
```

---

## 3. Module inventory

| Module | Owner | Description |
|---|---|---|
| `roles.move` | BahariMRV | AdminCap, ManagerCap, MonitorCap, VerifierCap, ContributionCap |
| `notary.move` | BahariMRV | ProvenanceAnchor — cryptographic commitment anchoring |
| `identity.move` | BahariMRV | DID provisioning for users, projects, operators |
| `project_ledger.move` | BahariMRV | ProjectImpactLedger, PlantingLedgerEntry, ContributionLedgerEntry |
| `planting_record.move` | BahariMRV | PlantingRecord — privacy-safe field data summary |
| `restoration.move` | BahariMRV | RestorationClaimAnchor, RestorationFulfillmentRecord |
| `certificates.move` | BahariMRV | ConservationCertificate — soul-bound tourist impact NFT |

---

## 4. Struct definitions

### 4.1 `ProjectImpactLedger` (shared, mutable)

```move
struct ProjectImpactLedger has key {
    id: UID,
    projectDID: String,           // did:iota:testnet:project:<mrvProjectId>
    mrvProjectId: String,         // BahariMRV backend UUID
    jahaziProjectDID: String,     // did:iota:testnet:project:<jahaziProjectId> (set on linking)
    // Running totals — updated atomically by entry functions
    totalSeedlingsPlanted: u64,
    totalSeedlingsAlive: u64,
    totalTouristContribCents: u64,  // USD cents
    totalContributors: u64,
    totalVerifiedPlantings: u64,
    totalCo2eMilligrams: u64,       // kg × 1000 for precision without floats
    plantingEntryCount: u64,
    contributionEntryCount: u64,
    lastUpdatedMs: u64,
}
```

### 4.2 `PlantingLedgerEntry` (frozen)

```move
struct PlantingLedgerEntry has key {
    id: UID,
    ledgerId: ID,
    plantingRecordId: ID,
    provenanceAnchorId: ID,
    seedlingsAdded: u64,
    aliveAdded: u64,
    co2eMilligramsAdded: u64,
    recordedAtMs: u64,
}
```

### 4.3 `ContributionLedgerEntry` (frozen)

```move
struct ContributionLedgerEntry has key {
    id: UID,
    ledgerId: ID,
    claimAnchorId: ID,
    amountCents: u64,
    contributorCount: u64,
    recordedAtMs: u64,
}
```

### 4.4 `ContributionCap` (transferable, held by Jahazi executor)

```move
struct ContributionCap has key, store {
    id: UID,
    ledgerId: ID,               // Scoped to one project ledger
    jahaziPlatformDID: String,
}
```

### 4.5 `PlantingRecord` (frozen)

```move
struct PlantingRecord has key {
    id: UID,
    projectDID: String,
    ledgerId: ID,
    provenanceAnchorId: ID,              // Required
    claimAnchorId: Option<ID>,           // Optional — only for Jahazi-funded plantings
    // Field data — aggregated, privacy-safe
    species: String,
    seedlingsPlanted: u64,
    seedlingsAlive: u64,
    survivalPctHundredths: u64,          // 9500 = 95.00%
    areaHaMillionths: u64,               // 500000 = 0.500000 ha
    methodologyVersion: String,
    reportingPeriodStartMs: u64,
    reportingPeriodEndMs: u64,
    // Evidence — hashes only
    evidenceRoot: String,                // Merkle root of field evidence SHA-256s
    satelliteCorrobHash: String,         // Hash of satellite proof (empty if unavailable)
    // Verification
    verifierDID: String,                 // Empty if only achieved_unverified
    verifierAttestationHash: String,
    // Correction chain
    previousRecordId: Option<ID>,        // Set when correcting an earlier record
    superseded: bool,
    // Metadata
    recordedAtMs: u64,
    mrvActivityId: String,               // BahariMRV backend UUID
}
```

### 4.6 `RestorationClaimAnchor` (frozen)

```move
struct RestorationClaimAnchor has key {
    id: UID,
    jahaziClaimId: String,          // Jahazi's claim ID (e.g. "rc_abc123")
    jahaziProjectDID: String,
    mrvProjectDID: String,
    periodStartMs: u64,
    periodEndMs: u64,
    totalCostCents: u64,
    totalUnits: u64,                // Seedlings claimed
    species: String,
    contributorCount: u64,
    contributorMerkleRoot: String,  // Merkle root of contributor breakdown — no PII
    locationHash: String,           // SHA-256 of "lat|lng" string
    jahaziPlatformDID: String,      // Jahazi platform DID that signed this claim
    // Status events are emitted; status is tracked via events, not mutation
    // (object is frozen — status updated via ClaimMatchedEvent)
}
```

### 4.7 `RestorationFulfillmentRecord` (frozen)

```move
struct RestorationFulfillmentRecord has key {
    id: UID,
    claimAnchorId: ID,
    plantingRecordId: ID,
    provenanceAnchorId: ID,
    actualUnitsPlanted: u64,
    fulfilledAtMs: u64,
    mrvProjectDID: String,
    certificatesIssuedCount: u64,   // Updated at issuance time
}
```

### 4.8 `ConservationCertificate` (soul-bound)

```move
struct ConservationCertificate has key {
    id: UID,
    certificateCode: String,         // Jahazi cert code (e.g. "JZ-A3B7C2D1")
    holderName: String,
    holderDID: String,               // did:iota:testnet:<userId>
    projectName: String,
    projectDID: String,
    species: String,
    seedlingsFunded: u64,
    co2eMilligrams5yr: u64,          // 5-yr estimate × 1000
    plantingDateMs: u64,
    // Full proof chain references
    plantingRecordId: ID,            // → PlantingRecord
    claimAnchorId: Option<ID>,       // → RestorationClaimAnchor
    provenanceAnchorId: ID,          // → ProvenanceAnchor
    fulfillmentRecordId: Option<ID>, // → RestorationFulfillmentRecord
    // Funding state
    fundingStatus: String,           // "certified" | "disbursed"
    issuedAtMs: u64,
}
```

---

## 5. Entry function inventory

### `project_ledger.move`

| Function | Cap required | Description |
|---|---|---|
| `create_ledger` | AdminCap | Create + share ProjectImpactLedger for a project |
| `link_jahazi` | AdminCap | Set jahaziProjectDID on an existing ledger |
| `issue_contribution_cap` | AdminCap | Issue ContributionCap to Jahazi executor address |
| `revoke_contribution_cap` | AdminCap | Delete a ContributionCap |
| `append_planting_entry` | ManagerCap | Append PlantingLedgerEntry + increment totals |
| `append_contribution_entry` | ContributionCap | Append ContributionLedgerEntry + increment totals |

### `planting_record.move`

| Function | Cap required | Description |
|---|---|---|
| `create_planting_record` | ManagerCap | Freeze PlantingRecord + emit PlantingRecordCreated |
| `emit_superseded` | AdminCap | Emit PlantingRecordSuperseded event |

### `restoration.move`

| Function | Cap required | Description |
|---|---|---|
| `anchor_restoration_claim` | AdminCap | Freeze RestorationClaimAnchor + emit ClaimAnchored |
| `record_fulfillment` | ManagerCap | Freeze RestorationFulfillmentRecord + emit FulfillmentRecorded |
| `emit_claim_matched` | AdminCap | Emit ClaimMatchedEvent (links claim to planting record) |

### `certificates.move`

| Function | Cap required | Description |
|---|---|---|
| `issue_certificate` | AdminCap | Transfer soul-bound ConservationCertificate to tourist |
| `invalidate_certificate` | AdminCap | Emit CertificateInvalidated event |

---

## 6. Event inventory

### `project_ledger.move`

- `LedgerCreated { ledgerId, projectDID, mrvProjectId }`
- `JahaziLinked { ledgerId, jahaziProjectDID }`
- `PlantingEntryAppended { ledgerId, plantingRecordId, seedlingsAdded, aliveAdded }`
- `ContributionEntryAppended { ledgerId, claimAnchorId, amountCents, contributorCount }`

### `planting_record.move`

- `PlantingRecordCreated { recordId, projectDID, ledgerId, species, seedlingsPlanted, claimAnchorId, provenanceAnchorId }`
- `PlantingRecordSuperseded { oldRecordId, newRecordId, projectDID }`

### `restoration.move`

- `ClaimAnchored { claimAnchorId, jahaziClaimId, mrvProjectDID, totalUnits, contributorCount }`
- `ClaimMatchedEvent { claimAnchorId, plantingRecordId, projectDID }`
- `FulfillmentRecorded { fulfillmentId, claimAnchorId, plantingRecordId, actualUnits }`

### `certificates.move`

- `CertificateIssued { certificateCode, holderDID, projectDID, species, seedlingsFunded, plantingRecordId }`
- `CertificateInvalidated { certificateCode, holderDID, reason }`

---

## 7. Executor service operations (Go → chain) — ✅ IMPLEMENTED

The executor service is at `BahariMRV/executor/` (TypeScript, `@iota/iota-sdk`). All 11 operations are implemented. The BahariMRV Go `chain.IOTA` adapter sends HTTP POST to `IOTA_EXECUTOR_URL` and the executor builds + submits Move PTBs.

| Operation | Called when | Move call | Status |
|---|---|---|---|
| `provision_identity` | User registered | `identity::provision_identity` | ✅ |
| `anchor` | Provenance event confirmed | `notary::anchor_sponsored` | ✅ |
| `create_ledger` | Project created | `project_ledger::create_ledger` | ✅ |
| `link_jahazi` | Project linked to Jahazi | `project_ledger::link_jahazi` | ✅ |
| `issue_contribution_cap` | Project-Jahazi link established | `project_ledger::issue_contribution_cap` | ✅ |
| `anchor_restoration_claim` | RestorationClaim received from Jahazi | `restoration::anchor_restoration_claim` | ✅ |
| `planting_record` | Activity anchored (provenance confirmed) | `planting_record::create_planting_record` | ✅ |
| `planting_entry` | PlantingRecord created | `project_ledger::append_planting_entry` | ✅ |
| `claim_fulfillment` | Planting matches restoration claim | `restoration::record_fulfillment` | ✅ |
| `contribution_entry` | Jahazi certificates issued | `project_ledger::append_contribution_entry` | ✅ |
| `issue_certificate` | Certificate issued per tourist | `certificates::issue_certificate` | ✅ |

---

## 8. Privacy design — contributor Merkle tree

The `contributorMerkleRoot` in `RestorationClaimAnchor` is computed as:

```
for each contributor in contributorBreakdown:
    leaf = SHA256(touristDID + ":" + amountUsdCents + ":" + seedlingsEquivalent + ":" + nonce)

tree = MerkleTree(leaves, SHA256)
contributorMerkleRoot = tree.root
```

A tourist proves inclusion by presenting:
- Their leaf preimage: `(touristDID, amountCents, seedlings, nonce)`
- Their Merkle proof path

Jahazi retains the full tree off-chain. Neither the tourist's DID nor their amount is stored on-chain in plaintext.

---

## 9. Correction / supersession flow

When a monitoring correction is needed after a PlantingRecord is anchored:

```
1. PM submits corrected monitoring record via field app
2. BahariMRV creates new ProvenanceEvent (claimType: "achieved_unverified" or "achievement_verified")
3. New ProvenanceAnchor created with previousEventId pointing to old anchor
4. New PlantingRecord created with:
     previousRecordId: <old PlantingRecord object ID>
     superseded: false  (the NEW record)
5. emit_superseded(admin, oldRecordId, newRecordId) called
6. ProjectImpactLedger adjusted:
     totalSeedlingsPlanted -= old.planted, += new.planted
     totalSeedlingsAlive   -= old.alive,   += new.alive
7. Old PlantingRecord stays on-chain with superseded: false
   (it's frozen — superseded flag can only be set at creation time)
   The event record is the supersession proof.
```

Note: because frozen objects can't be mutated, the `superseded: bool` on `PlantingRecord` is set at creation time for the original (always `false`) and new records reference the old via `previousRecordId`. The `PlantingRecordSuperseded` event is the authoritative signal that a record has been replaced.

---

## 10. Go backend changes — ✅ IMPLEMENTED

All new chain operation types and methods are implemented in:
- `internal/chain/adapter.go` — 7 new interface methods
- `internal/chain/iota.go` — `CreateLedger`, `LinkJahazi`, `IssueContributionCap`, `AnchorRestorationClaim`, `CreatePlantingRecord`, `AppendPlantingEntry`, `RecordFulfillment`
- `internal/chain/sandbox.go` — sandbox stubs for all new operations
- `internal/jahazi/chain_ops.go` — `ChainOps` service wiring lifecycle events to chain calls
- `internal/models/models.go` — `ConservationProject.LedgerObjectID`, `RestorationRequest.ChainAnchorObjectID`
- DB migrations 000006 — new chain object ID columns

---

## 11. Jahazi backend changes — ✅ IMPLEMENTED

- `issueChainCertificatesAsync()` called non-blocking from `verifyAndIssueCertificates()`
- `BahariLinkClientService.appendContributionEntry()` — POSTs to `POST /api/v1/chain/contribution-entry`
- `BahariLinkClientService.issueCertificateOnChain()` — POSTs to `POST /api/v1/chain/issue-certificate`
- `ConservationProject.iotaLedgerObjectId` — Prisma schema field added
iotaLedgerObjectId  String?  // IOTA ProjectImpactLedger object ID
```

---

## 12. Test coverage required

## 12. Test coverage — ✅ IMPLEMENTED (15/15 passing)

All tests are in `tests/conservation_chain_tests.move`. Run with:
```bash
iota move test --skip-fetch-latest-git-deps
# Test result: OK. Total tests: 15; passed: 15; failed: 0
```

| Test | Description | Status |
|---|---|---|
| `test_issue_manager_cap` | ManagerCap issuance and DID check | ✅ |
| `test_revoke_manager_cap` | Cap deletion confirmed | ✅ |
| `test_issue_verifier_cap` | VerifierCap issuance | ✅ |
| `test_three_claim_types` | evidence_submitted, achieved_unverified, achievement_verified | ✅ |
| `test_notary_rejects_duplicate` | EDuplicateAnchor on same event_id | ✅ |
| `test_anchor_via_manager_cap` | Manager-cap-gated anchor path | ✅ |
| `test_provision_multiple_identities` | 3 identities provisioned | ✅ |
| `test_identity_rejects_duplicate` | EDuplicateDID on same user_id | ✅ |
| `test_create_and_link_ledger` | Ledger creation + Jahazi DID link | ✅ |
| `test_planting_entries_accumulate` | Two planting entries, running totals correct | ✅ |
| `test_contribution_cap_flow` | ContributionCap issued, contribution entry appended | ✅ |
| `test_contribution_cap_rejects_wrong_ledger` | ELedgerMismatch on wrong ledger | ✅ |
| `test_anchor_restoration_claim` | RestorationClaimAnchor creation | ✅ |
| `test_record_fulfillment` | RestorationFulfillmentRecord creation | ✅ |
| `test_identity_provision_plus_anchor_plus_ledger` | Full flow: identity → cap → anchor → ledger entry | ✅ |

---

## 13. Deployment status

| Step | Status |
|---|---|
| Move sources written (7 modules) | ✅ |
| Tests passing (15/15) | ✅ |
| Executor service built (`BahariMRV/executor/`) | ✅ |
| BahariMRV chain adapter extended (7 new ops) | ✅ |
| Jahazi chain wiring (`issueChainCertificatesAsync`) | ✅ |
| `iota client publish` to testnet | 🟡 Run `bash chain/scripts/deploy-testnet.sh` after `iota client faucet` |
| Set `IOTA_PACKAGE_ID` + `IOTA_EXECUTOR_URL` in `.env` | 🟡 After deploy |
| Set `CHAIN_MODE=iota` | 🟡 After deploy |
