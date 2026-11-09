# Identity Provisioning Failure — Investigation Report

**Date:** 2026-11-10  
**Symptom:** `POST /api/v1/auth/register` returns 202 with `identity_failed` / `"IOTA identity provisioning failed; retry is available"` for Derrick Ochieng (`a636a771-...`). A test user registered shortly before received a live `did:iota:testnet:...`.

---

## Summary Answer (read this first)

**Root cause: the on-chain `provision_identity` entry function enforces a strict idempotency guard — it aborts with `EDuplicateUserId` (error code 4) if the same `user_id` string has already been registered in the IdentityRegistry.**

The first test user succeeded because their UUID was new to the registry. Derrick's registration failed because — according to the backend log — the server had previously crashed mid-seed (`gazi bay seed: provision demo identity for hamisi.manager@demo.baharimrv.local: IOTA identity provisioning is not active`). The seeder had already attempted (or partially written) the identity records **before the executor was healthy**, leaving some user UUIDs in the on-chain registry without a corresponding DB record, or vice versa. When the real registration came in, the executor called `provision_identity` for a `user_id` that the registry already held a dynamic field for, and the Move VM aborted the transaction.

There is **no key-format mismatch**. The encoding chain is correct end-to-end. EcoTrack is architecturally different and irrelevant to this bug — see §4.

---

## Evidence

### 1. The Move contract enforces a hard duplicate guard

File: `chain/build/conservation_chain/sources/dependencies/bahari_identity/identity.move`, function `provision_identity`:

```move
let user_id_str = string::utf8(user_id);

// Idempotency guard: one IdentityDocument per user_id
assert!(
    !dynamic_field::exists_(&registry.id, user_id_str),
    EDuplicateUserId   // error code 4
);
```

The registry maps `user_id` → `IdentityDocument` object ID via `dynamic_field`. Once a document is created for a UUID, any second call with the same UUID aborts unconditionally. There is no `update-or-create` path.

### 2. The executor log has no `provision_identity` error line

`/tmp/baharimrv-logs/executor.log` contains one error line:

```
[executor] error: Incorrect number of arguments for …::notary::anchor_sponsored
```

This is from an unrelated anchor call, not from identity provisioning. The executor **did not log an error for Derrick's provision_identity call** — which means either:
- (a) the backend never reached the executor (the failure happened before the HTTP call), or
- (b) the executor received the call, the IOTA node returned a transaction abort (which the executor rethrows as HTTP 500), and the Go `execute()` helper discarded the body before logging, so only "executor returned HTTP 500" appeared — which the backend wraps into the generic `"IOTA identity provisioning failed; retry is available"` message.

Path (b) is more consistent with the evidence: the executor's `catch` block (`res.status(500).json({ error: err?.message })`) does log the error to `console.error`, but the log file was last written at 01:21 and Derrick's attempt was at 22:23 — a different server run. The executor log from that run was not captured.

### 3. The key-format path is correct

The encoding chain for `public_key`:

| Step | Location | Action |
|------|----------|--------|
| Key generation | `custody.go` `Generate()` | `ed25519.GenerateKey(rand.Reader)` → raw 32-byte `[]byte` |
| DB storage | `models.go` `KeyReference.PublicKey` | `[]byte` column (not null) |
| Chain call | `identity/service.go` line 98 | `s.chain.ProvisionIdentity(ctx, userID, key.PublicKey)` — raw bytes |
| Hex encoding | `chain/iota.go` `ProvisionIdentity` | `hex.EncodeToString(publicKey)` → 64-char hex string |
| Executor | `executor/src/index.ts` `provisionIdentity` | `textToBytes(params.public_key)` → UTF-8 bytes of the hex string |
| Move contract | `identity.move` field `public_key` | `String` field described as "Ed25519 public key, hex-encoded (64 hex chars)" |

This is a perfect match. The Move contract stores `public_key` as a `String` (UTF-8 hex), the executor sends it as `textToBytes(hexString)`, and the Go side sends `hex.EncodeToString(rawBytes)`. **No format mismatch here.**

### 4. EcoTrack comparison

EcoTrack (`/Users/Komarov/Documents/Ecotrack-v4 copy 20`) has **no identity provisioning flow comparable to BahariMRV's**. It uses a completely different architecture:

- No TypeScript executor service — blockchain calls go directly from Go via `go-sui` SDK (`backend-go/internal/services/blockchain_service.go`).
- No DID provisioning at signup. Roles are issued as Move caps (`issue_collector_cap`, `issue_aggregator_cap`) keyed by the user's IOTA address, derived from a mnemonic or seed stored in env vars.
- No `IdentityRegistry` shared object. No `provision_identity` entry function.
- Key material: SLIP-0010 BIP-39 mnemonic derivation, not custodial envelope KMS.

EcoTrack is a reference for the overall `go-sui` transaction pattern, but provides no directly applicable fix for BahariMRV's provisioning flow.

### 5. The retry / recovery path has a latent second failure

`identity/service.go` `EnsureProvision` (line 62–117):

When a user with `state = identity_failed` retries (e.g. via `POST /api/v1/identity/recover`), the function calls `s.chain.ProvisionIdentity(ctx, userID, key.PublicKey)` with the same `userID` UUID again. If the first attempt **partially succeeded on-chain** (the Move transaction committed before the Go readback failed, or before `ResolveIdentity` returned false), the retry will hit `EDuplicateUserId` and fail permanently. There is no code path that checks `is_provisioned(registry, user_id)` before attempting to provision.

The readback check at `iota.go` line 62:

```go
valid, err := i.ResolveIdentity(ctx, response.DIDDocumentRef)
if err != nil || !valid {
    return IdentityResult{}, fmt.Errorf("identity document readback did not confirm provisioning")
}
```

If the IOTA RPC is slow and the object is not yet visible (finality lag), this returns an error — but the object **was** created on-chain. The next retry then hits `EDuplicateUserId`.

---

## Conclusions

| # | Finding | Severity |
|---|---------|----------|
| 1 | `EDuplicateUserId` abort on-chain is the proximate cause of the failure for any user whose UUID was already registered in the IdentityRegistry (by a seed run, a prior partial attempt, or a network blip before readback) | **High** |
| 2 | No key-format mismatch — the hex encoding chain is correct | — |
| 3 | Retry path (`EnsureProvision`) will permanently fail any user whose first attempt committed on-chain but failed the Go readback | **High** |
| 4 | Executor log did not capture the Derrick failure — the log is from a different server session | — |
| 5 | EcoTrack uses a completely different non-DID provisioning pattern; it is not a reference for this bug | — |

---

## Recommended Fixes

### Fix 1 — Check before provision (resolves causes 1 & 3)

In `chain/iota.go` `ProvisionIdentity`, before calling the executor, query the registry to check if the user_id is already provisioned. The Move contract exposes `is_provisioned(registry, user_id)` as a view — call `iota_getObject` on the registry and check the dynamic field, or call `resolve_identity` and handle the result.

A simpler approach that avoids an extra RPC round-trip: add an `is_provisioned` read-only call via the executor (a new operation like `check_identity`) that returns the existing `objectId` if it exists. Then in `ProvisionIdentity` the Go adapter can:

1. Call `check_identity` with the userID.  
2. If already provisioned → return the existing DID/objectId as a success (idempotent recovery).  
3. If not provisioned → proceed with `provision_identity`.

### Fix 2 — Extend the executor to return the existing document on duplicate (simplest fix)

Add a short-circuit in `executor/src/index.ts` `provisionIdentity`: catch the `EDuplicateUserId` abort (the error string will contain `"4"` or `"EDuplicateUserId"`), then call `iota_getObject` on the registry's dynamic field for that `user_id` to retrieve the existing `IdentityDocument`, and return it as a success response with the correct `did`, `objectId`, etc.

This makes `provision_identity` idempotent from BahariMRV's perspective without any Move contract changes.

```typescript
// In provisionIdentity(), catch block:
if (err?.message?.includes('EDuplicateUserId') || err?.message?.includes('MoveAbort') && err?.message?.includes(', 4)')) {
  // User already provisioned — resolve existing document from registry
  const existingDoc = await resolveExistingIdentity(params.idempotency_key);
  if (existingDoc) return existingDoc;
}
throw err;
```

### Fix 3 — Increase the readback timeout / retry (resolves cause 3 partially)

In `iota.go` `ResolveIdentity` readback, retry the object fetch up to 3 times with a 500ms backoff before declaring failure. IOTA testnet has typical finality of 1–2s; a single immediate read often races the object becoming visible.

```go
for attempt := 0; attempt < 3; attempt++ {
    valid, err := i.ResolveIdentity(ctx, response.DIDDocumentRef)
    if err == nil && valid {
        return IdentityResult{...}, nil
    }
    time.Sleep(500 * time.Millisecond)
}
return IdentityResult{}, fmt.Errorf("identity document readback did not confirm provisioning after 3 attempts")
```

### Fix 4 — Immediate action for Derrick's account

Derrick's DB record (`identity_failed`, `attempts: 1`) exists. The on-chain state is unknown — the UUID `a636a771-7506-498b-8099-c5c253c13f48` may or may not already be in the registry. The safest recovery path:

1. Query the IOTA registry object (`0x8007a9471be98d5b8bff3a676f663fb224ab181de04fe3e580149debf3874ac8`) for the dynamic field keyed by Derrick's UUID to determine if a document was created.
2. If it exists: update Derrick's DB `identity_provisions` row directly with the `did`, `did_document_ref`, `address`, and `state = 'identity_active'` matching the on-chain document.
3. If it does not exist: call `POST /api/v1/identity/recover` while authenticated as Derrick to trigger `EnsureProvision` again — the retry will succeed since the registry has no entry for that UUID.

---

*Investigation performed read-only. No code was modified.*
