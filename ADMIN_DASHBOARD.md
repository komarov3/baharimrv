# BahariMRV Admin Dashboard Specification

**Status:** Draft for clarification, not approved for implementation
**Date:** 2026-10-31
**Scope:** Platform Administrator web experience and supporting backend read/write surfaces
**Primary references:** SPEC.md, PRODUCT.md, DESIGN.md, current Go/React codebase
**Implementation status:** Specification only. No admin dashboard, admin route, or admin API currently exists.

---

## 1. Purpose and positioning

The Admin Dashboard is the master control and data hub for BahariMRV. It is the single place where a platform_administrator can see the whole platform state, act on failures, govern users and configuration, and audit every consequential change.

It must satisfy the BahariMRV product principles:

1. Evidence precedes claims. Admin views never upgrade pending, estimated, sandbox, or unavailable data into verified-looking data.
2. Identity is mandatory. User activation, provisioning failure, retry, and revocation are first-class admin workflows.
3. Verification is explicit. Admin can see who verified what, under which methodology, and when.
4. The chain is checked. Admin anchor views report live chain state and readback results, not only database rows.
5. Honest degraded states. Sandbox, unavailable, failed, and retryable states remain visible and actionable.
6. Privacy by default. Admin sees the minimum necessary PII and every sensitive access is auditable.

The dashboard is a one-stop data hub: cross-domain search, linked drill-downs, consistent filters, export, and retry operations across users, projects, evidence, verification, provenance, carbon, finance, integrations, and audit data.

---

## 2. Current status analysis

### 2.1 What already exists and can be reused

| Area | Current state | Admin relevance |
|---|---|---|
| Roles | platform_administrator exists in models.RoleAdmin, but registration only permits manager, monitor, verifier, and buyer. No admin bootstrap route is visible. | Admin identity must be provisioned through environment/seed or a controlled bootstrap, not public registration. |
| Web shell | React/TypeScript app with sidebar, project switcher, auth context, API client, and useApiData. No /app/admin route. | Add an admin shell and role-gated navigation. |
| Backend routes | All conservation, verification, market, proof, identity, sync, and health routes are under /api/v1 and require a local session. No /api/v1/admin/* routes. | Add admin-only aggregate and governance routes. |
| Health | /healthz and /api/v1/health report chain, satellite, registry, and payment modes plus server time. | Reuse as the integration-mode source of truth. |
| Audit | AuditEvent is written by auditWrites for mutating requests. Reads are intentionally not audited. | Add admin read/export audit where sensitive. |
| Users | User, IdentityProvision, KeyReference, LocalSession, ExternalIdentity, FederatedGrant. | Full user/identity directory and governance. |
| Projects | ConservationProject, Site, Plot, Methodology, activities, monitoring records, evidence. | Cross-project operations, quality, and assignment views. |
| Verification | VerificationRequest, VerifierAttestation, Correction, Dispute. | Queue, SLA, attestation, dispute, and correction oversight. |
| Provenance | ProvenanceEvent, AnchorAttempt, anchor retry endpoint. | Anchor queue, retries, readback, and dispute handling. |
| Proofs | ProofDocument, generation, download, public proof, verification checks. | Proof registry, revocation/dispute, public trust monitoring. |
| Carbon/finance | CreditBatch, MarketplaceListing, Purchase, CustodyEvent, RevenueAllocation, Retirement, ReconciliationCase, FinancialImport. | Inventory, reconciliation, sales, retirements, revenue, imports. |
| Satellite | SatelliteObservation, SatellitePass, satellite service mode. | Corroboration coverage, cloud cover, adapter health. |
| Sync | SyncAction, sync batch/delta endpoints. | Offline queue health and conflict resolution. |
| Storage | Evidence store with 25 MB file limit and content hashing. | Storage usage, orphaned files, evidence access. |
| Design system | Inter, emerald/navy, 232px sidebar, flat surfaces, compact rows, honest state chips. | Admin console should extend, not replace, this language. |

### 2.2 Current gaps that the dashboard must close

1. No admin role bootstrap. There is no public registration for platform_administrator; seed data does not create one. The spec must define a controlled bootstrap.
2. No admin authorization middleware usage. Role checks exist in services, but no admin-only route group.
3. No cross-project aggregate endpoints. Managers see their own projects; admins need platform-wide aggregates.
4. No user lifecycle endpoints. Role changes, disable/enable, session revocation, and identity retry are not exposed as admin actions.
5. No configuration management. Modes and thresholds are environment/config driven; no runtime admin controls.
6. No job/queue visibility. Anchor, sync, satellite, registry, and proof generation failures are not centrally visible.
7. No audit read surface. AuditEvent exists but is not queryable by admins.
8. No global search. Data is fragmented by project and user.
9. No data export. Audit, financial, and operational exports are not defined.
10. No incident model. Failures are visible only where they happen.

### 2.3 Data that can be surfaced immediately without new ingestion

The dashboard can initially be built almost entirely from existing tables: users and identity state, projects, sites, plots, activities, monitoring records, evidence, satellite observations and passes, verification requests and attestations, corrections, disputes, provenance events, anchor attempts, proof documents, credit batches, listings, purchases, custody events, revenue allocations, retirements, reconciliation cases, financial imports, project documents, sync actions, and audit events.
---

## 3. Admin identity, access, and safety

### 3.1 Bootstrap

- The first platform administrator is created only by a controlled bootstrap process: environment-controlled seed command, or a one-time setup endpoint protected by a server-side setup secret and disabled after first use.
- Public /auth/register must never accept platform_administrator.
- Every admin account must still complete mandatory IOTA identity provisioning before activation.
- Demo seed may optionally create admin@demo.baharimrv.local only when SEED_DEMO_DATA=true and never in production.

### 3.2 Authorization

- All /api/v1/admin/* routes require: auth.RequireLocal, RequireRole(models.RoleAdmin), auditWrites for mutations, and step-up for sensitive or destructive mutations.
- Role checks are enforced server-side. Hiding an admin button is never authorization.
- Admin permissions should be split into capability groups even if one role initially owns them:
  - admin.users.read, admin.users.write
  - admin.identity.read, admin.identity.retry
  - admin.projects.read, admin.projects.write
  - admin.verification.read, admin.verification.override
  - admin.provenance.retry, admin.provenance.revoke
  - admin.proofs.read, admin.proofs.revoke
  - admin.finance.read, admin.finance.export
  - admin.integrations.read, admin.integrations.write
  - admin.audit.read, admin.audit.export
  - admin.config.read, admin.config.write
- Future delegation can use these capability flags without changing route shapes.

### 3.3 Step-up and sensitive actions

Sensitive actions require a fresh step-up token:

- Role change.
- User disable/enable.
- Session revocation.
- Identity recovery/rotation trigger.
- Anchor retry or disable.
- Proof revocation or dispute resolution.
- Registry reconciliation resolution.
- Retirement retry.
- Configuration change.
- Maintenance mode toggle.
- Bulk retry operations.
- Data export of PII or financial data.

Step-up should use the existing StepUp service and password re-entry. A future TOTP factor can be added without changing the action contract.

### 3.4 Audit

- Every admin mutation is audited with user, action, resource, request ID, status, IP, and timestamp.
- Sensitive admin reads (user detail, identity detail, evidence download, audit export, financial export) should create an explicit audit event even though ordinary reads are not audited.
- Audit records are append-only. Admins cannot delete or edit audit events.
- Audit views show the actor's role at the time if available.

### 3.5 Privacy

- Admin user directory shows PII only when needed and only to authorized admins.
- Key material, wrapped keys, nonces, and password hashes are never returned.
- Evidence downloads by admin are auditable and respect access classification.
- Public proof verification never exposes restricted evidence.
- Financial exports are scoped and audited.

---

## 4. Information architecture

### 4.1 Admin navigation

Admin users get an additional top-level console, separate from the conservation project shell:

/admin
  /overview
  /search
  /users
  /identities
  /projects
  /field-operations
  /evidence
  /verification
  /disputes
  /provenance
  /anchors
  /proofs
  /public-trust
  /registry
  /marketplace
  /retirements
  /finance
  /satellite
  /integrations
  /sync
  /audit
  /security
  /methodologies
  /configuration
  /alerts
  /jobs
  /exports
  /support

The admin console should use the same design tokens and component language as the rest of BahariMRV, but can be denser and more table-driven. It should not use decorative dashboard cards that obscure state.

### 4.2 Global context

- Admin views are platform-wide by default, not project-scoped.
- A global project filter can narrow any view.
- A global user filter can narrow any view.
- Every record links to related records: user -> identity -> projects -> activities -> verification -> provenance -> proof -> registry -> finance.
- Breadcrumbs and open in new tab support for investigation workflows.

### 4.3 Saved views

- Admins can save filter/sort/column configurations per page.
- Saved views are user-specific and auditable when shared.

---

## 5. Feature inventory

### 5.1 Control Tower (/admin/overview)

Purpose: One-screen platform health and work queue.

KPI tiles (all with status and honest labels):

- Users: total, active, pending identity, failed identity, disabled, by role.
- Projects: total, active, sites, plots, area restored.
- Activities: draft, monitoring, completion claimed, awaiting verification, verified, rejected, disputed.
- Verification queue: requested, in review, overdue, approved, rejected, corrections requested.
- Anchors: pending, submitted, confirmed, failed, disabled, retries today.
- Proofs: pending, anchored, superseded, revoked, disputed, failed.
- Registry: batches, issued, available, sold, retired, reconciled, unreconciled, sandbox.
- Marketplace: listings, purchases, failed settlements, reconciliation pending.
- Retirements: completed, pending, failed, reconciliation pending.
- Finance: imported amount by source, last import time, reconciliation cases.
- Satellite: passes, coverage, cloud cover, unavailable.
- Sync: submitted, conflict, failed, pending device actions (if tracked).
- Alerts: critical, warning, info, unresolved.

Widgets:

- System health strip: chain, satellite, registry, payment modes; DB status; storage status; last job heartbeat.
- Needs attention queue: prioritized list of failures and overdue work with deep links.
- Recent admin actions.
- Recent integration errors (deduplicated).
- Trend sparklines for key counts over 7/30 days.
- Top failing operations by count.
- Quick actions: retry failed anchors, open verification queue, open reconciliation cases, open identity failures.

Honest state rules:

- Sandbox inventory is labeled sandbox and cannot be counted as issued credits.
- Estimated CO2e is labeled estimated, not verified.
- Pending anchors are not counted as anchored.
- Unavailable satellite data is labeled unavailable, not zero.

### 5.2 Global search (/admin/search)

Purpose: One entry point for investigation.

Searchable entities:

- Users by name, email, ID, DID, address.
- Projects by name, ID, community group.
- Sites/plots by code, ID, coordinates.
- Activities by ID, project, status.
- Verification requests by ID, verifier, activity.
- Proofs by ID, achievement, project.
- Credit batches by serial, issuance ID, project.
- Purchases/retirements by ID, owner, batch.
- Audit events by request ID, user, action, resource.

Features:

- Type filters and saved searches.
- Result cards with status chips and deep links.
- Permission-aware snippets.
- Recent searches.
- Export search results.

### 5.3 Users and roles (/admin/users)

Views:

- User table with columns: name, email, role, status, identity state, created, last login, projects, actions.
- Filters: role, status, identity state, created date range, project, has linked platforms, has active sessions.
- Sort and pagination.

User detail (/admin/users/:id):

- Profile: name, email, phone, role, status, created/updated.
- Identity: state, DID, address, network, attempts, last error, activated at, last provisioned at, provision tx ref.
- Key custody metadata: provider, external ref, created/rotated/revoked timestamps (no key material).
- Linked platforms and federated grants.
- Sessions: active/expired/revoked, IP, created, expiry.
- Role capabilities.
- Project memberships and managed projects.
- Recent audit events.
- Related identity actions.

Actions (step-up where marked):

- Create admin/finance user (admin only).
- Change role (step-up).
- Disable/enable user (step-up).
- Force identity retry.
- Trigger recovery/rotation workflow (step-up).
- Revoke all sessions (step-up).
- Unlink platform identity (step-up).
- Reset password via secure flow (step-up; no plaintext handling).
- Annotate user (internal note, auditable).

Safety:

- Cannot disable the last active platform administrator.
- Cannot remove own admin role without another admin confirming.
- Role changes are additive in audit history.

### 5.4 Identity operations (/admin/identities)

Purpose: Monitor mandatory IOTA identity provisioning.

Views:

- Identity table: user, role, state, network, attempts, last error, created, activated.
- Filters: state, network, role, error type, attempt count.
- Summary: counts by state, failure reasons, retry success rate.
- Retry queue: pending/failed identities with age.

Actions:

- Bulk retry failed identities (step-up, filtered).
- Inspect provision transaction reference and explorer link.
- Mark identity as requiring manual review (does not fabricate success).
- Export identity status report.

Honest state rules:

- pending and failed are never shown as active.
- A retry does not imply success until the chain readback confirms it.

### 5.5 Projects and sites (/admin/projects)

Views:

- Project table: name, manager, community group, conservation type, methodology, sites, plots, area, status, created, last MRV.
- Filters: manager, conservation type, methodology, status, created date, has failures.
- Project detail admin view: sites, plots, assigned monitors, documents, wallet summary, alerts, activities.
- Site/plot table with coordinates, area, assigned monitor, activity type, status, chain state, NDVI.

Actions:

- View project as admin (read-only unless explicitly granted).
- Reassign project manager (step-up).
- Reassign monitor to plot.
- Archive/unarchive project (step-up).
- Trigger satellite refresh for project (if adapter available).
- Export project register.

Map view:

- Global plot map with filters and clustering.
- NDVI overlay status honest labels.
- Evidence and failure overlays.

### 5.6 Field operations and data quality (/admin/field-operations)

Views:

- Activity ledger across all projects.
- Monitoring record volume by day/project/monitor.
- Evidence gap list: missing required evidence, low GPS accuracy, missing photos/voice.
- Sync conflict list with payload summary.
- Duplicate submission detection.
- Offline queue age and device sync lag (if client telemetry exists).

Actions:

- Request correction/additional evidence (through verification workflow, not direct edit).
- Retry sync action (step-up).
- Mark conflict resolved with reason (step-up).
- Export data quality report.

Honest state rules:

- Admin cannot edit submitted monitoring values directly; corrections are additive.
- Conflicts remain visible until resolved.

### 5.7 Evidence browser (/admin/evidence)

Views:

- Evidence object table: ID, activity, project, type, filename, size, hash, access classification, created, storage status.
- Storage usage by project and type.
- Orphaned evidence (no linked record).
- Failed upload/hash mismatch list.

Actions:

- Download evidence (audited).
- Verify hash against stored file.
- Mark evidence as restricted/public classification review.
- Trigger cleanup of orphaned files (step-up, dry-run first).

Privacy:

- Restricted evidence requires explicit permission and audit.
- Evidence is never served as public without access classification.

### 5.8 Verification oversight (/admin/verification)

Views:

- Verification queue across all verifiers.
- Filters: verifier, project, status, decision, date, overdue.
- SLA metrics: time to assignment, time to decision, overdue count.
- Attestation list with verifier DID, organization, methodology, timestamp.
- Verification decision history.

Actions:

- Reassign verification request (step-up).
- Cancel/reopen request with reason (step-up).
- View evidence packet.
- Export verification report.

Safety:

- Admin cannot approve/reject on behalf of an independent verifier unless an explicit override capability and step-up are used; any override is marked as admin override in provenance.

### 5.9 Disputes and corrections (/admin/disputes)

Views:

- Dispute table: activity, reason, status, created, resolved at, linked corrections.
- Correction timeline.
- Disputed proofs and anchors.

Actions:

- Assign dispute reviewer.
- Resolve dispute with documented reason (step-up).
- Trigger superseding proof generation.
- Export dispute report.

### 5.10 Provenance and anchors (/admin/provenance, /admin/anchors)

Views:

- Provenance event feed: event type, activity, claim type, actor, timestamp, chain state.
- Anchor queue by state.
- Anchor attempt detail: commitment hash, network, tx ref, explorer URL, submitted/confirmed time, readback result, error.
- Failed anchor list with retry eligibility.
- Chain mode and network status.

Actions:

- Retry single anchor (step-up).
- Bulk retry failed anchors by filter (step-up).
- Disable/enable anchoring for a specific failure class (step-up, with reason).
- Open explorer link.
- Export anchor audit.

Honest state rules:

- confirmed requires chain readback and matching commitment.
- disabled is not success.
- Sandbox anchors are not public ledger facts.

### 5.11 Proofs and public trust (/admin/proofs, /admin/public-trust)

Views:

- Proof document table: ID, project, activity, type, status, version, generated at, issuer DID, verifier, chain ref, manifest hash.
- Filters: status, type, project, date, proof ID.
- Supersession/dispute/revocation chains.
- Public proof verification logs (if instrumented): proof ID, checked at, overall status, failed checks, requester IP hash.
- Proof generation failures.

Actions:

- Generate proof (usually manager/verifier action; admin can trigger for support with audit).
- Revoke proof with reason (step-up).
- Mark proof disputed.
- Trigger superseding proof.
- Export proof registry.

Honest state rules:

- Proof status comes from actual generation/anchoring.
- verified_achievement proof requires independent verification milestone.

### 5.12 Registry and carbon inventory (/admin/registry)

Views:

- Adapter status and mode.
- Credit batch table: project, issuance ID, registry name, vintage, serial range, quantity, available, status, adapter mode, reconciled at.
- Inventory summary by project and status.
- Reconciliation cases.
- Sandbox inventory clearly labeled.

Actions:

- Trigger reconciliation (step-up).
- Resolve reconciliation case with reason (step-up).
- Block/unblock inventory (step-up).
- Export registry inventory.

### 5.13 Marketplace and custody (/admin/marketplace)

Views:

- Listings, purchases, custody events.
- Settlement status and failures.
- Buyer activity summary (aggregated, privacy-aware).
- Revenue allocation by purchase/project/recipient.

Actions:

- Retry failed settlement (step-up, if adapter supports).
- Freeze listing (step-up).
- Export marketplace audit.

### 5.14 Retirements (/admin/retirements)

Views:

- Retirement table: batch, owner, quantity, status, registry certificate, tx ref, object ID, created, completed, last error.
- Filters: status, project, date.
- Certificate availability.

Actions:

- Retry retirement (step-up).
- Inspect chain/registry references.
- Export retirement certificates list.

### 5.15 Finance and wallet (/admin/finance)

Views:

- Financial import table: source, external ID, project, amount, currency, occurred at, payload hash.
- Import health: last import, failures, duplicates.
- Project wallet summary across all projects.
- Revenue allocation summary.
- Reconciliation cases affecting finance.
- Export financial evidence.

Actions:

- Trigger import sync (step-up, if adapter supports).
- Export financial report (audited).

Honest state rules:

- Financial imports are read-only from BahariLink unless a future decision changes the boundary.
- Allocation calculations are labeled as sandbox if payment/registry adapters are sandbox.

### 5.16 Satellite and environmental data (/admin/satellite)

Views:

- Satellite pass table: project, activity, mission, pass type, window, acquired at, scene, orbit, tile, cloud cover, coverage, status detail, adapter mode, result hash.
- Coverage summary by project.
- Cloud cover distribution.
- Missing/unavailable corroboration list.

Actions:

- Trigger satellite fetch for project/activity (step-up).
- Export satellite coverage report.

### 5.17 Integrations and system health (/admin/integrations)

Views:

- Adapter cards: chain, satellite, registry, payment, storage, KMS, database, email (if added).
- Mode, configured status, last success, last failure, error sample, latency.
- Configuration summary (non-secret).
- Rate limit configuration.
- OIDC clients and grants.

Actions:

- Test adapter connection (step-up or read-only health check).
- Rotate/refresh credentials (through secure ops, not in-browser secret entry if avoidable).
- Enable/disable integration (step-up).
- Maintenance mode toggle (step-up).

Honest state rules:

- A sandbox adapter is never labeled live.
- Unconfigured live adapter is unavailable, not sandbox.

### 5.18 Sync and offline queue (/admin/sync)

Views:

- Sync action table: user, client ID, action type, status, request hash, resource ID, error, created/updated.
- Counts by status.
- Conflict detail with payload.
- Retry eligibility.

Actions:

- Retry failed sync action (step-up).
- Resolve conflict with documented reason (step-up).
- Export sync health.

### 5.19 Audit and security (/admin/audit, /admin/security)

Audit views:

- Audit event table: time, user, role, action, resource, status, request ID, IP.
- Filters: date, user, role, action, resource path, status code.
- Export CSV/JSON (audited).

Security views:

- Failed login attempts.
- Step-up failures.
- Permission denials.
- Rate limit hits.
- Session anomalies (multiple IPs, expired/revoked).
- Admin action summary.

Actions:

- Export security report.
- Revoke suspicious sessions (step-up).
- Lock user (step-up).

### 5.20 Methodologies and configuration (/admin/methodologies, /admin/configuration)

Methodologies:

- Methodology table: ID, version, name, effective period, parameters, created.
- Create new methodology version.
- Deprecate methodology (step-up).
- View usage by projects.

Configuration:

- Feature flags.
- Alert thresholds (survival below threshold, overdue monitoring days, anchor retry limits).
- Notification templates.
- Evidence size limit (read-only unless deployment config).
- Session TTL (read-only unless deployment config).
- Localization status.

Actions:

- Update feature flags (step-up).
- Update thresholds (step-up).
- Export configuration.

### 5.21 Alerts and incidents (/admin/alerts, /admin/jobs)

Alerts:

- Alert table across projects: severity, title, detail, action, status, created, resolved.
- Filters: severity, project, type, resolved.
- Bulk acknowledge/resolve (step-up).

Jobs:

- Background job runs: type, status, started, finished, duration, error, retry count.
- Failed job list.
- Manual trigger (step-up).

### 5.22 Support and diagnostics (/admin/support, /admin/exports)

Support:

- User lookup and impersonation request workflow (optional, high-risk).
- Diagnostic bundle generation.
- Contact/issue notes.

Exports:

- Predefined exports: users, projects, activities, verification, anchors, proofs, registry, finance, audit, security.
- Custom filtered export.
- Export history and download links with expiry.
- All exports audited.

---

## 6. Data model additions

The dashboard can start with existing tables, but the following additions are recommended for a powerful, maintainable admin experience:

### 6.1 AdminCapability or role capability flags

- Supports delegated admin without creating new roles.
- Capability strings as listed in section 3.2.

### 6.2 SystemConfig

- Key, value, type, description, updated by, updated at.
- Used for thresholds, feature flags, and non-secret runtime settings.
- Secret values remain in environment/KMS, never in this table.

### 6.3 Alert

- ID, scope (platform/project/user), severity, type, title, detail, action label/href, status, created, resolved, resolved by.
- Allows platform-wide alerting beyond project alerts.

### 6.4 Incident / JobRun

- Tracks background job execution and integration incidents.
- Fields: type, status, started/finished, duration, error, retry count, related resource, metadata.

### 6.5 AdminNote

- Internal notes attached to users/projects/activities/etc.
- Auditable, immutable-ish (append corrections).

### 6.6 ExportJob

- Tracks export requests, filters, status, file reference, expiry, requester.

### 6.7 SavedView

- User, page, name, filters/sort/columns, shared flag.

### 6.8 VerificationOverride (if admin override is allowed)

- Records admin override of verification with reason and step-up receipt.

### 6.9 PublicProofVerificationLog (optional)

- Tracks public verification requests for trust monitoring.
- Must avoid storing unnecessary PII; store hashed IP and minimal metadata.

---

## 7. API additions

All new endpoints are under /api/v1/admin/ and require local auth, admin role, audit, and step-up for mutations.

### 7.1 Overview and health

- GET /admin/overview
- GET /admin/health
- GET /admin/metrics?range=7d|30d
- GET /admin/alerts
- PATCH /admin/alerts/:id
- GET /admin/jobs
- POST /admin/jobs/:type/run

### 7.2 Users and identity

- GET /admin/users
- POST /admin/users
- GET /admin/users/:id
- PATCH /admin/users/:id
- POST /admin/users/:id/disable
- POST /admin/users/:id/enable
- POST /admin/users/:id/role
- POST /admin/users/:id/identity/retry
- POST /admin/users/:id/sessions/revoke
- GET /admin/users/:id/sessions
- GET /admin/users/:id/audit
- GET /admin/identities
- GET /admin/identities/summary

### 7.3 Projects and field operations

- GET /admin/projects
- GET /admin/projects/:id
- PATCH /admin/projects/:id
- POST /admin/projects/:id/archive
- GET /admin/sites
- GET /admin/plots
- GET /admin/activities
- GET /admin/monitoring-records
- GET /admin/evidence
- GET /admin/evidence/:id
- POST /admin/evidence/:id/verify-hash
- GET /admin/data-quality

### 7.4 Verification and disputes

- GET /admin/verification-requests
- GET /admin/verification-requests/:id
- POST /admin/verification-requests/:id/reassign
- POST /admin/verification-requests/:id/cancel
- GET /admin/attestations
- GET /admin/disputes
- POST /admin/disputes/:id/resolve
- GET /admin/corrections

### 7.5 Provenance and anchors

- GET /admin/provenance-events
- GET /admin/anchors
- GET /admin/anchors/:id
- POST /admin/anchors/:id/retry
- POST /admin/anchors/retry-batch
- POST /admin/anchors/:id/disable
- GET /admin/chain-status

### 7.6 Proofs and public trust

- GET /admin/proofs
- GET /admin/proofs/:id
- POST /admin/proofs/:id/revoke
- POST /admin/proofs/:id/dispute
- POST /admin/proofs/:id/regenerate
- GET /admin/public-verifications

### 7.7 Registry, market, finance, retirements

- GET /admin/registry/batches
- GET /admin/registry/reconciliation
- POST /admin/registry/reconcile
- POST /admin/registry/cases/:id/resolve
- GET /admin/marketplace/listings
- GET /admin/marketplace/purchases
- GET /admin/marketplace/custody-events
- GET /admin/retirements
- POST /admin/retirements/:id/retry
- GET /admin/finance/imports
- POST /admin/finance/imports/sync
- GET /admin/wallets
- GET /admin/revenue-allocations

### 7.8 Integrations, sync, audit, config

- GET /admin/integrations
- POST /admin/integrations/:name/test
- PATCH /admin/integrations/:name
- GET /admin/sync-actions
- POST /admin/sync-actions/:id/retry
- POST /admin/sync-actions/:id/resolve
- GET /admin/audit-events
- GET /admin/security-events
- GET /admin/methodologies
- POST /admin/methodologies
- PATCH /admin/methodologies/:id
- GET /admin/config
- PATCH /admin/config/:key
- GET /admin/saved-views
- POST /admin/saved-views

### 7.9 Search and export

- GET /admin/search?q=...&types=...
- POST /admin/exports
- GET /admin/exports
- GET /admin/exports/:id/download

All list endpoints should support filtering, sorting, pagination, and field selection where useful.

---

## 8. UI/UX specification

### 8.1 Visual language

- Extend the existing design system: Inter, emerald primary, navy for financial/proof surfaces, canvas background, white surfaces, fine borders.
- Admin pages may use denser tables and smaller padding, but remain calm and scannable.
- Status chips use existing honest-state vocabulary: measured, estimated, claimed, verified, pending, rejected, unavailable, anchored, failed, disputed, revoked, superseded, sandbox, achieved_unverified.
- Sandbox states use a distinct sandbox chip and never emerald success styling.

### 8.2 Layout

- Desktop: 232px sidebar, contextual top bar, 12-column work area.
- Admin sidebar groups: Overview, People, Operations, Trust, Carbon & Finance, System, Audit, Configuration.
- Mobile: collapse to navigation drawer; tables become card/row stacks with labels.
- Split views for investigation: list on left, detail on right on desktop; stacked on mobile.

### 8.3 Table behavior

- Server-side pagination, sorting, and filtering.
- Column chooser and saved views.
- CSV/JSON export.
- Row selection for bulk actions.
- Inline status chips and deep links.
- Empty states with explanation and next action.
- Error states with retry.
- Loading skeletons, not blank screens.

### 8.4 Detail pages

- Header with record ID, status chip, created/updated.
- Related records section with links.
- Timeline of events.
- Actions panel with confirmation and step-up.
- Audit trail excerpt.

### 8.5 Honesty in admin UI

- Every metric and table must label state.
- No red/green color-only meaning; include text.
- Sandbox inventory is never mixed with issued inventory without a visible mode label.
- Estimated and verified values are never visually equivalent.
- Failed jobs are not hidden behind no data.

---

## 9. Security, privacy, and compliance

### 9.1 Access control

- Admin console is not linked from public or ordinary user navigation.
- Direct URL access requires admin role; otherwise show not found or forbidden without leaking admin route existence.
- All admin APIs enforce role server-side.

### 9.2 Step-up

- Sensitive actions require recent step-up token.
- Step-up tokens are single-use, short-lived, and action-scoped.
- Failed step-up attempts are audited.

### 9.3 Data protection

- No secrets or key material in admin responses.
- Evidence downloads are access-classified and audited.
- PII exports are audited and time-limited.
- Financial data is read-only import unless a future decision changes the boundary.

### 9.4 Retention and deletion

- Admin cannot delete audit events.
- User deletion, if ever supported, must be a documented, auditable, and reversible workflow; prefer disable.
- Evidence deletion follows retention policy and is never silent.

---

## 10. Observability and operations

- Admin dashboard should consume structured health and metrics endpoints.
- Integration health includes last success/failure timestamps and error samples.
- Failed jobs and retry counts are visible.
- Alerting thresholds configurable.
- Admin actions should emit metrics for operational monitoring.

---

## 11. Testing and release gates

- Unit tests for admin authorization and capability checks.
- Integration tests for every admin endpoint against PostgreSQL.
- Tests for honest state labeling in admin responses.
- Security tests: non-admin cannot access admin routes; step-up required for sensitive actions; audit events written.
- E2E tests for critical admin workflows: user disable, identity retry, anchor retry, proof revocation, reconciliation resolution.
- No release of admin dashboard with simulated health, fabricated inventory, or un-audited destructive actions.
- Accessibility tests: keyboard navigation, screen reader labels, contrast.

---

## 12. Delivery phases

### Phase A: Foundation (read-only hub)

- Admin bootstrap.
- Admin role gate and shell.
- Overview, users, identities, projects, audit read.
- Global search v1.
- Integration health.

### Phase B: Operations and trust

- Field operations, evidence browser, verification oversight, disputes.
- Provenance and anchor queue with retry.
- Proofs and public trust monitoring.

### Phase C: Carbon, finance, and system

- Registry, marketplace, retirements, finance.
- Satellite admin.
- Sync admin.
- Methodologies and configuration.

### Phase D: Advanced

- Saved views, exports, jobs, incidents.
- Alert management.
- Delegated capabilities.
- Optional impersonation workflow.

---

## 13. Open decisions

1. Admin bootstrap method: Environment seed command vs one-time setup endpoint.
2. Admin override of verification: Whether admins may override verifier decisions and how that is represented in provenance.
3. Impersonation: Whether support impersonation is allowed; if so, it must be auditable and time-limited.
4. Configuration mutability: Which settings are runtime-configurable vs deployment-only.
5. Public verification logging: Whether to log public proof verification requests and retention period.
6. Delegated admin: Whether to implement capability flags in first release or keep a single admin role.
7. Data retention: Retention periods for audit, sync, and public verification logs.

---

## 14. Non-goals for first release

- No deletion of audit events.
- No direct editing of submitted monitoring values.
- No fabrication of live integration success.
- No admin ability to mark sandbox inventory as issued.
- No admin ability to mark estimated impact as verified.
- No plaintext key handling.
- No password sharing or cross-platform token sharing.

---

## Appendix A: Current route inventory available to admin views

The admin dashboard can initially consume these existing endpoints:

- GET /api/v1/health
- GET /api/v1/auth/me
- GET /api/v1/projects
- GET /api/v1/projects/:id/activities
- GET /api/v1/projects/:id/dashboard
- GET /api/v1/projects/:id/wallet
- GET /api/v1/projects/:id/reconciliation
- GET /api/v1/projects/:id/documents
- GET /api/v1/monitors
- GET /api/v1/methodologies
- GET /api/v1/verification-requests
- GET /api/v1/marketplace/listings
- GET /api/v1/achievements/:id/proofs
- GET /api/v1/users/me/identity
- GET /api/v1/anchors/:id/status
- GET /api/v1/sync/delta

All admin-specific aggregates and mutations require new endpoints as listed in section 7.

---

## Appendix B: Suggested admin KPI definitions

| KPI | Definition | State handling |
|---|---|---|
| Active users | Users with status = active | Identity state shown separately |
| Pending identity | identity_provisions.state in (pending, provisioning) | Not counted as active |
| Failed identity | identity_provisions.state = failed | Requires retry |
| Verified activities | Activities with status verified | Only after independent verification |
| Completion claimed | Activities with status completion_claimed | Not verified |
| Confirmed anchors | Anchor attempts with confirmed chain readback | Pending/submitted not counted |
| Issued credits | Credit batches with reconciled registry issuance | Sandbox excluded |
| Available credits | Sum of available quantity for reconciled batches | Sandbox excluded |
| Reconciliation cases open | Cases with unresolved status | Blocks affected inventory |
| Evidence gaps | Activities missing required evidence | Honest missing state |

---

*End of specification.*

