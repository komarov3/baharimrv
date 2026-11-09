/// BahariMRV — Project Impact Ledger
///
/// A shared, mutable on-chain accumulator for one conservation project's impact.
/// Both BahariMRV (planting data) and Jahazi (tourist contributions) write into it
/// via capability-gated entry functions.
///
/// The ledger itself holds running totals. Individual entries are separate frozen
/// objects that reference the ledger, keeping the shared object lean and cheap to update.
///
/// Privacy: no PII, no individual tourist amounts. All monetary values in cents.
/// CO₂ in milligrams (× 1000). Area in millionths of hectares (× 1_000_000).
module conservation_chain::project_ledger {
    use iota::object::{Self, UID, ID};
    use iota::tx_context::{Self, TxContext};
    use iota::transfer;
    use iota::event;
    use iota::clock::{Self, Clock};
    use std::string::{Self, String};
    use conservation_chain::roles::{AdminCap, ManagerCap};

    // ─── Error codes ─────────────────────────────────────────────────────────

    const ELedgerMismatch: u64 = 1;   // Cap's ledgerId doesn't match this ledger

    // ─── Shared ledger ────────────────────────────────────────────────────────

    /// Singleton shared object per conservation project.
    /// Explorer-readable real-time impact totals, updated by entry functions.
    public struct ProjectImpactLedger has key {
        id:                       UID,
        projectDID:               String,   // did:iota:testnet:project:<mrvProjectId>
        mrvProjectId:             String,   // BahariMRV backend UUID
        jahaziProjectDID:         String,   // did:iota:testnet:project:<jahaziId> (empty until linked)
        // Running totals
        totalSeedlingsPlanted:    u64,
        totalSeedlingsAlive:      u64,
        totalTouristContribCents: u64,      // USD cents, no floats
        totalContributors:        u64,
        totalVerifiedPlantings:   u64,
        totalCo2eMilligrams:      u64,      // kg × 1000 for precision
        // Entry counters
        plantingEntryCount:       u64,
        contributionEntryCount:   u64,
        lastUpdatedMs:            u64,
    }

    // ─── Capability ───────────────────────────────────────────────────────────

    /// Issued by BahariMRV AdminCap to the Jahazi executor address.
    /// Authorises Jahazi to append ContributionLedgerEntry records to one specific ledger.
    /// Scoped per project — different projects need different caps.
    public struct ContributionCap has key, store {
        id:                UID,
        ledgerId:          ID,     // Which ProjectImpactLedger this cap is for
        jahaziPlatformDID: String, // Jahazi platform DID that holds this cap
    }

    // ─── Frozen entry objects ─────────────────────────────────────────────────

    /// Immutable audit record of one planting batch appended to the ledger.
    /// Created by BahariMRV when a verified planting activity is confirmed on-chain.
    public struct PlantingLedgerEntry has key {
        id:                  UID,
        ledgerId:            ID,
        plantingRecordId:    ID,        // → PlantingRecord
        provenanceAnchorId:  ID,        // → ProvenanceAnchor
        seedlingsAdded:      u64,
        aliveAdded:          u64,
        co2eMilligramsAdded: u64,
        recordedAtMs:        u64,
    }

    /// Immutable audit record of one tourist contribution batch appended to the ledger.
    /// Created by Jahazi executor (via ContributionCap) when certificates are issued.
    public struct ContributionLedgerEntry has key {
        id:               UID,
        ledgerId:         ID,
        claimAnchorId:    ID,   // → RestorationClaimAnchor
        amountCents:      u64,
        contributorCount: u64,
        recordedAtMs:     u64,
    }

    // ─── Events ───────────────────────────────────────────────────────────────

    public struct LedgerCreated has copy, drop {
        ledgerId:    address,
        projectDID:  String,
        mrvProjectId: String,
    }

    public struct JahaziLinked has copy, drop {
        ledgerId:        address,
        jahaziProjectDID: String,
    }

    public struct ContributionCapIssued has copy, drop {
        capId:             address,
        ledgerId:          ID,
        jahaziPlatformDID: String,
        recipient:         address,
    }

    public struct PlantingEntryAppended has copy, drop {
        ledgerId:           ID,
        entryId:            address,
        plantingRecordId:   ID,
        seedlingsAdded:     u64,
        aliveAdded:         u64,
        co2eMilligramsAdded: u64,
        newTotalSeedlings:  u64,
    }

    public struct ContributionEntryAppended has copy, drop {
        ledgerId:           ID,
        entryId:            address,
        claimAnchorId:      ID,
        amountCents:        u64,
        contributorCount:   u64,
        newTotalContribs:   u64,
    }

    // ─── Entry functions ─────────────────────────────────────────────────────

    /// Create and share a ProjectImpactLedger for a conservation project.
    /// Called once per project, by the BahariMRV executor on project creation.
    public entry fun create_ledger(
        _admin:       &AdminCap,
        project_did:  vector<u8>,
        mrv_project_id: vector<u8>,
        clock:        &Clock,
        ctx:          &mut TxContext
    ) {
        let ledger_uid  = object::new(ctx);
        let ledger_addr = object::uid_to_address(&ledger_uid);
        let proj_str    = string::utf8(project_did);

        let ledger = ProjectImpactLedger {
            id:                       ledger_uid,
            projectDID:               proj_str,
            mrvProjectId:             string::utf8(mrv_project_id),
            jahaziProjectDID:         string::utf8(b""),
            totalSeedlingsPlanted:    0,
            totalSeedlingsAlive:      0,
            totalTouristContribCents: 0,
            totalContributors:        0,
            totalVerifiedPlantings:   0,
            totalCo2eMilligrams:      0,
            plantingEntryCount:       0,
            contributionEntryCount:   0,
            lastUpdatedMs:            clock::timestamp_ms(clock),
        };

        event::emit(LedgerCreated {
            ledgerId:     ledger_addr,
            projectDID:   proj_str,
            mrvProjectId: string::utf8(mrv_project_id),
        });

        transfer::share_object(ledger);
    }

    /// Link a Jahazi project DID to this ledger.
    /// Called when the BahariMRV project is linked to a Jahazi project.
    public entry fun link_jahazi(
        _admin:            &AdminCap,
        ledger:            &mut ProjectImpactLedger,
        jahazi_project_did: vector<u8>,
        clock:             &Clock,
    ) {
        let ledger_addr = object::uid_to_address(&ledger.id);
        let jahazi_str  = string::utf8(jahazi_project_did);
        ledger.jahaziProjectDID = jahazi_str;
        ledger.lastUpdatedMs    = clock::timestamp_ms(clock);

        event::emit(JahaziLinked {
            ledgerId:         ledger_addr,
            jahaziProjectDID: jahazi_str,
        });
    }

    /// Issue a ContributionCap to the Jahazi executor address.
    /// Allows Jahazi to call append_contribution_entry on this ledger.
    public entry fun issue_contribution_cap(
        _admin:            &AdminCap,
        to:                address,
        ledger:            &ProjectImpactLedger,
        jahazi_platform_did: vector<u8>,
        ctx:               &mut TxContext
    ) {
        let cap_uid   = object::new(ctx);
        let cap_addr  = object::uid_to_address(&cap_uid);
        let ledger_id = object::uid_to_inner(&ledger.id);
        let did_str   = string::utf8(jahazi_platform_did);

        let cap = ContributionCap {
            id:                cap_uid,
            ledgerId:          ledger_id,
            jahaziPlatformDID: did_str,
        };

        event::emit(ContributionCapIssued {
            capId:             cap_addr,
            ledgerId:          ledger_id,
            jahaziPlatformDID: did_str,
            recipient:         to,
        });

        transfer::transfer(cap, to);
    }

    /// Revoke a ContributionCap. Jahazi can no longer write to this ledger.
    public entry fun revoke_contribution_cap(_admin: &AdminCap, cap: ContributionCap) {
        let ContributionCap { id, .. } = cap;
        object::delete(id);
    }

    /// Append a PlantingLedgerEntry and update running totals.
    /// Called by BahariMRV after a PlantingRecord is created on-chain.
    public entry fun append_planting_entry(
        _cap:               &ManagerCap,
        ledger:             &mut ProjectImpactLedger,
        planting_record_id: ID,
        provenance_anchor_id: ID,
        seedlings_added:    u64,
        alive_added:        u64,
        co2e_mg_added:      u64,
        clock:              &Clock,
        ctx:                &mut TxContext
    ) {
        let entry_uid  = object::new(ctx);
        let entry_addr = object::uid_to_address(&entry_uid);
        let ledger_id  = object::uid_to_inner(&ledger.id);
        let now        = clock::timestamp_ms(clock);

        let entry = PlantingLedgerEntry {
            id:                  entry_uid,
            ledgerId:            ledger_id,
            plantingRecordId:    planting_record_id,
            provenanceAnchorId:  provenance_anchor_id,
            seedlingsAdded:      seedlings_added,
            aliveAdded:          alive_added,
            co2eMilligramsAdded: co2e_mg_added,
            recordedAtMs:        now,
        };

        // Update running totals
        ledger.totalSeedlingsPlanted  = ledger.totalSeedlingsPlanted + seedlings_added;
        ledger.totalSeedlingsAlive    = ledger.totalSeedlingsAlive + alive_added;
        ledger.totalCo2eMilligrams    = ledger.totalCo2eMilligrams + co2e_mg_added;
        ledger.totalVerifiedPlantings = ledger.totalVerifiedPlantings + 1;
        ledger.plantingEntryCount     = ledger.plantingEntryCount + 1;
        ledger.lastUpdatedMs          = now;

        event::emit(PlantingEntryAppended {
            ledgerId:            ledger_id,
            entryId:             entry_addr,
            plantingRecordId:    planting_record_id,
            seedlingsAdded:      seedlings_added,
            aliveAdded:          alive_added,
            co2eMilligramsAdded: co2e_mg_added,
            newTotalSeedlings:   ledger.totalSeedlingsPlanted,
        });

        transfer::freeze_object(entry);
    }

    /// Append a ContributionLedgerEntry and update running totals.
    /// Called by Jahazi executor (via ContributionCap) after certificates are issued.
    public entry fun append_contribution_entry(
        cap:              &ContributionCap,
        ledger:           &mut ProjectImpactLedger,
        claim_anchor_id:  ID,
        amount_cents:     u64,
        contributor_count: u64,
        clock:            &Clock,
        ctx:              &mut TxContext
    ) {
        // Validate cap is scoped to this ledger
        assert!(cap.ledgerId == object::uid_to_inner(&ledger.id), ELedgerMismatch);

        let entry_uid  = object::new(ctx);
        let entry_addr = object::uid_to_address(&entry_uid);
        let ledger_id  = object::uid_to_inner(&ledger.id);
        let now        = clock::timestamp_ms(clock);

        let entry = ContributionLedgerEntry {
            id:               entry_uid,
            ledgerId:         ledger_id,
            claimAnchorId:    claim_anchor_id,
            amountCents:      amount_cents,
            contributorCount: contributor_count,
            recordedAtMs:     now,
        };

        // Update running totals
        ledger.totalTouristContribCents  = ledger.totalTouristContribCents + amount_cents;
        ledger.totalContributors         = ledger.totalContributors + contributor_count;
        ledger.contributionEntryCount    = ledger.contributionEntryCount + 1;
        ledger.lastUpdatedMs             = now;

        event::emit(ContributionEntryAppended {
            ledgerId:         ledger_id,
            entryId:          entry_addr,
            claimAnchorId:    claim_anchor_id,
            amountCents:      amount_cents,
            contributorCount: contributor_count,
            newTotalContribs: ledger.totalTouristContribCents,
        });

        transfer::freeze_object(entry);
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun project_did(l: &ProjectImpactLedger): String          { l.projectDID }
    public fun jahazi_project_did(l: &ProjectImpactLedger): String    { l.jahaziProjectDID }
    // Test-friendly byte version
    public fun jahazi_project_did_bytes(l: &ProjectImpactLedger): vector<u8> { *l.jahaziProjectDID.bytes() }
    public fun total_planted(l: &ProjectImpactLedger): u64            { l.totalSeedlingsPlanted }
    public fun total_alive(l: &ProjectImpactLedger): u64              { l.totalSeedlingsAlive }
    public fun total_contrib_cents(l: &ProjectImpactLedger): u64      { l.totalTouristContribCents }
    public fun total_contributors(l: &ProjectImpactLedger): u64       { l.totalContributors }
    public fun total_co2e_mg(l: &ProjectImpactLedger): u64            { l.totalCo2eMilligrams }
    public fun planting_entry_count(l: &ProjectImpactLedger): u64     { l.plantingEntryCount }
    public fun contribution_entry_count(l: &ProjectImpactLedger): u64 { l.contributionEntryCount }
    public fun cap_ledger_id(c: &ContributionCap): ID                 { c.ledgerId }
    public fun cap_jahazi_did(c: &ContributionCap): String            { c.jahaziPlatformDID }

    // ─── Test helpers ────────────────────────────────────────────────────────

    #[test_only]
    public fun create_and_share_for_testing(
        project_did: vector<u8>,
        mrv_project_id: vector<u8>,
        clock: &Clock,
        ctx: &mut TxContext
    ) {
        let ledger_uid = object::new(ctx);
        let ledger = ProjectImpactLedger {
            id:                       ledger_uid,
            projectDID:               string::utf8(project_did),
            mrvProjectId:             string::utf8(mrv_project_id),
            jahaziProjectDID:         string::utf8(b""),
            totalSeedlingsPlanted:    0,
            totalSeedlingsAlive:      0,
            totalTouristContribCents: 0,
            totalContributors:        0,
            totalVerifiedPlantings:   0,
            totalCo2eMilligrams:      0,
            plantingEntryCount:       0,
            contributionEntryCount:   0,
            lastUpdatedMs:            clock::timestamp_ms(clock),
        };
        transfer::share_object(ledger);
    }
}
