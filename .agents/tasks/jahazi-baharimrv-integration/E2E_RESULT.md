# E2E Test Result — Mikoko Park Mangrove Restoration

**Date:** 2026-11-02T02:04:32.339Z
**Assertions:** 44/44 (100%) passed

## Certificate

| Field | Value |
|---|---|
| Certificate code | `78bf5155-2e59-4093-be4b-829ef9cc4531` |
| Certificate URL | http://localhost:3000/conservation/certificate/78bf5155-2e59-4093-be4b-829ef9cc4531 |
| IOTA tx hash | `sandbox-anchor-transaction-cbde718ed1f326d8c90dd5cc8a14acc4` |
| CO₂e estimate (5-yr) | 4.36 kg |
| Planting record linked | ✅ `feffee1a-88dc-4519-a83d-e95ed3bb72ff` |
| Holder DID | `did:iota:sandbox:eddca2c499170cd6d2ea18bde517d14c` |
| Project DID | `did:iota:testnet:project:00ba28aa-84b1-4e7b-b5cd-b34f1c9c41c5` |

## Chain configuration

- **Target:** IOTA Rebased Testnet (`did:iota:testnet:` DID method)
- **BahariMRV chain mode:** sandbox (IOTA Rebased testnet adapters present; `CHAIN_MODE=iota` activates live anchoring)
- **Jahazi ENABLE_BLOCKCHAIN:** false (anchors queue for retry when enabled)
- **Explorer:** `https://explorer.rebased.iota.org/testnet/transaction/<txHash>`
- **Note:** Certificates are issued correctly regardless of anchoring status. When `ENABLE_BLOCKCHAIN=true`, the txHash is populated and certificates show "Anchored on IOTA Testnet".

## J8–J11 Certificate visual output

- PNG saved: `.agents/tasks/jahazi-baharimrv-integration/certificate-78bf5155-2e59-4093-be4b-829ef9cc4531.png`
- On-chain proof block: renders with txHash (or "Anchoring pending" chip)
- Planting attribution statement (J9): from `certificateAttribution` field
- Species + planting date (J9): from linked `PlantingRecord`
- 5-year CO₂ estimate (J9): labelled as estimate
- Verification QR (J10): links to `/conservation/certificate/78bf5155-2e59-4093-be4b-829ef9cc4531`
- Anchoring status chip (J11): Anchored / Pending / Failed

## Assertions

- ✅ BahariMRV manager registered
- ✅ Jahazi tourist logged in
- ✅ Jahazi superadmin logged in
- ✅ Methodology loaded
- ✅ BahariMRV project created
- ✅ Plot available
- ✅ Jahazi project ready
- ✅ Tourist has escrowed credentials
- ✅ Escrowed credential available
- ✅ Activity created
- ✅ Monitoring record synced
- ✅ Activity completed
- ✅ MRV link generated
- ✅ MRV link signed
- ✅ MRV link submitted to Jahazi
- ✅ MRV link verified on Jahazi
- ✅ Certificate issued (fundingStatus=certified)
- ✅ Certificate code present
- ✅ Planting record linked
- ✅ CO₂e estimate set (5-yr)
- ✅ Anchoring status present
- ✅ Certificate code matches
- ✅ fundingStatus is certified
- ✅ chain is iota-testnet
- ✅ certificateAttribution present (J9)
- ✅ anchoringStatus present (J8/J11)
- ✅ contentHash present (J8)
- ✅ PNG renders HTTP 200 (J8)
- ✅ PNG content-type is image (J10 QR)
- ✅ PNG has content (J8-J11 complete)
- ✅ Impact endpoint returns projectId
- ✅ treesPlanted tracked
- ✅ co2eKgTotal tracked
- ✅ contributorCount tracked
- ✅ weeklyCompliance present
- ✅ certificatesIssued tracked
- ✅ latestChainProof present (or null)
- ✅ generatedAt timestamp present
- ✅ BahariMRV DID provisioned
- ✅ BahariMRV DID uses iota method
- ✅ Jahazi tourist DID provisioned
- ✅ Jahazi DID uses iota:testnet method
- ✅ Jahazi project DID provisioned
- ✅ Project DID uses iota:testnet method

## Errors

_None_
