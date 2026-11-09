/// BahariMRV — Conservation Impact Certificates
///
/// Soul-bound on-chain badges issued to tourists who funded verified planting.
/// "Soul-bound" = has key but no store ability, so the holder cannot re-transfer.
///
/// The certificate carries the full proof chain:
///   plantingRecordId    → PlantingRecord (what was planted, when, where)
///   claimAnchorId       → RestorationClaimAnchor (what the tourist funded)
///   provenanceAnchorId  → ProvenanceAnchor (cryptographic commitment)
///   fulfillmentRecordId → RestorationFulfillmentRecord (claim ↔ planting link)
///
/// A tourist's wallet app can traverse the full chain without touching any database.
module conservation_chain::certificates {
    use iota::object::{Self, UID, ID};
    use iota::tx_context::{TxContext};
    use iota::transfer;
    use iota::event;
    use iota::clock::{Self, Clock};
    use std::string::{Self, String};
    use std::option::{Option};
    use conservation_chain::roles::AdminCap;

    // ─── Certificate NFT ─────────────────────────────────────────────────────

    /// Soul-bound conservation impact certificate — one per tourist per fulfillment.
    /// No `store` ability → holder cannot transfer it; it's permanently theirs.
    public struct ConservationCertificate has key {
        id: UID,

        /// Jahazi certificate code (e.g. "JZ-A3B7C2D1E4")
        certificateCode:     String,

        /// Holder identity
        holderName:          String,
        holderDID:           String,  // did:iota:testnet:<userId>

        /// Project
        projectName:         String,
        projectDID:          String,  // did:iota:testnet:project:<mrvProjectId>

        /// What was planted
        species:             String,
        seedlingsFunded:     u64,     // Attributed to this tourist specifically
        co2eMilligrams5yr:   u64,     // 5-yr CO₂ estimate × 1000

        /// When
        plantingDateMs:      u64,

        /// Full proof chain — traverse to verify without a database
        plantingRecordId:    ID,          // → PlantingRecord
        claimAnchorId:       Option<ID>,  // → RestorationClaimAnchor (Jahazi-funded only)
        provenanceAnchorId:  ID,          // → ProvenanceAnchor
        fulfillmentRecordId: Option<ID>,  // → RestorationFulfillmentRecord

        /// Funding and chain state
        fundingStatus:       String,  // "certified" | "disbursed"
        anchoringStatus:     String,  // "anchored" | "pending" | "failed"
        issuedAtMs:          u64,
    }

    // ─── Events ───────────────────────────────────────────────────────────────

    public struct CertificateIssued has copy, drop {
        certificateCode:    String,
        holderDID:          String,
        projectDID:         String,
        species:            String,
        seedlingsFunded:    u64,
        co2eMilligrams5yr:  u64,
        plantingRecordId:   ID,
        provenanceAnchorId: ID,
        objectId:           address,
    }

    public struct CertificateInvalidated has copy, drop {
        certificateCode: String,
        holderDID:       String,
        reason:          String,
    }

    // ─── Entry functions ─────────────────────────────────────────────────────

    /// Issue a soul-bound conservation certificate to a tourist.
    ///
    /// Called by the Jahazi executor after verifyAndIssueCertificates() completes.
    /// All four proof chain IDs should be populated where available.
    public entry fun issue_certificate(
        _admin:              &AdminCap,
        recipient:           address,
        certificate_code:    vector<u8>,
        holder_name:         vector<u8>,
        holder_did:          vector<u8>,
        project_name:        vector<u8>,
        project_did:         vector<u8>,
        species:             vector<u8>,
        seedlings_funded:    u64,
        co2e_mg_5yr:         u64,
        planting_date_ms:    u64,
        planting_record_id:  ID,
        claim_anchor_id:     Option<ID>,
        provenance_anchor_id: ID,
        fulfillment_record_id: Option<ID>,
        funding_status:      vector<u8>,
        anchoring_status:    vector<u8>,
        clock:               &Clock,
        ctx:                 &mut TxContext
    ) {
        let cert_uid  = object::new(ctx);
        let cert_addr = object::uid_to_address(&cert_uid);
        let code_str  = string::utf8(certificate_code);
        let did_str   = string::utf8(holder_did);
        let proj_str  = string::utf8(project_did);

        let cert = ConservationCertificate {
            id:                  cert_uid,
            certificateCode:     code_str,
            holderName:          string::utf8(holder_name),
            holderDID:           did_str,
            projectName:         string::utf8(project_name),
            projectDID:          proj_str,
            species:             string::utf8(species),
            seedlingsFunded:     seedlings_funded,
            co2eMilligrams5yr:   co2e_mg_5yr,
            plantingDateMs:      planting_date_ms,
            plantingRecordId:    planting_record_id,
            claimAnchorId:       claim_anchor_id,
            provenanceAnchorId:  provenance_anchor_id,
            fulfillmentRecordId: fulfillment_record_id,
            fundingStatus:       string::utf8(funding_status),
            anchoringStatus:     string::utf8(anchoring_status),
            issuedAtMs:          clock::timestamp_ms(clock),
        };

        event::emit(CertificateIssued {
            certificateCode:    code_str,
            holderDID:          did_str,
            projectDID:         proj_str,
            species:            string::utf8(species),
            seedlingsFunded:    seedlings_funded,
            co2eMilligrams5yr:  co2e_mg_5yr,
            plantingRecordId:   planting_record_id,
            provenanceAnchorId: provenance_anchor_id,
            objectId:           cert_addr,
        });

        // Soul-bound: transferred to tourist but has no `store`, so cannot be re-transferred
        transfer::transfer(cert, recipient);
    }

    /// Emit an invalidation event when evidence is later found fraudulent.
    /// The on-chain certificate cannot be deleted (owned by tourist), but this
    /// event signals verifiers it is no longer authoritative.
    public entry fun invalidate_certificate(
        _admin:           &AdminCap,
        certificate_code: vector<u8>,
        holder_did:       vector<u8>,
        reason:           vector<u8>,
    ) {
        event::emit(CertificateInvalidated {
            certificateCode: string::utf8(certificate_code),
            holderDID:       string::utf8(holder_did),
            reason:          string::utf8(reason),
        });
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun get_code(c: &ConservationCertificate): String           { c.certificateCode }
    public fun get_holder_did(c: &ConservationCertificate): String     { c.holderDID }
    public fun get_project_did(c: &ConservationCertificate): String    { c.projectDID }
    public fun get_species(c: &ConservationCertificate): String        { c.species }
    public fun get_seedlings(c: &ConservationCertificate): u64         { c.seedlingsFunded }
    public fun get_co2e_mg(c: &ConservationCertificate): u64           { c.co2eMilligrams5yr }
    public fun get_planting_record_id(c: &ConservationCertificate): ID { c.plantingRecordId }
    public fun get_provenance_id(c: &ConservationCertificate): ID      { c.provenanceAnchorId }
}
