/// BahariMRV — On-Chain Planting Record
///
/// Privacy-safe, human-readable summary of a verified conservation planting event.
/// Frozen permanently once created — corrections create superseding records.
///
/// Relationship to ProvenanceAnchor:
///   ProvenanceAnchor = cryptographic commitment (hash only, verifiable)
///   PlantingRecord   = auditable summary (species, counts, area, hashes — readable)
///
/// Both are anchored on-chain. A verifier can:
///   1. Read PlantingRecord.evidenceRoot
///   2. Recompute from downloaded field evidence
///   3. Confirm SHA-256(canonical_claim) == ProvenanceAnchor.commitment
module conservation_chain::planting_record {
    use iota::object::{Self, UID, ID};
    use iota::tx_context::{TxContext};
    use iota::transfer;
    use iota::event;
    use iota::clock::{Self, Clock};
    use std::string::{Self, String};
    use std::option::{Self, Option};
    use conservation_chain::roles::{AdminCap, ManagerCap};

    // ─── Error codes ─────────────────────────────────────────────────────────

    const EEmptyEvidenceRoot:    u64 = 1;
    const EEmptyProjectDID:      u64 = 2;
    const EZeroSeedlings:        u64 = 3;

    // ─── Planting record ─────────────────────────────────────────────────────

    /// Permanently frozen, publicly readable conservation planting summary.
    /// Created after a ProvenanceAnchor is confirmed on-chain.
    public struct PlantingRecord has key {
        id: UID,

        projectDID:             String,  // did:iota:testnet:project:<mrvProjectId>
        ledgerId:               ID,      // → ProjectImpactLedger

        /// Proof chain references
        provenanceAnchorId:     ID,      // → ProvenanceAnchor (required)
        claimAnchorId:          Option<ID>, // → RestorationClaimAnchor (Jahazi-funded only)

        /// Field data — aggregated, privacy-safe
        /// No raw GPS coordinates, no beneficiary names, no media
        species:                String,
        seedlingsPlanted:       u64,
        seedlingsAlive:         u64,
        survivalPctHundredths:  u64,    // 9500 = 95.00%
        areaHaMillionths:       u64,    // 500000 = 0.500000 ha
        methodologyVersion:     String,
        reportingPeriodStartMs: u64,
        reportingPeriodEndMs:   u64,

        /// Evidence — hashes only (media stays off-chain in restricted storage)
        evidenceRoot:           String, // Merkle root of field evidence SHA-256s
        satelliteCorrobHash:    String, // Hash of satellite proof (empty if unavailable)
        photoHash:              String, // Hash of geo-tagged field photo (privacy-safe)
        locationHash:           String, // SHA-256 of "lat|lng" string

        /// Verification
        verifierDID:            String, // Empty if achieved_unverified only
        verifierAttestationHash: String,
        isVerified:             bool,   // true = achievement_verified; false = achieved_unverified

        /// Correction chain — additive, original never deleted
        previousRecordId:       Option<ID>, // Set when correcting an earlier record
        supersededByEventOnly:  bool,       // True if superseded via emit_superseded event

        /// Metadata
        recordedAtMs:           u64,
        mrvActivityId:          String,   // BahariMRV backend activity UUID
    }

    // ─── Events ───────────────────────────────────────────────────────────────

    public struct PlantingRecordCreated has copy, drop {
        recordId:            address,
        projectDID:          String,
        ledgerId:            ID,
        species:             String,
        seedlingsPlanted:    u64,
        isVerified:          bool,
        claimAnchorId:       Option<ID>,
        provenanceAnchorId:  ID,
        mrvActivityId:       String,
    }

    public struct PlantingRecordSuperseded has copy, drop {
        oldRecordId: ID,
        newRecordId: ID,
        projectDID:  String,
    }

    // ─── Entry functions ─────────────────────────────────────────────────────

    /// Create and freeze a PlantingRecord for a verified conservation activity.
    ///
    /// Called by the BahariMRV executor after a ProvenanceAnchor is confirmed.
    /// The planting_entry is then appended to the ProjectImpactLedger separately.
    public entry fun create_planting_record(
        _cap:                    &ManagerCap,
        project_did:             vector<u8>,
        ledger_id:               ID,
        provenance_anchor_id:    ID,
        claim_anchor_id:         Option<ID>,
        species:                 vector<u8>,
        seedlings_planted:       u64,
        seedlings_alive:         u64,
        survival_pct_hundredths: u64,
        area_ha_millionths:      u64,
        methodology_version:     vector<u8>,
        period_start_ms:         u64,
        period_end_ms:           u64,
        evidence_root:           vector<u8>,
        satellite_corrob_hash:   vector<u8>,
        photo_hash:              vector<u8>,
        location_hash:           vector<u8>,
        verifier_did:            vector<u8>,
        verifier_attestation_hash: vector<u8>,
        is_verified:             bool,
        previous_record_id:      Option<ID>,
        mrv_activity_id:         vector<u8>,
        clock:                   &Clock,
        ctx:                     &mut TxContext
    ) {
        assert!(!project_did.is_empty(),  EEmptyProjectDID);
        assert!(!evidence_root.is_empty(), EEmptyEvidenceRoot);
        assert!(seedlings_planted > 0,    EZeroSeedlings);

        let record_uid   = object::new(ctx);
        let record_addr  = object::uid_to_address(&record_uid);
        let proj_str     = string::utf8(project_did);
        let activity_str = string::utf8(mrv_activity_id);

        let record = PlantingRecord {
            id:                      record_uid,
            projectDID:              proj_str,
            ledgerId:                ledger_id,
            provenanceAnchorId:      provenance_anchor_id,
            claimAnchorId:           claim_anchor_id,
            species:                 string::utf8(species),
            seedlingsPlanted:        seedlings_planted,
            seedlingsAlive:          seedlings_alive,
            survivalPctHundredths:   survival_pct_hundredths,
            areaHaMillionths:        area_ha_millionths,
            methodologyVersion:      string::utf8(methodology_version),
            reportingPeriodStartMs:  period_start_ms,
            reportingPeriodEndMs:    period_end_ms,
            evidenceRoot:            string::utf8(evidence_root),
            satelliteCorrobHash:     string::utf8(satellite_corrob_hash),
            photoHash:               string::utf8(photo_hash),
            locationHash:            string::utf8(location_hash),
            verifierDID:             string::utf8(verifier_did),
            verifierAttestationHash: string::utf8(verifier_attestation_hash),
            isVerified:              is_verified,
            previousRecordId:        previous_record_id,
            supersededByEventOnly:   false,
            recordedAtMs:            clock::timestamp_ms(clock),
            mrvActivityId:           activity_str,
        };

        event::emit(PlantingRecordCreated {
            recordId:           record_addr,
            projectDID:         proj_str,
            ledgerId:           ledger_id,
            species:            string::utf8(species),
            seedlingsPlanted:   seedlings_planted,
            isVerified:         is_verified,
            claimAnchorId:      claim_anchor_id,
            provenanceAnchorId: provenance_anchor_id,
            mrvActivityId:      activity_str,
        });

        transfer::freeze_object(record);
    }

    /// Emit a supersession event when a new record corrects an old one.
    /// The old record stays on-chain (frozen, immutable) — only the event signals supersession.
    /// Verifiers follow the supersession chain: newRecord.previousRecordId → old record.
    public entry fun emit_superseded(
        _admin:       &AdminCap,
        old_record_id: ID,
        new_record_id: ID,
        project_did:  vector<u8>,
    ) {
        event::emit(PlantingRecordSuperseded {
            oldRecordId: old_record_id,
            newRecordId: new_record_id,
            projectDID:  string::utf8(project_did),
        });
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun get_project_did(r: &PlantingRecord): String       { r.projectDID }
    public fun get_species(r: &PlantingRecord): String            { r.species }
    public fun get_planted(r: &PlantingRecord): u64               { r.seedlingsPlanted }
    public fun get_alive(r: &PlantingRecord): u64                 { r.seedlingsAlive }
    public fun get_evidence_root(r: &PlantingRecord): String      { r.evidenceRoot }
    public fun get_provenance_id(r: &PlantingRecord): ID          { r.provenanceAnchorId }
    public fun get_claim_id(r: &PlantingRecord): Option<ID>       { r.claimAnchorId }
    public fun get_ledger_id(r: &PlantingRecord): ID              { r.ledgerId }
    public fun is_verified(r: &PlantingRecord): bool              { r.isVerified }
    public fun get_mrv_activity_id(r: &PlantingRecord): String    { r.mrvActivityId }
}
