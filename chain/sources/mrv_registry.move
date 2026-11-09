/// BahariMRV — MRV Attestation Registry
///
/// Links catch and conservation records to their IOTA proof references.
///
/// An MRVAttestation is a shared, permanently readable on-chain record that
/// says: "backend record <record_id> of type <record_type> was independently
/// verified; the provenance proof is at IOTA object <provenance_anchor_id>."
///
/// This is the authoritative on-chain registry for BahariMRV claims. It sits
/// above the lower-level ProvenanceAnchor objects (conservation_chain::notary)
/// and the CarbonCredit tokens (conservation_chain::carbon_credit), linking them
/// into a single verifiable claim per source record.
///
/// Record types:
///   "catch"          — catch record linked to a fisheries_chain CatchAnchor
///   "conservation"   — conservation activity linked to a ProvenanceAnchor
///   "carbon_credit"  — conservation activity linked to a CarbonCredit issuance
///
/// Verification levels:
///   0 — self-reported (field monitor / fisher only)
///   1 — project-manager verified (ManagerCap holder signed off)
///   2 — independent-verifier verified (VerifierCap holder approved)
///   3 — audit-grade (government / international standard body)
///
/// The executor calls register_self_attestation (level 0) on every provenance
/// anchor write, then register_attestation (level 1+) when a manager or
/// verifier confirms the record.
module conservation_chain::mrv_registry {
    use iota::object::{Self, UID, ID};
    use iota::tx_context::{Self, TxContext};
    use iota::transfer;
    use iota::event;
    use iota::dynamic_field;
    use std::string::{Self, String};
    use std::option::{Self, Option};
    use conservation_chain::roles::{ManagerCap, VerifierCap};

    // ─── Error codes ─────────────────────────────────────────────────────────
    const EEmptyRecordId:        u64 = 1;
    const EEmptyCommitment:      u64 = 2;
    const EInvalidVerifyLevel:   u64 = 3;
    const EDuplicateAttestation: u64 = 4;
    const EInvalidRecordType:    u64 = 5;

    // ─── Structs ─────────────────────────────────────────────────────────────

    /// Shared singleton registry — maps record_id → attestation object ID.
    /// One attestation per source record prevents double-claiming.
    public struct MRVRegistry has key {
        id: UID,
        total_attestations:        u64,
        catch_attestations:        u64,
        conservation_attestations: u64,
        carbon_attestations:       u64,
    }

    /// A permanently shared on-chain attestation linking a backend record
    /// to its IOTA proof and a verified claim outcome.
    public struct MRVAttestation has key, store {
        id: UID,
        /// Backend UUID of the catch/conservation/carbon record.
        record_id: String,
        /// Record type: "catch" | "conservation" | "carbon_credit"
        record_type: String,
        /// DID of the fisher, monitor, or project the record belongs to.
        actor_did: String,
        /// Object ID of the primary on-chain anchor (ProvenanceAnchor or CatchAnchor).
        anchor_object_id: Option<ID>,
        /// IOTA transaction digest of the anchor transaction (for explorer links).
        anchor_tx_digest: String,
        /// SHA-256 of the canonical record JSON at attestation time.
        /// Tamper evidence: any backend change invalidates this commitment.
        commitment: String,
        /// Verification level 0–3 (see module header).
        verification_level: u8,
        /// DID of the verifier. Empty string for level 0 (self-reported).
        verifier_did: String,
        /// Optional: weight_grams for catch attestations.
        weight_grams: Option<u64>,
        /// Optional: activity type for conservation attestations.
        activity_type: Option<String>,
        /// Unix timestamp (ms) when the attestation was created.
        attested_at_ms: u64,
    }

    // ─── Events ──────────────────────────────────────────────────────────────

    public struct AttestationRegistered has copy, drop {
        attestation_id: ID,
        record_id: String,
        record_type: String,
        actor_did: String,
        verification_level: u8,
        attested_at_ms: u64,
    }

    // ─── Init ─────────────────────────────────────────────────────────────────

    fun init(ctx: &mut TxContext) {
        transfer::share_object(MRVRegistry {
            id: object::new(ctx),
            total_attestations: 0,
            catch_attestations: 0,
            conservation_attestations: 0,
            carbon_attestations: 0,
        });
    }

    // ─── Helpers ──────────────────────────────────────────────────────────────

    fun is_valid_record_type(t: &String): bool {
        *t == string::utf8(b"catch")        ||
        *t == string::utf8(b"conservation") ||
        *t == string::utf8(b"carbon_credit")
    }

    fun increment_type_counter(registry: &mut MRVRegistry, record_type: &String) {
        if (*record_type == string::utf8(b"catch")) {
            registry.catch_attestations = registry.catch_attestations + 1;
        } else if (*record_type == string::utf8(b"conservation")) {
            registry.conservation_attestations = registry.conservation_attestations + 1;
        } else {
            registry.carbon_attestations = registry.carbon_attestations + 1;
        };
        registry.total_attestations = registry.total_attestations + 1;
    }

    // ─── Entry functions ──────────────────────────────────────────────────────

    /// Register a level-1 (manager) attestation.
    ///
    /// Requires ManagerCap. Called when a project manager confirms a conservation
    /// or catch record. For catch records, the manager_did in ManagerCap acts
    /// as the officer/BMU representative.
    public entry fun register_manager_attestation(
        registry:           &mut MRVRegistry,
        manager_cap:        &ManagerCap,
        record_id:          vector<u8>,
        record_type:        vector<u8>,
        actor_did:          vector<u8>,
        anchor_object_id:   Option<ID>,
        anchor_tx_digest:   vector<u8>,
        commitment:         vector<u8>,
        weight_grams:       Option<u64>,
        activity_type:      vector<u8>,
        attested_at_ms:     u64,
        ctx:                &mut TxContext
    ) {
        assert!(!record_id.is_empty(),  EEmptyRecordId);
        assert!(!commitment.is_empty(), EEmptyCommitment);

        let record_id_str   = string::utf8(record_id);
        let record_type_str = string::utf8(record_type);
        assert!(is_valid_record_type(&record_type_str), EInvalidRecordType);
        assert!(
            !dynamic_field::exists_(&registry.id, record_id_str),
            EDuplicateAttestation
        );

        let att_uid       = object::new(ctx);
        let att_id        = object::uid_to_inner(&att_uid);
        let verifier_did  = conservation_chain::roles::manager_did(manager_cap);
        let activity_type_str = string::utf8(activity_type);

        let attestation = MRVAttestation {
            id:                 att_uid,
            record_id:          record_id_str,
            record_type:        record_type_str,
            actor_did:          string::utf8(actor_did),
            anchor_object_id,
            anchor_tx_digest:   string::utf8(anchor_tx_digest),
            commitment:         string::utf8(commitment),
            verification_level: 1,
            verifier_did:       string::utf8(verifier_did),
            weight_grams,
            activity_type: if (activity_type_str != string::utf8(b"")) option::some(activity_type_str) else option::none(),
            attested_at_ms,
        };

        dynamic_field::add(&mut registry.id, record_id_str, att_id);
        increment_type_counter(registry, &attestation.record_type);

        event::emit(AttestationRegistered {
            attestation_id:     att_id,
            record_id:          attestation.record_id,
            record_type:        attestation.record_type,
            actor_did:          attestation.actor_did,
            verification_level: 1,
            attested_at_ms,
        });

        transfer::share_object(attestation);
    }

    /// Register a level-2 (independent verifier) attestation.
    ///
    /// Requires VerifierCap. Called when an NGO or government verifier
    /// approves an achievement claim. Only the verifier DID changes
    /// vs a manager attestation — the verification_level becomes 2.
    public entry fun register_verifier_attestation(
        registry:           &mut MRVRegistry,
        verifier_cap:       &VerifierCap,
        record_id:          vector<u8>,
        record_type:        vector<u8>,
        actor_did:          vector<u8>,
        anchor_object_id:   Option<ID>,
        anchor_tx_digest:   vector<u8>,
        commitment:         vector<u8>,
        weight_grams:       Option<u64>,
        activity_type:      vector<u8>,
        attested_at_ms:     u64,
        ctx:                &mut TxContext
    ) {
        assert!(!record_id.is_empty(),  EEmptyRecordId);
        assert!(!commitment.is_empty(), EEmptyCommitment);

        let record_id_str   = string::utf8(record_id);
        let record_type_str = string::utf8(record_type);
        assert!(is_valid_record_type(&record_type_str), EInvalidRecordType);
        assert!(
            !dynamic_field::exists_(&registry.id, record_id_str),
            EDuplicateAttestation
        );

        let att_uid      = object::new(ctx);
        let att_id       = object::uid_to_inner(&att_uid);
        let verifier_did = conservation_chain::roles::verifier_did(verifier_cap);
        let activity_type_str = string::utf8(activity_type);

        let attestation = MRVAttestation {
            id:                 att_uid,
            record_id:          record_id_str,
            record_type:        record_type_str,
            actor_did:          string::utf8(actor_did),
            anchor_object_id,
            anchor_tx_digest:   string::utf8(anchor_tx_digest),
            commitment:         string::utf8(commitment),
            verification_level: 2,
            verifier_did:       string::utf8(verifier_did),
            weight_grams,
            activity_type: if (activity_type_str != string::utf8(b"")) option::some(activity_type_str) else option::none(),
            attested_at_ms,
        };

        dynamic_field::add(&mut registry.id, record_id_str, att_id);
        increment_type_counter(registry, &attestation.record_type);

        event::emit(AttestationRegistered {
            attestation_id:     att_id,
            record_id:          attestation.record_id,
            record_type:        attestation.record_type,
            actor_did:          attestation.actor_did,
            verification_level: 2,
            attested_at_ms,
        });

        transfer::share_object(attestation);
    }

    /// Register a self-reported (level-0) attestation.
    ///
    /// No cap required. Called by the executor backend on every new anchor
    /// write. Provides on-chain immutability of the record's existence
    /// without third-party verification. Level-1/2 attestations replace
    /// or supplement this once a verifier reviews the record.
    ///
    /// Note: the backend must choose: either register level-0 here OR level-1+
    /// via manager/verifier entry. Both cannot exist for the same record_id
    /// (registry rejects duplicates). Typical flow: register level-0 immediately
    /// on first anchor; let manager/verifier upgrade to 1/2 in a new record
    /// (different record_id suffix, e.g. "<uuid>-verified").
    public entry fun register_self_attestation(
        registry:         &mut MRVRegistry,
        record_id:        vector<u8>,
        record_type:      vector<u8>,
        actor_did:        vector<u8>,
        anchor_object_id: Option<ID>,
        anchor_tx_digest: vector<u8>,
        commitment:       vector<u8>,
        weight_grams:     Option<u64>,
        activity_type:    vector<u8>,
        attested_at_ms:   u64,
        ctx:              &mut TxContext
    ) {
        assert!(!record_id.is_empty(),  EEmptyRecordId);
        assert!(!commitment.is_empty(), EEmptyCommitment);

        let record_id_str   = string::utf8(record_id);
        let record_type_str = string::utf8(record_type);
        assert!(is_valid_record_type(&record_type_str), EInvalidRecordType);
        assert!(
            !dynamic_field::exists_(&registry.id, record_id_str),
            EDuplicateAttestation
        );

        let att_uid = object::new(ctx);
        let att_id  = object::uid_to_inner(&att_uid);
        let activity_type_str = string::utf8(activity_type);

        let attestation = MRVAttestation {
            id:                 att_uid,
            record_id:          record_id_str,
            record_type:        record_type_str,
            actor_did:          string::utf8(actor_did),
            anchor_object_id,
            anchor_tx_digest:   string::utf8(anchor_tx_digest),
            commitment:         string::utf8(commitment),
            verification_level: 0,
            verifier_did:       string::utf8(b""),
            weight_grams,
            activity_type: if (activity_type_str != string::utf8(b"")) option::some(activity_type_str) else option::none(),
            attested_at_ms,
        };

        dynamic_field::add(&mut registry.id, record_id_str, att_id);
        increment_type_counter(registry, &attestation.record_type);

        event::emit(AttestationRegistered {
            attestation_id:     att_id,
            record_id:          attestation.record_id,
            record_type:        attestation.record_type,
            actor_did:          attestation.actor_did,
            verification_level: 0,
            attested_at_ms,
        });

        transfer::share_object(attestation);
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun get_record_id(a: &MRVAttestation): String           { a.record_id }
    public fun get_record_type(a: &MRVAttestation): String         { a.record_type }
    public fun get_actor_did(a: &MRVAttestation): String           { a.actor_did }
    public fun get_anchor_tx_digest(a: &MRVAttestation): String    { a.anchor_tx_digest }
    public fun get_commitment(a: &MRVAttestation): String          { a.commitment }
    public fun get_verification_level(a: &MRVAttestation): u8      { a.verification_level }
    public fun get_verifier_did(a: &MRVAttestation): String        { a.verifier_did }
    public fun get_attested_at_ms(a: &MRVAttestation): u64         { a.attested_at_ms }
    public fun get_total_attestations(r: &MRVRegistry): u64        { r.total_attestations }
    public fun get_catch_attestations(r: &MRVRegistry): u64        { r.catch_attestations }
    public fun get_conservation_attestations(r: &MRVRegistry): u64 { r.conservation_attestations }
    public fun get_carbon_attestations(r: &MRVRegistry): u64       { r.carbon_attestations }

    public fun resolve_attestation(registry: &MRVRegistry, record_id: vector<u8>): ID {
        let record_id_str = string::utf8(record_id);
        *dynamic_field::borrow<String, ID>(&registry.id, record_id_str)
    }

    // ─── Test helpers ─────────────────────────────────────────────────────────

    #[test_only]
    public fun create_registry_for_testing(ctx: &mut TxContext): MRVRegistry {
        MRVRegistry {
            id: object::new(ctx),
            total_attestations: 0,
            catch_attestations: 0,
            conservation_attestations: 0,
            carbon_attestations: 0,
        }
    }

    #[test_only]
    public fun destroy_registry_for_testing(r: MRVRegistry) {
        let MRVRegistry { id, .. } = r;
        object::delete(id);
    }
}
