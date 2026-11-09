/// BahariMRV — Carbon Credit Token
///
/// Mintable, transferable, and retirable on-chain carbon credit tokens for
/// the BahariMRV fisheries and conservation MRV programme.
///
/// Each CarbonCredit represents a verified quantity of CO₂-equivalent
/// sequestration or avoided emissions, denominated in kg_co2e (integer,
/// avoiding floating point). 1 tonne = 1000 kg_co2e.
///
/// Credits are generated from:
///   "mangrove_protection"    — area-based sequestration
///   "gear_exchange"          — avoided ALDFG ghost-fishing emissions
///   "seagrass_restoration"   — blue carbon sequestration
///   "marine_protected_area"  — avoided fishing pressure emissions
///   "coral_restoration"      — avoided bleaching / CO₂ credit (verifier-grade)
///   "mangrove_planting"      — new-planting sequestration (links to PlantingRecord)
///
/// Lifecycle:
///   AdminCap → issue_credit(recipient, ...) → fisher/project earner holds CarbonCredit
///   Current holder → transfer_credit(new_holder)
///   Current holder → retire_credit(reason) → credit frozen, double-counting prevented
///
/// Anti-double-mint: CarbonCreditRegistry maps conservation_record_id → credit object ID.
/// One credit per source record — the registry rejects duplicates.
///
/// This module is part of conservation_chain. The backend calls it via the
/// BahariMRV executor service (executor/src/index.ts) — never directly.
module conservation_chain::carbon_credit {
    use iota::object::{Self, UID, ID};
    use iota::tx_context::{Self, TxContext};
    use iota::transfer;
    use iota::event;
    use iota::clock::{Self, Clock};
    use iota::dynamic_field;
    use std::string::{Self, String};
    use std::option::{Self, Option};
    use conservation_chain::roles::AdminCap;

    // ─── Error codes ─────────────────────────────────────────────────────────
    const EZeroAmount:          u64 = 1;
    const EEmptyConservationId: u64 = 2;
    const EAlreadyRetired:      u64 = 3;
    const EDuplicateCredit:     u64 = 4;
    const EEmptyActivityType:   u64 = 5;

    // ─── Structs ─────────────────────────────────────────────────────────────

    /// Shared singleton registry — maps conservation_record_id → credit object ID.
    /// Prevents minting two credits from the same verified conservation activity.
    public struct CarbonCreditRegistry has key {
        id: UID,
        /// Total credits ever minted, in kg_co2e.
        total_issued_kg: u64,
        /// Total credits retired (claimed), in kg_co2e.
        total_retired_kg: u64,
        /// Total number of credit objects minted.
        total_credits: u64,
        /// Number of credits in RETIRED state.
        retired_credits: u64,
    }

    /// A transferable carbon credit token owned by the earner (fisher or project).
    ///
    /// Origin fields are set at mint and never change.
    /// current_holder tracks custody as credits are sold or retired.
    public struct CarbonCredit has key, store {
        id: UID,
        /// Backend UUID of the conservation activity that generated this credit.
        conservation_record_id: String,
        /// Activity type: "mangrove_protection", "gear_exchange", etc.
        activity_type: String,
        /// DID of the fisher or conservation project that earned this credit.
        earner_did: String,
        /// IOTA address of the original earner (immutable).
        origin_earner: address,
        /// IOTA address of the current holder (updated on each transfer).
        current_holder: address,
        /// Carbon amount in kg CO₂-equivalent (integer; 1000 = 1 tonne).
        kg_co2e: u64,
        /// Vintage year — year the underlying activity was verified.
        vintage_year: u64,
        /// Optional object ID of the linked MRV provenance anchor on-chain.
        /// Links this credit to the immutable provenance evidence.
        provenance_anchor_id: Option<ID>,
        /// IOTA transaction digest of the provenance anchoring transaction.
        provenance_tx_digest: String,
        /// Methodology / standard under which the credit was issued.
        /// e.g. "VCS-VM0033", "Gold Standard Fisheries", "BahariMRV-v1"
        standard: String,
        /// Unix timestamp (ms) when the credit was minted.
        issued_at_ms: u64,
        /// Lifecycle status: "ACTIVE" or "RETIRED".
        status: String,
    }

    // ─── Events ──────────────────────────────────────────────────────────────

    public struct CarbonCreditIssued has copy, drop {
        credit_id: ID,
        conservation_record_id: String,
        activity_type: String,
        earner_did: String,
        kg_co2e: u64,
        vintage_year: u64,
    }

    public struct CarbonCreditTransferred has copy, drop {
        credit_id: ID,
        from: address,
        to: address,
        kg_co2e: u64,
    }

    public struct CarbonCreditRetired has copy, drop {
        credit_id: ID,
        conservation_record_id: String,
        retired_by: address,
        kg_co2e: u64,
        reason: String,
    }

    // ─── Init ─────────────────────────────────────────────────────────────────

    fun init(ctx: &mut TxContext) {
        transfer::share_object(CarbonCreditRegistry {
            id: object::new(ctx),
            total_issued_kg: 0,
            total_retired_kg: 0,
            total_credits: 0,
            retired_credits: 0,
        });
    }

    // ─── Entry functions ──────────────────────────────────────────────────────

    /// Issue a carbon credit for a verified conservation activity.
    ///
    /// Requires AdminCap — only the BahariMRV platform admin can mint credits,
    /// ensuring every credit maps 1:1 to a verified MRV registry record.
    /// Rejects duplicate conservation_record_id via CarbonCreditRegistry.
    /// The credit is transferred to `recipient` (the earner's IOTA address).
    ///
    /// provenance_anchor_id: pass option::none() if anchoring asynchronously.
    public entry fun issue_credit(
        registry:               &mut CarbonCreditRegistry,
        admin:                  &AdminCap,
        recipient:              address,
        conservation_record_id: vector<u8>,
        activity_type:          vector<u8>,
        earner_did:             vector<u8>,
        kg_co2e:                u64,
        vintage_year:           u64,
        provenance_anchor_id:   Option<ID>,
        provenance_tx_digest:   vector<u8>,
        standard:               vector<u8>,
        clock:                  &Clock,
        ctx:                    &mut TxContext
    ) {
        assert!(kg_co2e > 0,                        EZeroAmount);
        assert!(!conservation_record_id.is_empty(), EEmptyConservationId);
        assert!(!activity_type.is_empty(),          EEmptyActivityType);

        let _ = admin; // AdminCap gate: presence of the cap is sufficient

        let conservation_id_str = string::utf8(conservation_record_id);
        assert!(
            !dynamic_field::exists_(&registry.id, conservation_id_str),
            EDuplicateCredit
        );

        let credit_uid    = object::new(ctx);
        let credit_id     = object::uid_to_inner(&credit_uid);
        let earner_did_str = string::utf8(earner_did);

        let credit = CarbonCredit {
            id: credit_uid,
            conservation_record_id: conservation_id_str,
            activity_type:          string::utf8(activity_type),
            earner_did:             earner_did_str,
            origin_earner:          recipient,
            current_holder:         recipient,
            kg_co2e,
            vintage_year,
            provenance_anchor_id,
            provenance_tx_digest:   string::utf8(provenance_tx_digest),
            standard:               string::utf8(standard),
            issued_at_ms:           clock::timestamp_ms(clock),
            status:                 string::utf8(b"ACTIVE"),
        };

        dynamic_field::add(&mut registry.id, conservation_id_str, credit_id);
        registry.total_issued_kg = registry.total_issued_kg + kg_co2e;
        registry.total_credits   = registry.total_credits + 1;

        event::emit(CarbonCreditIssued {
            credit_id,
            conservation_record_id: credit.conservation_record_id,
            activity_type:          credit.activity_type,
            earner_did:             credit.earner_did,
            kg_co2e,
            vintage_year,
        });

        transfer::transfer(credit, recipient);
    }

    /// Transfer a carbon credit to a buyer.
    ///
    /// Called by the current holder — fisher sells to a buyer,
    /// or buyer resells on a secondary market.
    public entry fun transfer_credit(
        credit:     CarbonCredit,
        new_holder: address,
        _ctx:       &mut TxContext
    ) {
        assert!(credit.status == string::utf8(b"ACTIVE"), EAlreadyRetired);

        let credit_id = object::uid_to_inner(&credit.id);
        let from      = credit.current_holder;
        let kg        = credit.kg_co2e;

        let mut updated        = credit;
        updated.current_holder = new_holder;

        event::emit(CarbonCreditTransferred {
            credit_id,
            from,
            to: new_holder,
            kg_co2e: kg,
        });

        transfer::transfer(updated, new_holder);
    }

    /// Retire (burn) a carbon credit to claim its offset.
    ///
    /// Called by the current holder. Once retired the credit is frozen —
    /// it cannot be transferred again, preventing double-counting.
    /// reason: short string, e.g. "corporate_offset_2026Q4", "personal_offset"
    public entry fun retire_credit(
        registry:   &mut CarbonCreditRegistry,
        credit:     CarbonCredit,
        reason:     vector<u8>,
        ctx:        &mut TxContext
    ) {
        assert!(credit.status == string::utf8(b"ACTIVE"), EAlreadyRetired);

        let credit_id              = object::uid_to_inner(&credit.id);
        let conservation_record_id = credit.conservation_record_id;
        let retired_by             = tx_context::sender(ctx);
        let kg                     = credit.kg_co2e;

        let mut retiring = credit;
        retiring.status  = string::utf8(b"RETIRED");

        registry.total_retired_kg = registry.total_retired_kg + kg;
        registry.retired_credits  = registry.retired_credits + 1;

        event::emit(CarbonCreditRetired {
            credit_id,
            conservation_record_id,
            retired_by,
            kg_co2e: kg,
            reason: string::utf8(reason),
        });

        transfer::freeze_object(retiring);
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun get_conservation_id(c: &CarbonCredit): String  { c.conservation_record_id }
    public fun get_activity_type(c: &CarbonCredit): String    { c.activity_type }
    public fun get_earner_did(c: &CarbonCredit): String       { c.earner_did }
    public fun get_kg_co2e(c: &CarbonCredit): u64             { c.kg_co2e }
    public fun get_vintage_year(c: &CarbonCredit): u64        { c.vintage_year }
    public fun get_standard(c: &CarbonCredit): String         { c.standard }
    public fun get_current_holder(c: &CarbonCredit): address  { c.current_holder }
    public fun get_origin_earner(c: &CarbonCredit): address   { c.origin_earner }
    public fun is_active(c: &CarbonCredit): bool {
        c.status == string::utf8(b"ACTIVE")
    }
    public fun get_total_issued_kg(r: &CarbonCreditRegistry): u64  { r.total_issued_kg }
    public fun get_total_retired_kg(r: &CarbonCreditRegistry): u64 { r.total_retired_kg }
    public fun get_total_credits(r: &CarbonCreditRegistry): u64    { r.total_credits }
    public fun get_retired_credits(r: &CarbonCreditRegistry): u64  { r.retired_credits }

    public fun resolve_credit(
        registry: &CarbonCreditRegistry,
        conservation_record_id: vector<u8>
    ): ID {
        let id_str = string::utf8(conservation_record_id);
        *dynamic_field::borrow<String, ID>(&registry.id, id_str)
    }

    // ─── Test helpers ─────────────────────────────────────────────────────────

    #[test_only]
    public fun create_registry_for_testing(ctx: &mut TxContext): CarbonCreditRegistry {
        CarbonCreditRegistry {
            id: object::new(ctx),
            total_issued_kg: 0,
            total_retired_kg: 0,
            total_credits: 0,
            retired_credits: 0,
        }
    }

    #[test_only]
    public fun destroy_registry_for_testing(r: CarbonCreditRegistry) {
        let CarbonCreditRegistry { id, .. } = r;
        object::delete(id);
    }
}
