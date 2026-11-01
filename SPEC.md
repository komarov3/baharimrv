# BahariMRV Product and Technical Specification

**Status:** Draft for clarification, not approved for implementation  
**Date:** 2026-10-30  
**Platform position:** BahariMRV is the fourth BahariLink platform and its conservation module.  
**Primary references:** EcoTrack active codebase and only the Conservation section of the BahariLink wireframe, screens C1-C4.

## 1. Product definition

BahariMRV is a mobile-first conservation monitoring, reporting, verification, provenance, and blue-carbon platform for Kenyan coastal projects.

It will reuse EcoTrack's Android, Go, PostgreSQL, offline-sync, IOTA wallet, sponsored-transaction, role-capability, and document-service patterns. It will not reuse plastic-waste workflows, EPR claims, material passports, fixed attribution formulas, or simulated verification behavior.

Every registered BahariMRV user must receive a working IOTA testnet identity. BahariMRV supports two separate provenance milestones: a manager-attested `achieved_unverified` completion claim and an independently verified conservation achievement. The second does not erase or disguise the first. Authorized users can generate a human-readable and machine-verifiable proof package on demand, similar in delivery to EcoTrack's Chain of Custody documents but backed by real evidence and chain verification. Official verified-achievement proofs require the independent verification milestone.

## 2. Scope

### 2.1 In scope

1. IOTA testnet identity provisioning for every registered user, across every supported sign-up method.
2. Role-based conservation project, site, plot, participant, and verifier management.
3. C1 Project Dashboard.
4. C2 offline MRV field capture and synchronization.
5. C3 verification pipeline, evidence documents, satellite corroboration, third-party audit, and registry issuance tracking.
6. IOTA provenance for verified conservation achievements and subsequent custody or status changes.
7. On-demand achievement proof, evidence manifest, Chain of Custody history, QR verification, and independent chain checks.
8. C4 blue-carbon marketplace using registry-backed credits only.
9. Project wallet, revenue allocation, audit history, and retirement certificates.
10. Responsive verifier, buyer, administrator, and public-proof web experiences.
11. English and Swahili-ready interface architecture, with English as the initial complete locale.

### 2.2 Out of scope for the first release

1. IOTA mainnet deployment.
2. Plastic collection, recycling, EPR, or other EcoTrack domain features.
3. Features drawn from non-conservation sections of the wireframe.
4. Issuing carbon credits before an external registry issues them.
5. Treating estimated carbon impact, a payment, a database status, or an IOTA transaction alone as proof of real-world restoration.
6. Live registry, satellite, payment, or custody integrations without provider credentials and an approved test environment. These integrations must have explicit unavailable or sandbox states, never fabricated success states.

## 3. Product principles

1. **Identity is mandatory.** A user is not fully activated until their IOTA identity provisioning has succeeded.
2. **Evidence precedes claims.** A submitted activity is not a verified achievement.
3. **Verification is explicit.** The system records who verified which evidence, under which methodology, and when.
4. **The chain is checked.** A transaction hash in PostgreSQL is not sufficient proof. Public verification queries IOTA and compares the anchored commitment.
5. **Offline work is preserved.** Field staff can capture signed records without connectivity and synchronize later without losing evidence.
6. **Privacy by default.** Public anchors contain commitments and minimal public metadata, not personal data, precise sensitive locations, private media, or payment details.
7. **Corrections are additive.** Anchored records are never rewritten. Corrections create linked superseding records and preserve the original history.
8. **No double counting.** Registry serials, project vintages, credit supply, sale, transfer, and retirement must reconcile.
9. **Honest degraded states.** Chain, satellite, registry, storage, or payment failures remain visible and retryable. They never become automatic success.

## 4. Users and permissions

| Role | Main capabilities |
|---|---|
| Community Project Manager | Create and manage projects, sites, plots, activities, monitoring assignments, documents, reports, proof requests, wallet views, and verifier assignments |
| Field Monitor | View assigned plots, capture signed monitoring records and evidence offline, synchronize, and view own submission history |
| Independent Verifier | Review assigned evidence, request corrections, approve or reject achievements, sign attestations, and issue verification statements |
| Carbon Buyer | Browse registry-backed inventory, purchase through the enabled payment adapter, view custody, retire credits, and download proofs |
| Finance or Custody Operator | Reconcile funds, manage approved disbursements, record revenue allocation, and export financial evidence |
| Platform Administrator | Manage users, roles, methodologies, integration health, anchor retries, disputes, revocation, and audit logs |
| Public Verifier | Scan a proof QR or enter a proof ID to inspect the permitted public evidence and independently validate its IOTA commitment |

Role checks apply in the backend and, where applicable, through IOTA role capabilities. Hiding a button is not authorization.

## 5. Wireframe-driven experience

### 5.1 C1 Project Dashboard

The project dashboard retains the wireframe's structure and information hierarchy:

- Project name and number of sites.
- Interactive map with GPS plots and NDVI overlay.
- Area restored, seedling survival, estimated CO2e over the selected methodology period, and credits issued.
- Clear labels distinguishing measured, estimated, verified, pending, and rejected values.
- Project wallet showing tourism contributions, carbon-credit sales, and community share.
- Alert cards such as survival below threshold, missing evidence, failed anchors, overdue monitoring, and registry mismatches.
- Primary actions for Field Visit and Reports.
- Mobile navigation: Home, Monitor, Credits, Wallet, Profile.

The 60 percent community share in the wireframe is a project-configurable rule, not a universal hardcoded constant. Every displayed split must show its effective rule and calculated amounts.

### 5.2 C2 MRV Field Capture

The field workflow retains:

- Plot identity and GPS accuracy.
- Alive and planted counts.
- Average height and species.
- Geo-tagged photos.
- Voice observations, stored as audio with optional transcription.
- Offline queue count and synchronization status.
- Signed monitor attestation using the monitor's IOTA-linked identity.
- Content commitment created from the canonical record and evidence manifest.

Additional required behavior:

- Capture client ID, device timestamp, server receipt time, methodology version, activity ID, and schema version.
- Preserve original media and compute per-file hashes before upload.
- Support retry, cancellation before submission, duplicate detection, and conflict resolution.
- Do not label a queued record as anchored or verified.
- Show separate states: On device, Uploading, Submitted, Awaiting verification, Anchoring, Anchored, Verification failed, and Sync conflict.

### 5.3 C3 Verification Pipeline

The pipeline retains the four wireframe stages:

1. Field data complete.
2. Satellite corroboration.
3. Third-party audit.
4. Registry issuance followed by authorized chain representation.

It also retains document tracking for the Project Design Document, monitoring reports, and co-benefit attestations.

Each stage must expose owner, status, timestamps, evidence references, exceptions, and next action. Approval rules are methodology-specific. A project cannot skip a required stage through an administrator edit.

Three different provenance events are supported:

- **Evidence submitted:** optional early anchor proving that a particular evidence bundle existed at a particular time. This does not prove completion or achievement.
- **Achievement completed:** optional project-manager-attested `achieved_unverified` anchor. It proves the existence and integrity of the completion claim, not its independent validity.
- **Achievement verified:** anchor after required independent verification, representing the approved conservation outcome.

The user-facing timeline and proof must distinguish these events. An unverified completion proof can be generated, but it cannot display a verified-achievement badge or supply registry-issued credits.

### 5.4 C4 Carbon Marketplace

The marketplace retains:

- Project, vintage, registry badge, available tonnes, co-benefit indicators, and price per tCO2e.
- Buyer purchase flow.
- Retirement certificate with registry serial reference and chain reference.
- Revenue split display.

Rules:

- Only registry-issued inventory can be sold as issued carbon credits.
- Forward or pre-issuance inventory must have a separate product type, contract, label, and fulfillment flow. It must never be presented as issued credits.
- Inventory availability comes from reconciled registry and internal custody state, not chain supply alone.
- Retirement requires confirmed registry retirement and a corresponding BahariMRV provenance event. A partial failure creates a reconciliation case, not a false completed certificate.
- Payment, custody, fees, currency, tax, refunds, and settlement are recorded explicitly. No float arithmetic is used for money.

### 5.5 Proof Center

The Proof Center is an additional surface required by the product request:

- Generate an achievement proof on demand.
- Select PDF, JSON, or ZIP evidence package according to permission.
- Display a chronological Chain of Custody and achievement timeline.
- Show QR code, proof ID, IOTA testnet transaction reference, issuer DID, verifier attestation, evidence manifest, and registry references where applicable.
- Show pending, anchored, superseded, revoked, disputed, or failed status honestly.
- Allow public verification without exposing restricted evidence.

### 5.6 Android visual parity redesign, approved 2026-11-01

The approved target is the native Android app only. The supplied C1-C4 wireframes are the visual authority. This redesign changes presentation and interaction composition, not the existing API, Room, offline-sync, authentication, attestation, proof, purchase, or retirement contracts. Web and administrator surfaces are outside this scope.

#### Shared Android composition

- Use compact white work surfaces, fine blue-gray borders, 12 dp cards, 8 dp controls, emerald primary actions, and navy financial proof surfaces.
- Preserve Material 3 navigation, system Back, window insets, dark theme, font scaling, and minimum 48 dp touch targets.
- Keep Home, Monitor, Credits, Wallet, and Profile as the five top-level destinations.
- Put secondary identifiers, integration metadata, advanced actions, and long history in expandable sections or Material bottom sheets. The reference hierarchy must remain visible before those details.
- Keep loading, empty, error, offline, sandbox, pending, failed, unverified, and unavailable states explicit. Decorative completion treatment never substitutes for a server state.

#### C1 Android mapping

- The first dashboard sequence is project header, map and NDVI status, 2 by 2 metric grid, wallet summary, alerts, then Field visit and Reports.
- Project selection, manual project ID loading, refresh, project summary counts, additional metrics, methodology, integration detail, and full plot detail remain available through progressive disclosure.
- Every existing plot remains selectable. NDVI and metric values retain their returned status labels.
- Replace the hardcoded wallet preview with values from the existing wallet API. Show loading, error, or unavailable states when that API does not return a value. Do not calculate financial balances on the client.

#### C2 Android mapping

- The capture sequence is Field monitoring context and queue count, GPS and plot chips, survival fields, photo capture, voice observation, local queue status, attestation, then Save monitoring record.
- Keep project, activity, plot, methodology version, coordinates, and accuracy editable in a compact Plot and GPS sheet opened from the context chips.
- Preserve device GPS, manual unverified coordinates, photo and audio picking, voice recording, evidence removal, local hashing, canonical commitments, attestation, save, manual sync, and submission history.
- Keep the full sync-state vocabulary and per-record errors. Queued or locally saved records must not appear anchored or verified.

#### C3 Android mapping

- Use the real verification route for the four-stage connected timeline and required-document list. Remove the static pipeline and document mock from Credits.
- Preserve queue selection, manual Activity ID lookup, stage owner/time/evidence/exception/next action, provenance events, activity actions, proof generation and downloads, and verifier decisions.
- Put activity actions, provenance events, proof center, and verifier decision controls below the reference timeline in clearly named expandable sections.
- Manager completion, independent verification, registry issuance, and chain representation remain distinct events.

#### C4 Android mapping

- The marketplace sequence is buyer header, compact listing cards, selected-listing purchase action, then retirement and certificate content.
- Preserve product-type separation, registry and integration status, tonnes, co-benefits, decimal prices, batch identifiers, password step-up, purchase receipts, retirement input, certificate references, and explorer metadata.
- Open purchase and retirement forms in Material sheets so the marketplace remains scannable. Issued credits and pre-issuance forwards keep distinct labels and actions.
- Remove the hardcoded community/project/platform allocation. Show an allocation only when the server returns it; otherwise state that allocation detail is unavailable.
- Retirement wording must reflect only the registry and chain results returned by the server. Partial failure remains a reconciliation state, never a completed certificate.

#### Verification criteria

- Every action available before the redesign remains reachable, except fabricated displays that are replaced by truthful server-backed or unavailable states.
- C1-C4 match the supplied hierarchy at compact Android width without hiding primary actions behind scrolling-only ambiguity.
- Existing unit tests pass, the Android app compiles, and Home, Monitor, Verification, Credits, purchase, retirement, offline save, and sync receive smoke tests on an emulator where backend state permits.
- Visual verification covers loading, populated, empty, error, offline, and server-pending states. Unavailable live credentials or provider integrations are reported rather than simulated.

## 6. Core workflows

### 6.1 Mandatory identity provisioning

1. User authenticates through an approved sign-up method.
2. Backend creates or resumes a unique provisioning transaction keyed by the authenticated user ID.
3. A wallet is generated through the approved custody model.
4. Backend creates a resolvable IOTA Identity DID document on IOTA testnet and binds it to the internal user ID.
5. Backend issues the appropriate role capability after role approval.
6. Backend stores the DID, address, key reference, provisioning status, and transaction references.
7. The account becomes active only when wallet, DID document, and required role provisioning succeed.
8. Failures keep the account in `identity_pending` or `identity_failed`, with safe retry and support details. No alternative sign-up path may bypass provisioning.

Provisioning is idempotent. Repeated registration, profile loading, token refresh, or login must not create duplicate identities.

### 6.2 Conservation activity and provenance

1. Project Manager defines the project, sites, plots, methodology, baseline, monitoring plan, and required evidence.
2. Field Monitor captures an activity or monitoring observation offline and signs it.
3. Backend validates identity, role, plot assignment, schema, timestamps, idempotency key, media hashes, and evidence manifest.
4. Evidence is stored in restricted off-chain storage. Public IPFS pinning is limited to explicitly approved non-sensitive artifacts.
5. If the Project Manager marks the activity complete, the backend may anchor their signed `achieved_unverified` claim. Chain readback must match the claim commitment before the anchor is shown as confirmed.
6. Satellite service attaches source imagery metadata, acquisition time, tile or scene ID, processing version, and NDVI results.
7. Independent Verifier reviews the complete package and requests corrections, rejects it, or signs an approval.
8. Backend canonicalizes the approved achievement and creates a separate SHA-256 commitment linked to any earlier completion anchor.
9. Backend anchors the verified achievement on IOTA testnet through a Move contract or notarization object.
10. The transaction is confirmed and independently read back before status becomes `anchored`.
11. The verified achievement becomes eligible for official achievement proofs, registry processing, or credit workflows.

If IOTA is unavailable, the applicable completion or verification record remains `proof_pending`. It is not displayed as chain verified.

### 6.3 On-demand proof provisioning

1. An authorized user requests a proof for an achievement, project report, credit batch, or retirement.
2. Backend reloads the current record, evidence manifest, attestations, provenance events, and applicable registry state.
3. Verification service reads IOTA testnet and checks the anchored commitment, issuer, status, and supersession chain.
4. Proof service creates a versioned JSON manifest and a PDF rendering from the same source data.
5. ZIP export includes the PDF, JSON, public evidence, verification instructions, and any permitted Chain of Custody records.
6. A QR code resolves to a permanent public verification URL containing a non-sensitive proof ID.
7. Public verification recomputes supported hashes, queries IOTA, validates DID and attestation status, and reports each check independently.
8. Later corrections produce a new proof version linked to the old one. Old URLs remain available and display their superseded status.

### 6.4 Credit custody and retirement

1. Registry issuance is imported or received through an adapter with project, vintage, quantity, and serial range.
2. Reconciliation confirms that internal inventory matches the registry before chain representation or listing.
3. Purchase creates an auditable custody event after confirmed payment or approved settlement.
4. Revenue allocation uses the project's effective split rule and Decimal arithmetic.
5. Retirement locks the specified serials, submits registry retirement, records the registry certificate, and anchors the retirement event.
6. Reconciliation detects quantity, serial, ownership, or status mismatches and blocks further sale of affected inventory.

## 7. Technical architecture

### 7.1 Applications

- **Field and user app:** Android, Kotlin, Jetpack Compose, Hilt, Room, DataStore, WorkManager, and EcoTrack-style MVVM/Clean Architecture.
- **Backend:** Go, Gin, GORM, PostgreSQL, REST API, WebSocket status updates, background jobs, and adapter-based external integrations.
- **Web portal:** React and TypeScript for verifiers, buyers, finance operators, administrators, and public proof verification.
- **Chain:** IOTA Rebased testnet, Move contracts, IOTA Identity DID documents, sponsored transactions, and testnet explorer links.

### 7.2 Backend modules

- Auth and Identity Provisioning
- User, Role, and Key Management
- Project, Site, Plot, and Methodology
- Conservation Activity and Monitoring
- Evidence and Media
- Offline Synchronization
- Satellite Corroboration
- Verification and Attestation
- IOTA Provenance and Anchor Queue
- Proof and Document Provisioning
- Registry Integration and Carbon Inventory
- Marketplace, Custody, Retirement, and Revenue Allocation
- Notifications, Audit, Disputes, and Reconciliation

### 7.3 EcoTrack reuse and required changes

| EcoTrack capability | BahariMRV treatment |
|---|---|
| Android module boundaries, Compose, Room, WorkManager | Reuse the framework and local-first patterns, with non-destructive migrations and conservation entities |
| BIP-39, Ed25519, address derivation, personal-message signing | Reuse after deterministic-vector, signature, recovery, and secure-storage tests |
| EncryptedSharedPreferences | Reuse for device-local secrets |
| Sponsored build, user sign, backend execute flow | Reuse for user-authorized provenance transactions |
| Go auth, RBAC, PostgreSQL, audit, idempotency | Reuse with authenticated registration binding and conservation roles |
| Move role capabilities and immutable evidence objects | Generalize for project managers, monitors, verifiers, achievements, attestations, and retirements |
| Material Passport timeline | Replace with Conservation Achievement Proof and evidence lineage |
| Chain of Custody PDF and ZIP generation | Reuse the delivery pattern, not the plastic calculations or unverified claims |
| Public passport QR verification | Reuse the URL pattern, but perform live chain and attestation checks |
| Optional or best-effort identity creation | Replace with mandatory, idempotent provisioning for every account |
| String-only DID attributes | Replace with resolvable IOTA Identity DID documents and lifecycle management |
| UID-derived recovery encryption or plaintext fallback | Prohibit |
| Client-supplied verified status or transaction hash | Prohibit |
| Simulated blockchain checks or hardcoded proof statements | Prohibit |

## 8. IOTA design

### 8.1 Identity

Each user record contains:

- Internal user ID.
- IOTA address.
- DID identifier and DID document reference.
- Key custody reference, never plaintext key material.
- Provisioning status and transaction references.
- Role capabilities.
- Created, rotated, revoked, and recovered timestamps.

DID rotation and recovery create auditable identity events. Old attestations remain resolvable to their historical issuer keys.

### 8.2 Achievement anchor

The public commitment is versioned and includes only approved fields:

- Achievement ID and project ID commitment.
- Claim type: `achieved_unverified` or `achievement_verified`.
- Activity type and methodology version hash.
- Reporting period.
- Measurement manifest hash, marked as claimed or independently approved.
- Evidence bundle root or Merkle root.
- Satellite corroboration hash.
- Verifier attestation hash and issuer DID.
- Previous achievement or correction anchor reference, where applicable.
- Approved timestamp.

Private coordinates, names, phone numbers, raw media, payment details, and restricted documents are excluded.

### 8.3 Anchoring and retries

- Anchor writes are idempotent by achievement ID and commitment hash.
- Queue states are `pending`, `submitted`, `confirmed`, `failed`, and `disabled`.
- Confirmation requires a chain readback and matching commitment.
- Retry preserves the original commitment and records every attempt.
- Explorer URLs are derived from the configured testnet, never hardcoded to mainnet.
- Batch anchors include per-item Merkle proofs so each achievement can be verified independently.

## 9. Proof package contract

### 9.1 Human-readable PDF

- BahariMRV proof ID, version, and proof type: completion claim or independently verified achievement.
- Project and achievement summary.
- Activity type, reporting period, units, and claimed or approved outcomes with an unambiguous verification status.
- Methodology and verification statement.
- Satellite corroboration summary and source references.
- Verifier organization, DID, decision, and timestamp.
- Chronological provenance and custody events.
- Evidence manifest with hashes and access classification.
- IOTA testnet transaction or block reference and explorer link.
- Registry issuance or retirement details when applicable.
- Amendments, disputes, revocation, or supersession notices.
- QR code and instructions for independent verification.

### 9.2 Machine-readable JSON

The JSON manifest is canonicalizable and contains schema version, proof ID, subject, claims, measurements, evidence hashes, attestations, provenance events, registry references, chain network, transaction references, proof status, and generation timestamp.

PDF and JSON are generated from the same snapshot. A document must never claim `verified`, `anchored`, `registry issued`, `retired`, `GPS verified`, or `signed` unless the corresponding check passed for that snapshot.

### 9.3 Public verification response

Return separate results for:

- Proof schema and signature.
- Evidence manifest integrity.
- IOTA commitment match.
- Issuer DID validity.
- Verifier attestation and revocation status.
- Registry reference status, when an adapter is available.
- Supersession or dispute status.

Overall status is `valid` only when all required checks pass. Unsupported external checks are labeled `unavailable`, not successful.

## 10. Data model

Primary entities:

- User, IdentityProvision, RoleAssignment, KeyReference
- ConservationProject, Site, Plot, Methodology, MonitoringPlan
- ConservationActivity, MonitoringRecord, SpeciesObservation, SurvivalMeasurement
- EvidenceBundle, EvidenceObject, SatelliteObservation
- VerificationRequest, VerifierAttestation, Correction, Dispute
- ProvenanceEvent, AnchorAttempt, CustodyEvent
- ProofDocument, ProofVerification
- RegistryAccount, CreditIssuance, CreditBatch, CreditSerialRange
- MarketplaceListing, Purchase, Retirement, RevenueRule, RevenueAllocation
- WalletLedgerEntry, PaymentSettlement, ReconciliationCase
- SyncAction, IdempotencyKey, AuditEvent

All monetary values use fixed-precision decimal types. Measurements carry explicit units, methodology versions, and uncertainty or estimation labels where applicable.

## 11. API outline

### Identity

- `POST /api/v1/auth/register`
- `POST /api/v1/identity/provision/retry`
- `GET /api/v1/users/me/identity`
- `POST /api/v1/users/me/identity/recover`
- `POST /api/v1/users/me/identity/rotate`

### Projects and MRV

- `POST /api/v1/projects`
- `GET /api/v1/projects/:id/dashboard`
- `POST /api/v1/projects/:id/plots`
- `POST /api/v1/projects/:id/activities`
- `POST /api/v1/activities/:id/monitoring-records`
- `POST /api/v1/monitoring-records/:id/evidence`
- `POST /api/v1/sync/batch`
- `GET /api/v1/sync/delta`

### Verification and provenance

- `POST /api/v1/activities/:id/submit-verification`
- `POST /api/v1/verification-requests/:id/decision`
- `GET /api/v1/activities/:id/provenance`
- `POST /api/v1/activities/:id/anchor/retry`
- `GET /api/v1/anchors/:id/status`

### Proofs

- `POST /api/v1/achievements/:id/proofs`
- `GET /api/v1/proofs/:id/download?format=pdf|json|zip`
- `GET /api/v1/public/proofs/:id`
- `POST /api/v1/public/proofs/:id/verify`

### Registry, marketplace, and wallet

- `GET /api/v1/marketplace/listings`
- `POST /api/v1/marketplace/purchases`
- `POST /api/v1/credit-batches/:id/retire`
- `GET /api/v1/projects/:id/wallet`
- `GET /api/v1/projects/:id/reconciliation`

Every mutating endpoint requires authorization, validation, audit logging, and idempotency where retries are possible.

## 12. Offline and conflict rules

- Local writes use UUID client IDs and are idempotent on the server.
- Media upload is resumable and checked by hash.
- WorkManager retries only when connectivity and server conditions permit.
- Room schema upgrades preserve queued records and evidence.
- A monitor can edit a local draft until first successful submission.
- Submitted corrections create new versions rather than overwriting synchronized records.
- Server rejects stale decisions on superseded evidence and returns the current version.
- Duplicate client IDs return the original result.
- Failed actions remain retryable and are not permanently marked as successful duplicates.

## 13. Security and privacy

- Bind registration and provisioning to a verified authentication token.
- Use KMS or HSM-backed envelope encryption for managed keys. No UID-derived encryption key, plaintext fallback, or recovery blob in routine profile responses.
- Keep private keys and mnemonics out of logs, analytics, proof documents, and public URLs.
- Require step-up authentication and rate limiting for recovery, rotation, purchase, retirement, and verifier decisions.
- Verify provenance signatures against the authenticated user's registered key.
- Scan uploaded files, enforce media limits, and strip unnecessary EXIF data from public derivatives.
- Use signed, expiring URLs for restricted evidence.
- Apply project-level access control to evidence and proof exports.
- Minimize precise locations and beneficiary data in public outputs.
- Maintain immutable audit events for identity, evidence, verification, proof, custody, and administrative actions.

## 14. External integrations

Adapter interfaces are required for:

- IOTA testnet RPC, Identity, faucet or gas sponsorship, and explorer.
- Satellite imagery and NDVI processing.
- Evidence storage and approved IPFS pinning.
- Carbon registry issuance, serial reconciliation, and retirement.
- Payment and settlement.
- Regulated fund custody where real funds are handled.
- Notifications.

Integration configuration must distinguish sandbox, test, unavailable, and live states. No integration may silently fall back to fabricated data.

## 15. Acceptance criteria

### Identity

- Given any supported registration method, when registration completes, then exactly one active IOTA testnet identity is bound to that user.
- Given an IOTA outage, when a user registers, then the account remains explicitly pending and retrying does not create duplicates.
- Given a recovered account, when recovery completes, then the restored key matches the registered identity or a valid rotation record.

### MRV and provenance

- Given no connectivity, when a monitor saves a complete record, then it remains locally available and synchronizes once without duplication.
- Given incomplete required evidence, when verification is requested, then the system identifies the gap and does not anchor an achievement.
- Given a rejected activity, when proof is requested, then no verified-achievement proof is issued.
- Given a manager-attested completion, when IOTA confirms its anchor, then the proof remains labeled `achieved_unverified` and cannot unlock registry-issued credit workflows.
- Given an independently approved achievement, when IOTA confirms the separate verified anchor, then independent readback returns the same commitment and links any earlier completion claim.
- Given an IOTA failure, when the user views the completion or achievement, then it shows `proof_pending`, not `verified on chain`.

### Proof service

- Given a valid achievement, when a user requests PDF and JSON, then both contain the same claims and references.
- Given a public proof QR, when a verifier scans it, then the service performs real chain and attestation checks and exposes only permitted evidence.
- Given an amended achievement, when an old proof is opened, then it links to the superseding version and does not appear current.
- Given missing registry connectivity, when proof verification runs, then registry status is `unavailable` and is never reported as confirmed.

### Marketplace

- Given no registry issuance, when a project completes monitoring, then no issued-credit inventory is created.
- Given a partial registry or chain failure during retirement, when the user views the certificate, then it shows reconciliation pending rather than completed retirement.
- Given a sale or retirement, when reconciliation runs, then serial quantity and ownership changes balance exactly and cannot be reused.

## 16. Testing and release gates

- Unit tests for wallet derivation, signatures, recovery, canonicalization, commitments, measurements, revenue allocation, and state machines.
- Move tests for every role capability, achievement, attestation, correction, credit representation, and retirement entry point.
- Go integration tests against PostgreSQL and an IOTA testnet environment.
- Android tests for offline capture, process death, migration, media retry, duplicate sync, and conflict handling.
- Contract tests for registry, satellite, storage, and payment adapters.
- End-to-end tests for every sign-up method, C1-C4, proof generation, QR verification, amendment, dispute, purchase, and retirement.
- Security tests for authorization, recovery, key handling, evidence access, replay, tampering, and rate limits.
- No release with simulated chain verification, hardcoded evidence claims, plaintext key fallback, or a backend-to-Move method mismatch.

## 17. Delivery sequence

### Phase 0: Foundation and chain spike

- Confirm the initial registry, satellite provider, methodology, and financial-custody boundary.
- Prove one end-to-end IOTA testnet DID provisioning flow and both completion and verified-achievement anchor flows.
- Align backend calls and deployed Move package through executable tests.

### Phase 1: Identity, projects, and C2 field capture

- Hardened mandatory identity provisioning.
- Project, site, plot, methodology, and role management.
- Offline MRV capture, evidence upload, synchronization, and basic C1 dashboard.

### Phase 2: C3 verification and proof provisioning

- Satellite corroboration, independent review, attestations, achievement anchoring, corrections, disputes, and Proof Center.

### Phase 3: C4 registry-backed market and wallet

- Registry adapter, inventory reconciliation, purchase, custody, revenue allocation, retirement, and financial audit.

### Phase 4: Pilot hardening

- Security review, field offline testing, verifier and buyer acceptance, Swahili content, operational runbooks, backup and recovery, and pilot release.

## 18. Decisions and remaining clarifications

### 18.1 Decisions recorded

1. **Application surfaces:** Android plus a responsive verifier, buyer, administrator, and public-proof web portal.
2. **Key custody:** Managed custodial wallets with KMS-backed recovery and a future export or rotation path.
3. **Achievement provenance:** Support both a project-manager-attested `achieved_unverified` anchor and a later independently verified achievement anchor. They have separate meanings and proofs.
4. **Carbon market launch:** Implement C4 with registry and payment sandbox integrations, without live financial or registry claims.

### 18.2 Remaining clarifications

1. **Registry and satellite providers:** Which carbon registry and satellite/NDVI provider should be targeted first? If none is selected, implementation will begin with contract-tested sandbox adapters and no provider branding.
2. **Financial custody:** Should tourism contributions and carbon revenue be displayed through read-only imports from BahariLink, or should BahariMRV itself initiate disbursements and revenue allocation? The recommended first-release boundary is read-only imports plus sandbox allocation calculations, with regulated disbursement handled outside BahariMRV.

## 19. Bidirectional Jahazi and BahariMRV identity

BahariMRV and Jahazi are independent OIDC providers and relying parties under a versioned BahariLink identity contract. A shared `@baharilink/identity-sdk` provides discovery, authorization-code and PKCE helpers, token validation, claim types, and scope constants. Neither platform shares passwords, refresh tokens, legacy signing secrets, or unrestricted bearer tokens with the other.

### 19.1 Protocol requirements

- Authorization Code flow with `state`, `nonce`, and PKCE S256 only.
- Exact pre-registered HTTPS redirect URIs. Development HTTP redirects are limited to loopback addresses.
- Hashed, single-use authorization codes expiring within two minutes.
- RS256 or ES256 identity tokens with stable `kid`, explicit issuer, audience, expiry, nonce, and `token_use` claims.
- Public JWKS discovery and cached key rotation.
- Short-lived access tokens for explicitly exposed API scopes. ID tokens are never accepted as API bearer tokens.
- Sovereign local sessions after a completed federation flow. Local refresh-token rotation remains local to each platform.
- Immediate denial for disabled clients, revoked grants, revoked links, invalid scopes, replayed codes, and redirect mismatches.

### 19.2 Account linking and provisioning

- Identity is keyed by `(issuer, subject)`, never by matching email addresses or phone numbers.
- A known linked identity can sign in directly.
- First-time linking to an existing account requires an authenticated local session and a separate external authorization callback.
- A platform may create a new federated account for an unknown external subject only when its local schema and onboarding requirements support it. It must not attach that subject to an existing account through contact-field matching.
- Local roles are loaded from the local account after linking. External role claims are informational and never authorize local actions.
- Unlinking revokes the associated grants and denies future federated API access.

### 19.3 Initial scopes

- `openid`, `profile`, `email`, and `phone` for consented identity claims.
- `conservation:read` for approved project and public-impact reads.
- `conservation:financial:read` for explicitly exposed financial imports.

Financial scope is never included in default platform-key or login grants. Public proof verification does not require SSO.

### 19.4 BahariLink SDK and API

The shared SDK has platform-neutral TypeScript types and helpers, a server entry point for discovery/JWKS/token exchange, and a browser entry point for PKCE/state/nonce generation. It contains no private keys or client secrets. Both platform APIs expose only their own registered clients, discovery, token, userinfo, linking, and revocation endpoints. BahariLink financial imports use a dedicated federated guard or route rather than broadening existing local JWT middleware.

## 20. Implementation boundary

The first implementation uses the recommended defaults: Android plus web, managed key custody, dual completion and verified-achievement provenance, provider-neutral registry and satellite sandboxes, payment sandbox, and read-only BahariLink financial imports. Live provider credentials, production KMS keys, signing keys, registered client secrets, and deployed IOTA package IDs are environment inputs and are not committed to source.
