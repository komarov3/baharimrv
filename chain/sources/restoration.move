/// BahariMRV — Restoration Claim and Fulfillment
///
/// Two frozen on-chain records that bridge Jahazi tourist contributions
/// to BahariMRV conservation activities:
///
/// RestorationClaimAnchor — created when Jahazi submits a claim to BahariMRV.
///   Proves: "N tourists committed X USD for Y seedlings of species Z in project P
///           between period [start, end]"
///
/// RestorationFulfillmentRecord — created when a planting activity matches the claim.
///   Proves: "The claim was fulfilled: A seedlings planted, evidence root = ...,
///           linked to ProvenanceAnchor and PlantingRecord"
///
/// These two objects, plus PlantingRecord and ProvenanceAnchor, form the complete
/// on-chain proof chain that tourist certificates reference.
module conservation_chain::restoration {
    use iota::object::{Self, UID, ID};
    use iota::tx_context::{TxContext};
    use iota::transfer;
    use iota::event;
    use iota::clock::{Self, Clock};
    use std::string::{Self, String};
    use std::option::{Self, Option};
    use conservation_chain::roles::{AdminCap, ManagerCap};

    // ─── Error codes ─────────────────────────────────────────────────────────

    const EEmptyClaimId:     u64 = 1;
    const EEmptyMerkleRoot:  u64 = 2;
    const EZeroUnits:        u64 = 3;

    // ─── RestorationClaimAnchor ───────────────────────────────────────────────

    /// Frozen proof that Jahazi submitted a tourist-funded restoration claim.
    /// Created by the BahariMRV executor when a RestorationClaim is received from Jahazi.
    /// The contributorMerkleRoot protects tourist privacy — no individual amounts on-chain.
    public struct RestorationClaimAnchor has key {
        id: UID,

        /// Jahazi's claim identifier (e.g. "rc_abc123def456")
        jahaziClaimId:          String,
        jahaziProjectDID:       String,  // did:iota:testnet:project:<jahaziProjectId>
        mrvProjectDID:          String,  // did:iota:testnet:project:<mrvProjectId>

        /// The claim period — when contributions were escrowed
        periodStartMs:          u64,
        periodEndMs:            u64,

        /// Financial and unit data
        totalCostCents:         u64,    // USD cents — no floats
        totalUnits:             u64,    // Seedlings claimed
        species:                String,
        contributorCount:       u64,

        /// Privacy-preserving commitment to contributor breakdown.
        /// Merkle root of leaves = SHA256(touristDID + ":" + amountCents + ":" + seedlings + ":" + nonce)
        /// Jahazi retains the full tree off-chain for tourist proof generation.
        contributorMerkleRoot:  String,

        /// Geographic context — hash of "lat|lng" string, no raw coordinates
        locationHash:           String,

        /// Jahazi platform identity that signed this claim
        jahaziPlatformDID:      String,

        /// Nonce from the claim payload (prevents replay)
        nonce:                  String,

        /// Anchoring timestamp
        anchoredAtMs:           u64,
    }

    // ─── RestorationFulfillmentRecord ─────────────────────────────────────────

    /// Frozen proof that a restoration claim was fulfilled by a planting activity.
    /// Created when a PlantingRecord matches a RestorationClaimAnchor.
    /// Tourist certificates reference this object as their impact proof.
    public struct RestorationFulfillmentRecord has key {
        id: UID,

        /// Links to all three on-chain proof objects
        claimAnchorId:      ID,  // → RestorationClaimAnchor
        plantingRecordId:   ID,  // → PlantingRecord
        provenanceAnchorId: ID,  // → ProvenanceAnchor

        /// What actually happened vs. what was claimed
        actualUnitsPlanted: u64,
        claimedUnits:       u64,  // From the claim (for auditors to compare)
        fulfilledAtMs:      u64,

        /// Project identity
        mrvProjectDID:      String,

        /// Number of certificates issued against this fulfillment
        /// (informational — certificate objects are the authoritative source)
        certificatesIssuedCount: u64,
    }

    // ─── Events ───────────────────────────────────────────────────────────────

    public struct ClaimAnchored has copy, drop {
        claimAnchorId:    address,
        jahaziClaimId:    String,
        mrvProjectDID:    String,
        totalUnits:       u64,
        contributorCount: u64,
        totalCostCents:   u64,
    }

    public struct ClaimMatchedEvent has copy, drop {
        claimAnchorId:    ID,
        plantingRecordId: ID,
        projectDID:       String,
    }

    public struct FulfillmentRecorded has copy, drop {
        fulfillmentId:      address,
        claimAnchorId:      ID,
        plantingRecordId:   ID,
        provenanceAnchorId: ID,
        actualUnits:        u64,
        claimedUnits:       u64,
    }

    // ─── Entry functions ─────────────────────────────────────────────────────

    /// Anchor a Jahazi restoration claim on-chain.
    ///
    /// Called by the BahariMRV executor when it receives a RestorationClaim via webhook.
    /// This happens BEFORE the matching planting activity — it's the upfront commitment.
    public entry fun anchor_restoration_claim(
        _admin:               &AdminCap,
        jahazi_claim_id:      vector<u8>,
        jahazi_project_did:   vector<u8>,
        mrv_project_did:      vector<u8>,
        period_start_ms:      u64,
        period_end_ms:        u64,
        total_cost_cents:     u64,
        total_units:          u64,
        species:              vector<u8>,
        contributor_count:    u64,
        contributor_merkle_root: vector<u8>,
        location_hash:        vector<u8>,
        jahazi_platform_did:  vector<u8>,
        nonce:                vector<u8>,
        clock:                &Clock,
        ctx:                  &mut TxContext
    ) {
        assert!(!jahazi_claim_id.is_empty(),          EEmptyClaimId);
        assert!(!contributor_merkle_root.is_empty(),  EEmptyMerkleRoot);
        assert!(total_units > 0,                      EZeroUnits);

        let anchor_uid  = object::new(ctx);
        let anchor_addr = object::uid_to_address(&anchor_uid);
        let mrv_str     = string::utf8(mrv_project_did);
        let claim_str   = string::utf8(jahazi_claim_id);

        let anchor = RestorationClaimAnchor {
            id:                     anchor_uid,
            jahaziClaimId:          claim_str,
            jahaziProjectDID:       string::utf8(jahazi_project_did),
            mrvProjectDID:          mrv_str,
            periodStartMs:          period_start_ms,
            periodEndMs:            period_end_ms,
            totalCostCents:         total_cost_cents,
            totalUnits:             total_units,
            species:                string::utf8(species),
            contributorCount:       contributor_count,
            contributorMerkleRoot:  string::utf8(contributor_merkle_root),
            locationHash:           string::utf8(location_hash),
            jahaziPlatformDID:      string::utf8(jahazi_platform_did),
            nonce:                  string::utf8(nonce),
            anchoredAtMs:           clock::timestamp_ms(clock),
        };

        event::emit(ClaimAnchored {
            claimAnchorId:    anchor_addr,
            jahaziClaimId:    claim_str,
            mrvProjectDID:    mrv_str,
            totalUnits:       total_units,
            contributorCount: contributor_count,
            totalCostCents:   total_cost_cents,
        });

        transfer::freeze_object(anchor);
    }

    /// Emit a ClaimMatchedEvent when a planting activity matches a restoration claim.
    ///
    /// Called immediately after a PlantingRecord is created that matches an open claim.
    /// The RestorationFulfillmentRecord is created separately (requires all three IDs).
    public entry fun emit_claim_matched(
        _admin:           &AdminCap,
        claim_anchor_id:  ID,
        planting_record_id: ID,
        project_did:      vector<u8>,
    ) {
        event::emit(ClaimMatchedEvent {
            claimAnchorId:    claim_anchor_id,
            plantingRecordId: planting_record_id,
            projectDID:       string::utf8(project_did),
        });
    }

    /// Record the fulfillment of a restoration claim.
    ///
    /// Called after the claim is matched AND the MRV link is pushed to Jahazi.
    /// This is the object tourist certificates link back to as their final proof.
    public entry fun record_fulfillment(
        _cap:               &ManagerCap,
        claim_anchor_id:    ID,
        planting_record_id: ID,
        provenance_anchor_id: ID,
        actual_units:       u64,
        claimed_units:      u64,
        mrv_project_did:    vector<u8>,
        certs_issued_count: u64,
        clock:              &Clock,
        ctx:                &mut TxContext
    ) {
        let rec_uid  = object::new(ctx);
        let rec_addr = object::uid_to_address(&rec_uid);

        let record = RestorationFulfillmentRecord {
            id:                      rec_uid,
            claimAnchorId:           claim_anchor_id,
            plantingRecordId:        planting_record_id,
            provenanceAnchorId:      provenance_anchor_id,
            actualUnitsPlanted:      actual_units,
            claimedUnits:            claimed_units,
            fulfilledAtMs:           clock::timestamp_ms(clock),
            mrvProjectDID:           string::utf8(mrv_project_did),
            certificatesIssuedCount: certs_issued_count,
        };

        event::emit(FulfillmentRecorded {
            fulfillmentId:      rec_addr,
            claimAnchorId:      claim_anchor_id,
            plantingRecordId:   planting_record_id,
            provenanceAnchorId: provenance_anchor_id,
            actualUnits:        actual_units,
            claimedUnits:       claimed_units,
        });

        transfer::freeze_object(record);
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun get_claim_id(a: &RestorationClaimAnchor): String          { a.jahaziClaimId }
    public fun get_mrv_did(a: &RestorationClaimAnchor): String           { a.mrvProjectDID }
    public fun get_total_units(a: &RestorationClaimAnchor): u64          { a.totalUnits }
    public fun get_contributor_count(a: &RestorationClaimAnchor): u64    { a.contributorCount }
    public fun get_merkle_root(a: &RestorationClaimAnchor): String       { a.contributorMerkleRoot }
    public fun get_species(a: &RestorationClaimAnchor): String           { a.species }
    public fun fulfillment_claim_id(r: &RestorationFulfillmentRecord): ID  { r.claimAnchorId }
    public fun fulfillment_planting_id(r: &RestorationFulfillmentRecord): ID { r.plantingRecordId }
    public fun fulfillment_actual_units(r: &RestorationFulfillmentRecord): u64 { r.actualUnitsPlanted }
}
