# BahariMRV — conservation_chain Move Package

On-chain contracts for IOTA Rebased Testnet. Handles conservation provenance
anchoring, IOTA Identity DID provisioning, and soul-bound conservation
certificates for tourist contributors.

## Modules

| Module | Purpose |
|---|---|
| `roles.move` | Role capabilities: AdminCap, ManagerCap, MonitorCap, VerifierCap |
| `notary.move` | Immutable provenance anchors — evidence_submitted / achieved_unverified / achievement_verified |
| `identity.move` | IOTA Identity DID provisioning for users, projects, and operators |
| `certificates.move` | Soul-bound conservation impact certificates for tourist funders |

## Why not reuse EcoTrack's `plastic_chain`?

EcoTrack's contract is domain-locked to plastic (CollectionRecord, MaterialPassport,
weight_grams, plastic_type, ward_name, EPR credits). Conservation anchors need
species, seedlings_funded, co2e_5yr, methodology_version, evidence_root, verifier_did.
Mixing plastic and conservation in one package would confuse the on-chain audit trail
that carbon buyers and verifiers read.

The architectural patterns (caps, notary, registry, soul-bound badges, events) are
directly reused. The EcoTrack executor service infrastructure is reused as-is — it
is package-agnostic and just needs `IOTA_PACKAGE_ID` updated.

## DID conventions

| Entity | DID format |
|---|---|
| User (tourist / manager / monitor / verifier) | `did:iota:testnet:<userId>` |
| Conservation project | `did:iota:testnet:project:<projectId>` |
| Tourism operator (Jahazi) | `did:iota:testnet:operator:<operatorId>` |
| Platform issuer | `did:iota:testnet:platform:baharimrv` |

## Deployment (IOTA Testnet)

### Prerequisites

```bash
# IOTA CLI 1.23.x (must match the cached framework version)
iota --version

# Funded testnet wallet
iota client active-address
iota client gas   # must show enough IOTA for gas
```

### Get testnet tokens

```bash
iota client faucet --address $(iota client active-address)
```

### Publish the package

```bash
cd chain/
iota client publish --gas-budget 100000000
```

This outputs:
```
Transaction Digest: <DIGEST>
Published Objects:
  PackageID: <PACKAGE_ID>
  UpgradeCap: <UPGRADE_CAP_ID>
```

Set these in BahariMRV backend `.env`:

```bash
CHAIN_MODE=iota
IOTA_PACKAGE_ID=<PACKAGE_ID>
IOTA_EXECUTOR_URL=<executor service URL>
IOTA_EXPLORER_URL=https://explorer.rebased.iota.org/testnet
```

### After deployment: extract shared object IDs

The `init` functions create two shared objects on deployment:
- `AnchorRegistry` (from notary.move)
- `IdentityRegistry` (from identity.move)

These object IDs are needed by the executor service. Find them in the publish
transaction output under "Created Objects", or query:

```bash
iota client objects --address $(iota client active-address)
```

Set in executor service config:
```bash
ANCHOR_REGISTRY_OBJECT_ID=<object ID>
IDENTITY_REGISTRY_OBJECT_ID=<object ID>
```

## Local development version mismatch

The test runner (`iota move test`) fails with `MISSING_DEPENDENCY: tx_context`
if the cached framework version (in `~/.move/`) doesn't match the installed CLI.

**Fix:**
```bash
# Option A — update CLI to match the cached framework
brew upgrade iota

# Option B — clear the cache and refetch
rm -rf ~/.move/https___github_com_iotaledger_iota_git_testnet
iota move test   # will fetch the correct version for the current CLI
```

The contract sources compile cleanly (`iota move build` succeeds with zero errors).
The test failures are an environment issue, not a code bug.

## Executor service contract

The Go backend's `chain.IOTA` adapter (`internal/chain/iota.go`) sends HTTP POST
requests to `IOTA_EXECUTOR_URL` with this JSON shape:

```json
{
  "operation": "anchor",
  "package_id": "<PACKAGE_ID>",
  "idempotency_key": "<ProvenanceEvent UUID>",
  "kind": "achieved_unverified",
  "commitment": "<sha256 hex>",
  "canonical_json": "<base64 canonical JSON bytes>",
  "issuer_did": "did:iota:testnet:...",
  "previous_object_id": "<object ID or empty string>",
  "network": "testnet"
}
```

And for identity provisioning:

```json
{
  "operation": "provision_identity",
  "package_id": "<PACKAGE_ID>",
  "idempotency_key": "<user UUID>",
  "public_key": "<hex Ed25519 public key>",
  "network": "testnet"
}
```

The executor must:
1. Build a Move PTB (Programmable Transaction Block) calling the appropriate entry
2. Sign with its sponsored gas wallet
3. Submit to IOTA testnet RPC
4. Read back the created object to confirm
5. Return `{ "did": ..., "transaction_reference": ..., "object_id": ... }`
   or `{ "transaction_reference": ..., "object_id": ... }` for anchors

The EcoTrack executor service handles this pattern exactly. Point it at this
package instead of `plastic_chain` by updating `IOTA_PACKAGE_ID`.

## Anchor idempotency

`notary::anchor_sponsored` uses a shared `AnchorRegistry` with dynamic fields
keyed by `event_id`. A duplicate event_id aborts with `EDuplicateAnchor` (code 4).
The Go backend's idempotency key is the `ProvenanceEvent.ID` UUID — the same
event can never be double-anchored.

## Corrections are additive

When a PM correction supersedes an earlier anchor:
1. A new `ProvenanceAnchor` is created with `previous_object_id` pointing to the old one
2. `emit_superseded` emits an `AnchorSuperseded` event
3. The old anchor stays on-chain, frozen and immutable
4. Verifiers follow the supersession chain to find the authoritative anchor

This mirrors the BahariMRV SPEC principle: "Corrections are additive. Anchored
records are never rewritten."
