/// BahariMRV — Conservation Provenance Notary
///
/// Immutably anchors conservation achievement provenance on IOTA Rebased Tangle.
/// Each anchor is a permanently frozen, publicly readable on-chain object.
///
/// What this contract anchors:
///   evidence_submitted    — a particular field evidence bundle existed at a time
///   achieved_unverified   — the project manager attested completion (not yet verified)
///   achievement_verified  — an independent verifier approved the achievement
///
/// The commitment shape exactly mirrors what the Go provenance service computes:
///   event_id     → backend UUID of the ProvenanceEvent (idempotency key)
///   kind         → claim type string
///   commitment   → SHA-256 hex of the canonical claim JSON
///   issuer_did   → DID of the user who signed and submitted the claim
///
/// Previous object linking uses Option<ID> — the executor converts the backend's
/// stored object ID string to a Move-typed ID before calling the entry function.
///
/// Pattern from EcoTrack notary.move; no plastic fields exist here.
module conservation_chain::notary {
    use iota::object::{Self, UID, ID};
    use iota::tx_context::{Self, TxContext};
    use iota::transfer;
    use iota::event;
    use iota::dynamic_field;
    use std::string::{Self, String};
    use std::option::{Self, Option};
    use conservation_chain::roles::{AdminCap, ManagerCap, VerifierCap};

    // ─── Error codes ─────────────────────────────────────────────────────────

    const EEmptyIdempotencyKey: u64 = 1;
    const EEmptyCommitment:     u64 = 2;
    const EEmptyIssuerDID:      u64 = 3;
    const EDuplicateAnchor:     u64 = 4;

    // ─── On-chain record ─────────────────────────────────────────────────────

    /// A permanently immutable on-chain record of one conservation provenance event.
    /// Once frozen, no field can ever change. Corrections create new anchors that
    /// reference this one via previous_object_id — the original is never destroyed.
    public struct ProvenanceAnchor has key {
        id: UID,

        /// Backend UUID (ProvenanceEvent.ID). Used as the idempotency key in the
        /// shared registry so the same event is never anchored twice.
        event_id: String,

        /// Claim type:
        ///   "evidence_submitted"   — field monitor submitted an evidence bundle
        ///   "achieved_unverified"  — PM attested completion; proves the claim existed
        ///   "achievement_verified" — independent verifier approved; the proof is valid
        kind: String,

        /// SHA-256 hex of the canonical claim JSON.
        /// Public verifiers recompute this from the downloaded evidence and compare.
        commitment: String,

        /// DID of the user who signed and submitted this claim.
        ///   evidence_submitted   → field monitor's DID
        ///   achieved_unverified  → project manager's DID
        ///   achievement_verified → verifier's DID
        issuer_did: String,

        /// Object ID of the previous anchor in this achievement's chain.
        /// None for the first anchor. Set when a correction supersedes an earlier event.
        previous_object_id: Option<ID>,

        /// True once a superseding anchor has been created for this event.
        /// The original anchor remains readable; this flag lets verifiers know
        /// a newer version exists.
        superseded: bool,

        /// True if an authorised party has revoked this anchor.
        /// Revocation is additive: the anchor is never deleted.
        revoked: bool,
    }

    // ─── Events ──────────────────────────────────────────────────────────────

    public struct ProvenanceAnchored has copy, drop {
        event_id:    String,
        kind:        String,
        commitment:  String,
        issuer_did:  String,
        object_id:   address,
    }

    public struct AnchorSuperseded has copy, drop {
        old_event_id: String,
        new_event_id: String,
    }

    public struct AnchorRevoked has copy, drop {
        event_id:  String,
        object_id: address,
    }

    // ─── Shared registry ─────────────────────────────────────────────────────

    /// Singleton shared object that maps event_id → anchor object ID.
    /// Prevents duplicate anchors for the same ProvenanceEvent.
    public struct AnchorRegistry has key {
        id:             UID,
        total_anchors:  u64,
    }

    fun init(ctx: &mut TxContext) {
        transfer::share_object(AnchorRegistry {
            id: object::new(ctx),
            total_anchors: 0,
        });
    }

    // ─── Entry points ────────────────────────────────────────────────────────

    /// Anchor a completion claim or evidence bundle.
    /// Requires a ManagerCap (issued by the platform admin to the project manager).
    /// Used for: evidence_submitted, achieved_unverified
    public entry fun anchor(
        registry:           &mut AnchorRegistry,
        _cap:               &ManagerCap,
        event_id:           vector<u8>,
        kind:               vector<u8>,
        commitment:         vector<u8>,
        issuer_did:         vector<u8>,
        previous_object_id: Option<ID>,
        ctx:                &mut TxContext
    ) {
        do_anchor(registry, event_id, kind, commitment, issuer_did, previous_object_id, ctx);
    }

    /// Anchor an independent verification decision.
    /// Requires a VerifierCap (issued by the platform admin to the verifier).
    /// Used for: achievement_verified
    public entry fun anchor_verified(
        registry:           &mut AnchorRegistry,
        _cap:               &VerifierCap,
        event_id:           vector<u8>,
        kind:               vector<u8>,
        commitment:         vector<u8>,
        issuer_did:         vector<u8>,
        previous_object_id: Option<ID>,
        ctx:                &mut TxContext
    ) {
        do_anchor(registry, event_id, kind, commitment, issuer_did, previous_object_id, ctx);
    }

    /// Admin-sponsored anchor — used when the backend submits on behalf of a
    /// user holding no on-chain cap (custodial key model from the Go backend).
    /// The admin cap is held by the executor service's sponsored wallet.
    public entry fun anchor_sponsored(
        registry:           &mut AnchorRegistry,
        _admin:             &AdminCap,
        event_id:           vector<u8>,
        kind:               vector<u8>,
        commitment:         vector<u8>,
        issuer_did:         vector<u8>,
        previous_object_id: Option<ID>,
        ctx:                &mut TxContext
    ) {
        do_anchor(registry, event_id, kind, commitment, issuer_did, previous_object_id, ctx);
    }

    /// Internal implementation — validates, creates, emits, and freezes.
    fun do_anchor(
        registry:           &mut AnchorRegistry,
        event_id:           vector<u8>,
        kind:               vector<u8>,
        commitment:         vector<u8>,
        issuer_did:         vector<u8>,
        previous_object_id: Option<ID>,
        ctx:                &mut TxContext
    ) {
        assert!(!event_id.is_empty(),  EEmptyIdempotencyKey);
        assert!(!commitment.is_empty(), EEmptyCommitment);
        assert!(!issuer_did.is_empty(), EEmptyIssuerDID);

        let event_id_str = string::utf8(event_id);

        // Idempotency: reject if already anchored
        assert!(
            !dynamic_field::exists_(&registry.id, event_id_str),
            EDuplicateAnchor
        );

        let anchor_uid = object::new(ctx);
        let anchor_addr = object::uid_to_address(&anchor_uid);

        let record = ProvenanceAnchor {
            id:                 anchor_uid,
            event_id:           event_id_str,
            kind:               string::utf8(kind),
            commitment:         string::utf8(commitment),
            issuer_did:         string::utf8(issuer_did),
            previous_object_id,
            superseded:         false,
            revoked:            false,
        };

        // Register in shared registry (event_id → anchor object ID)
        let anchor_id = object::uid_to_inner(&record.id);
        dynamic_field::add(&mut registry.id, event_id_str, anchor_id);
        registry.total_anchors = registry.total_anchors + 1;

        event::emit(ProvenanceAnchored {
            event_id:   event_id_str,
            kind:       string::utf8(kind),
            commitment: string::utf8(commitment),
            issuer_did: string::utf8(issuer_did),
            object_id:  anchor_addr,
        });

        // Freeze permanently — no field can ever change after this
        transfer::freeze_object(record);
    }

    // ─── Supersession (corrections are additive) ──────────────────────────────

    /// Emit a supersession event when a new anchor replaces an old one.
    /// The old anchor stays on-chain — this is a notification only.
    /// Verifiers follow the supersession chain to reach the authoritative anchor.
    public entry fun emit_superseded(
        _admin:       &AdminCap,
        old_event_id: vector<u8>,
        new_event_id: vector<u8>,
    ) {
        event::emit(AnchorSuperseded {
            old_event_id: string::utf8(old_event_id),
            new_event_id: string::utf8(new_event_id),
        });
    }

    /// Emit a revocation event. The on-chain anchor is not deleted
    /// (it's frozen), but verifiers can see it's no longer authoritative.
    public entry fun emit_revoked(
        _admin:    &AdminCap,
        event_id:  vector<u8>,
        object_id: address,
    ) {
        event::emit(AnchorRevoked {
            event_id:  string::utf8(event_id),
            object_id,
        });
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun get_event_id(a: &ProvenanceAnchor): String    { a.event_id }
    public fun get_kind(a: &ProvenanceAnchor): String         { a.kind }
    public fun get_commitment(a: &ProvenanceAnchor): String   { a.commitment }
    public fun get_issuer_did(a: &ProvenanceAnchor): String   { a.issuer_did }
    public fun is_superseded(a: &ProvenanceAnchor): bool      { a.superseded }
    public fun is_revoked(a: &ProvenanceAnchor): bool         { a.revoked }
    public fun total_anchors(r: &AnchorRegistry): u64         { r.total_anchors }

    // ─── Test helpers ────────────────────────────────────────────────────────

    #[test_only]
    public fun create_registry_for_testing(ctx: &mut TxContext): AnchorRegistry {
        AnchorRegistry { id: object::new(ctx), total_anchors: 0 }
    }
    #[test_only]
    /// Creates and shares a registry in one call — the share must happen inside
    /// this module because AnchorRegistry does not have 'store'.
    public fun create_and_share_registry_for_testing(ctx: &mut TxContext) {
        transfer::share_object(AnchorRegistry { id: object::new(ctx), total_anchors: 0 });
    }
    #[test_only]
    public fun destroy_registry_for_testing(r: AnchorRegistry) {
        let AnchorRegistry { id, .. } = r;
        object::delete(id);
    }
}
