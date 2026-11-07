/// BahariMRV conservation_chain — Unit Tests
#[test_only]
module conservation_chain::conservation_chain_tests {
    use iota::test_scenario::{Self, next_tx, ctx};
    use iota::clock::{Self};
    use iota::object;
    use conservation_chain::roles::{Self, ManagerCap, VerifierCap};
    use conservation_chain::project_ledger::{Self, ProjectImpactLedger, ContributionCap};
    use conservation_chain::notary::{Self, AnchorRegistry};
    use bahari_identity::identity::{Self, IdentityRegistry};
    use conservation_chain::restoration;

    // ─── Roles ───────────────────────────────────────────────────────────────

    #[test]
    fun test_issue_manager_cap() {
        let admin   = @0xAD;
        let manager = @0xBE;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            roles::issue_manager_cap(&admin_cap, manager,
                b"did:iota:testnet:manager-001", b"project-uuid-001",
                ctx(&mut scenario));
            roles::destroy_admin_for_testing(admin_cap);
        };

        next_tx(&mut scenario, manager);
        {
            assert!(test_scenario::has_most_recent_for_address<ManagerCap>(manager), 0);
            let cap = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            assert!(roles::manager_did(&cap) == b"did:iota:testnet:manager-001", 1);
            assert!(roles::manager_project(&cap) == b"project-uuid-001", 2);
            test_scenario::return_to_address(manager, cap);
        };

        test_scenario::end(scenario);
    }

    #[test]
    fun test_revoke_manager_cap() {
        let admin   = @0xAD;
        let manager = @0xBE;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            roles::issue_manager_cap(&admin_cap, manager,
                b"did:iota:testnet:revoke-test", b"proj-revoke",
                ctx(&mut scenario));
            roles::destroy_admin_for_testing(admin_cap);
        };

        next_tx(&mut scenario, manager);
        {
            let cap = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            roles::revoke_manager_cap(cap);
        };

        next_tx(&mut scenario, manager);
        {
            assert!(!test_scenario::has_most_recent_for_address<ManagerCap>(manager), 0);
        };

        test_scenario::end(scenario);
    }

    #[test]
    fun test_issue_verifier_cap() {
        let admin    = @0xAD;
        let verifier = @0xCA;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            roles::issue_verifier_cap(&admin_cap, verifier,
                b"did:iota:testnet:verifier-001", b"global",
                ctx(&mut scenario));
            roles::destroy_admin_for_testing(admin_cap);
        };

        next_tx(&mut scenario, verifier);
        {
            let cap = test_scenario::take_from_address<VerifierCap>(&scenario, verifier);
            assert!(roles::verifier_did(&cap) == b"did:iota:testnet:verifier-001", 0);
            test_scenario::return_to_address(verifier, cap);
        };

        test_scenario::end(scenario);
    }

    // ─── Notary ──────────────────────────────────────────────────────────────

    #[test]
    fun test_three_claim_types() {
        let admin = @0xAD;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        { notary::create_and_share_registry_for_testing(ctx(&mut scenario)); };

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let mut registry = test_scenario::take_shared<AnchorRegistry>(&scenario);

            notary::anchor_sponsored(&mut registry, &admin_cap,
                b"ev-1", b"evidence_submitted",
                b"aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111",
                b"did:iota:testnet:monitor-001", std::option::none(), ctx(&mut scenario));

            notary::anchor_sponsored(&mut registry, &admin_cap,
                b"ev-2", b"achieved_unverified",
                b"bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222",
                b"did:iota:testnet:manager-001", std::option::none(), ctx(&mut scenario));

            notary::anchor_sponsored(&mut registry, &admin_cap,
                b"ev-3", b"achievement_verified",
                b"cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333",
                b"did:iota:testnet:verifier-001", std::option::none(), ctx(&mut scenario));

            assert!(notary::total_anchors(&registry) == 3, 0);
            test_scenario::return_shared(registry);
            roles::destroy_admin_for_testing(admin_cap);
        };

        test_scenario::end(scenario);
    }

    #[test]
    #[expected_failure(abort_code = conservation_chain::notary::EDuplicateAnchor)]
    fun test_notary_rejects_duplicate() {
        let admin = @0xAD;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        { notary::create_and_share_registry_for_testing(ctx(&mut scenario)); };

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let mut registry = test_scenario::take_shared<AnchorRegistry>(&scenario);

            notary::anchor_sponsored(&mut registry, &admin_cap,
                b"ev-dup", b"evidence_submitted",
                b"aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111",
                b"did:iota:testnet:monitor-001", std::option::none(), ctx(&mut scenario));

            notary::anchor_sponsored(&mut registry, &admin_cap,
                b"ev-dup", b"evidence_submitted",
                b"bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222",
                b"did:iota:testnet:monitor-001", std::option::none(), ctx(&mut scenario));

            test_scenario::return_shared(registry);
            roles::destroy_admin_for_testing(admin_cap);
        };

        test_scenario::end(scenario);
    }

    #[test]
    fun test_anchor_via_manager_cap() {
        let admin   = @0xAD;
        let manager = @0xBE;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        {
            notary::create_and_share_registry_for_testing(ctx(&mut scenario));
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            roles::issue_manager_cap(&admin_cap, manager,
                b"did:iota:testnet:mgr", b"proj-001",
                ctx(&mut scenario));
            roles::destroy_admin_for_testing(admin_cap);
        };

        next_tx(&mut scenario, manager);
        {
            let cap      = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            let mut registry = test_scenario::take_shared<AnchorRegistry>(&scenario);

            notary::anchor(
                &mut registry, &cap,
                b"event-mgr-001", b"achieved_unverified",
                b"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
                b"did:iota:testnet:mgr",
                std::option::none(),
                ctx(&mut scenario),
            );

            assert!(notary::total_anchors(&registry) == 1, 0);
            test_scenario::return_shared(registry);
            test_scenario::return_to_address(manager, cap);
        };

        test_scenario::end(scenario);
    }

    // ─── Identity ────────────────────────────────────────────────────────────

    #[test]
    fun test_provision_multiple_identities() {
        let admin = @0xAD;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        { identity::create_and_share_registry_for_testing(ctx(&mut scenario)); };

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let mut registry = test_scenario::take_shared<IdentityRegistry>(&scenario);

            identity::provision_identity(&mut registry,
                b"manager-001", b"did:iota:testnet:manager-001",
                b"pkMgr001aabb", b"baharimrv", ctx(&mut scenario));

            identity::provision_identity(&mut registry,
                b"monitor-001", b"did:iota:testnet:monitor-001",
                b"pkMon001aabb", b"baharimrv", ctx(&mut scenario));

            identity::provision_identity(&mut registry,
                b"tourist-001", b"did:iota:testnet:tourist-001",
                b"pkTou001aabb", b"jahazi", ctx(&mut scenario));

            assert!(identity::total_identities(&registry) == 3, 0);
            test_scenario::return_shared(registry);
            roles::destroy_admin_for_testing(admin_cap);
        };

        test_scenario::end(scenario);
    }

    #[test]
    #[expected_failure(abort_code = bahari_identity::identity::EDuplicateUserId)]
    fun test_identity_rejects_duplicate() {
        let admin = @0xAD;
        let mut scenario = test_scenario::begin(admin);

        next_tx(&mut scenario, admin);
        { identity::create_and_share_registry_for_testing(ctx(&mut scenario)); };

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let mut registry = test_scenario::take_shared<IdentityRegistry>(&scenario);

            identity::provision_identity(&mut registry,
                b"user-dup", b"did:iota:testnet:user-dup",
                b"pk001", b"baharimrv", ctx(&mut scenario));

            identity::provision_identity(&mut registry,
                b"user-dup", b"did:iota:testnet:user-dup-v2",
                b"pk002", b"baharimrv", ctx(&mut scenario));

            test_scenario::return_shared(registry);
            roles::destroy_admin_for_testing(admin_cap);
        };

        test_scenario::end(scenario);
    }

    // ─── Project Ledger ──────────────────────────────────────────────────────

    #[test]
    fun test_create_and_link_ledger() {
        let admin = @0xAD;
        let mut scenario = test_scenario::begin(admin);
        let clock = clock::create_for_testing(ctx(&mut scenario));

        next_tx(&mut scenario, admin);
        {
            project_ledger::create_and_share_for_testing(
                b"did:iota:testnet:project:mrv-001", b"mrv-uuid-001",
                &clock, ctx(&mut scenario),
            );
        };

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let mut ledger = test_scenario::take_shared<ProjectImpactLedger>(&scenario);

            project_ledger::link_jahazi(&admin_cap, &mut ledger,
                b"did:iota:testnet:project:jahazi-001", &clock);

            // Compare via bytes() helper
            assert!(project_ledger::jahazi_project_did_bytes(&ledger)
                == b"did:iota:testnet:project:jahazi-001", 0);
            assert!(project_ledger::total_planted(&ledger) == 0, 1);

            test_scenario::return_shared(ledger);
            roles::destroy_admin_for_testing(admin_cap);
        };

        clock::destroy_for_testing(clock);
        test_scenario::end(scenario);
    }

    #[test]
    fun test_planting_entries_accumulate() {
        let admin   = @0xAD;
        let manager = @0xBE;
        let mut scenario = test_scenario::begin(admin);
        let clock = clock::create_for_testing(ctx(&mut scenario));

        next_tx(&mut scenario, admin);
        {
            project_ledger::create_and_share_for_testing(
                b"did:iota:testnet:project:mrv-001", b"mrv-001",
                &clock, ctx(&mut scenario),
            );
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            roles::issue_manager_cap(&admin_cap, manager,
                b"did:iota:testnet:manager-001", b"mrv-001", ctx(&mut scenario));
            roles::destroy_admin_for_testing(admin_cap);
        };

        next_tx(&mut scenario, manager);
        {
            let cap    = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            let mut ledger = test_scenario::take_shared<ProjectImpactLedger>(&scenario);

            project_ledger::append_planting_entry(
                &cap, &mut ledger,
                object::id_from_address(@0x111),
                object::id_from_address(@0x222),
                100, 95, 435500,
                &clock, ctx(&mut scenario),
            );

            assert!(project_ledger::total_planted(&ledger) == 100, 0);
            assert!(project_ledger::total_alive(&ledger) == 95, 1);
            assert!(project_ledger::total_co2e_mg(&ledger) == 435500, 2);
            assert!(project_ledger::planting_entry_count(&ledger) == 1, 3);

            test_scenario::return_shared(ledger);
            test_scenario::return_to_address(manager, cap);
        };

        next_tx(&mut scenario, manager);
        {
            let cap    = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            let mut ledger = test_scenario::take_shared<ProjectImpactLedger>(&scenario);

            project_ledger::append_planting_entry(
                &cap, &mut ledger,
                object::id_from_address(@0x333),
                object::id_from_address(@0x444),
                200, 190, 871000,
                &clock, ctx(&mut scenario),
            );

            assert!(project_ledger::total_planted(&ledger) == 300, 0);
            assert!(project_ledger::total_alive(&ledger) == 285, 1);
            assert!(project_ledger::total_co2e_mg(&ledger) == 1306500, 2);
            assert!(project_ledger::planting_entry_count(&ledger) == 2, 3);

            test_scenario::return_shared(ledger);
            test_scenario::return_to_address(manager, cap);
        };

        clock::destroy_for_testing(clock);
        test_scenario::end(scenario);
    }

    #[test]
    fun test_contribution_cap_flow() {
        let admin       = @0xAD;
        let jahazi_exec = @0xEE;
        let mut scenario = test_scenario::begin(admin);
        let clock = clock::create_for_testing(ctx(&mut scenario));

        next_tx(&mut scenario, admin);
        {
            project_ledger::create_and_share_for_testing(
                b"did:iota:testnet:project:mrv-001", b"mrv-001",
                &clock, ctx(&mut scenario),
            );
        };

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let ledger = test_scenario::take_shared<ProjectImpactLedger>(&scenario);
            project_ledger::issue_contribution_cap(
                &admin_cap, jahazi_exec, &ledger,
                b"did:iota:testnet:platform:jahazi", ctx(&mut scenario),
            );
            test_scenario::return_shared(ledger);
            roles::destroy_admin_for_testing(admin_cap);
        };

        next_tx(&mut scenario, jahazi_exec);
        {
            // Take the ContributionCap — use fully qualified type
            let cap = test_scenario::take_from_address<ContributionCap>(&scenario, jahazi_exec);
            let mut ledger = test_scenario::take_shared<ProjectImpactLedger>(&scenario);

            project_ledger::append_contribution_entry(
                &cap, &mut ledger,
                object::id_from_address(@0xCCC),
                4500, 3,
                &clock, ctx(&mut scenario),
            );

            assert!(project_ledger::total_contrib_cents(&ledger) == 4500, 0);
            assert!(project_ledger::total_contributors(&ledger) == 3, 1);
            assert!(project_ledger::contribution_entry_count(&ledger) == 1, 2);

            test_scenario::return_shared(ledger);
            test_scenario::return_to_address(jahazi_exec, cap);
        };

        clock::destroy_for_testing(clock);
        test_scenario::end(scenario);
    }

    #[test]
    #[expected_failure(abort_code = conservation_chain::project_ledger::ELedgerMismatch)]
    fun test_contribution_cap_rejects_wrong_ledger() {
        let admin       = @0xAD;
        let jahazi_exec = @0xEE;
        let mut scenario = test_scenario::begin(admin);
        let clock = clock::create_for_testing(ctx(&mut scenario));

        // Create ledger-1 and issue cap for it
        next_tx(&mut scenario, admin);
        {
            project_ledger::create_and_share_for_testing(
                b"did:iota:testnet:project:mrv-001", b"mrv-001",
                &clock, ctx(&mut scenario),
            );
        };

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let ledger1 = test_scenario::take_shared<ProjectImpactLedger>(&scenario);
            project_ledger::issue_contribution_cap(
                &admin_cap, jahazi_exec, &ledger1,
                b"did:iota:testnet:platform:jahazi", ctx(&mut scenario),
            );
            test_scenario::return_shared(ledger1);
            roles::destroy_admin_for_testing(admin_cap);
        };

        // Create ledger-2
        next_tx(&mut scenario, admin);
        {
            project_ledger::create_and_share_for_testing(
                b"did:iota:testnet:project:mrv-002", b"mrv-002",
                &clock, ctx(&mut scenario),
            );
        };

        // Try using cap (for ledger-1) on ledger-2 — should abort
        next_tx(&mut scenario, jahazi_exec);
        {
            let cap = test_scenario::take_from_address<ContributionCap>(&scenario, jahazi_exec);
            // take_shared always picks the most recently shared object of this type
            let mut ledger2 = test_scenario::take_shared<ProjectImpactLedger>(&scenario);

            project_ledger::append_contribution_entry(
                &cap, &mut ledger2,
                object::id_from_address(@0xDDD),
                1000, 1,
                &clock, ctx(&mut scenario),
            );

            test_scenario::return_shared(ledger2);
            test_scenario::return_to_address(jahazi_exec, cap);
        };

        clock::destroy_for_testing(clock);
        test_scenario::end(scenario);
    }

    // ─── Restoration ─────────────────────────────────────────────────────────

    #[test]
    fun test_anchor_restoration_claim() {
        let admin = @0xAD;
        let mut scenario = test_scenario::begin(admin);
        let clock = clock::create_for_testing(ctx(&mut scenario));

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            restoration::anchor_restoration_claim(
                &admin_cap,
                b"rc_abc123def456",
                b"did:iota:testnet:project:jahazi-001",
                b"did:iota:testnet:project:mrv-001",
                1730000000000, 1730604800000,
                4500, 300,
                b"Rhizophora mucronata",
                3,
                b"aabbccdd1122aabbccdd1122aabbccdd1122aabbccdd1122aabbccdd1122aabb",
                b"eeff99001122eeff99001122eeff99001122eeff99001122eeff99001122eeff",
                b"did:iota:testnet:platform:jahazi",
                b"nonce-unique-001",
                &clock, ctx(&mut scenario),
            );
            roles::destroy_admin_for_testing(admin_cap);
        };

        clock::destroy_for_testing(clock);
        test_scenario::end(scenario);
    }

    #[test]
    fun test_record_fulfillment() {
        let admin   = @0xAD;
        let manager = @0xBE;
        let mut scenario = test_scenario::begin(admin);
        let clock = clock::create_for_testing(ctx(&mut scenario));

        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            roles::issue_manager_cap(&admin_cap, manager,
                b"did:iota:testnet:manager-001", b"mrv-001",
                ctx(&mut scenario));
            roles::destroy_admin_for_testing(admin_cap);
        };

        next_tx(&mut scenario, manager);
        {
            let cap = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            restoration::record_fulfillment(
                &cap,
                object::id_from_address(@0xA11),
                object::id_from_address(@0xA22),
                object::id_from_address(@0xA33),
                300, 300,
                b"did:iota:testnet:project:mrv-001",
                3,
                &clock, ctx(&mut scenario),
            );
            test_scenario::return_to_address(manager, cap);
        };

        clock::destroy_for_testing(clock);
        test_scenario::end(scenario);
    }

    // ─── Full flow ────────────────────────────────────────────────────────────

    #[test]
    fun test_identity_provision_plus_anchor_plus_ledger() {
        let admin   = @0xAD;
        let manager = @0xBE;
        let mut scenario = test_scenario::begin(admin);
        let clock = clock::create_for_testing(ctx(&mut scenario));

        // 1. Setup all registries and ledger
        next_tx(&mut scenario, admin);
        {
            identity::create_and_share_registry_for_testing(ctx(&mut scenario));
            notary::create_and_share_registry_for_testing(ctx(&mut scenario));
            project_ledger::create_and_share_for_testing(
                b"did:iota:testnet:project:mrv-001", b"mrv-001",
                &clock, ctx(&mut scenario),
            );
        };

        // 2. Provision identities + ManagerCap
        next_tx(&mut scenario, admin);
        {
            let admin_cap = roles::create_admin_for_testing(ctx(&mut scenario));
            let mut id_reg = test_scenario::take_shared<IdentityRegistry>(&scenario);

            identity::provision_identity(&mut id_reg,
                b"project-001", b"did:iota:testnet:project:mrv-001",
                b"pkProj001", b"baharimrv", ctx(&mut scenario));

            identity::provision_identity(&mut id_reg,
                b"manager-001", b"did:iota:testnet:manager-001",
                b"pkMgr001", b"baharimrv", ctx(&mut scenario));

            assert!(identity::total_identities(&id_reg) == 2, 0);

            roles::issue_manager_cap(&admin_cap, manager,
                b"did:iota:testnet:manager-001", b"mrv-001",
                ctx(&mut scenario));

            test_scenario::return_shared(id_reg);
            roles::destroy_admin_for_testing(admin_cap);
        };

        // 3. Manager anchors a completion claim
        next_tx(&mut scenario, manager);
        {
            let cap      = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            let mut registry = test_scenario::take_shared<AnchorRegistry>(&scenario);

            notary::anchor(
                &mut registry, &cap,
                b"achievement-001", b"achieved_unverified",
                b"e2e1111e2e1111e2e1111e2e1111e2e1111e2e1111e2e1111e2e1111e2e1111",
                b"did:iota:testnet:manager-001",
                std::option::none(),
                ctx(&mut scenario),
            );

            assert!(notary::total_anchors(&registry) == 1, 0);
            test_scenario::return_shared(registry);
            test_scenario::return_to_address(manager, cap);
        };

        // 4. Append planting entry to ledger
        next_tx(&mut scenario, manager);
        {
            let cap    = test_scenario::take_from_address<ManagerCap>(&scenario, manager);
            let mut ledger = test_scenario::take_shared<ProjectImpactLedger>(&scenario);

            project_ledger::append_planting_entry(
                &cap, &mut ledger,
                object::id_from_address(@0xBBB),
                object::id_from_address(@0xCCC),
                100, 95, 435500,
                &clock, ctx(&mut scenario),
            );

            assert!(project_ledger::total_planted(&ledger) == 100, 0);
            assert!(project_ledger::total_alive(&ledger) == 95, 1);
            assert!(project_ledger::planting_entry_count(&ledger) == 1, 2);

            test_scenario::return_shared(ledger);
            test_scenario::return_to_address(manager, cap);
        };

        clock::destroy_for_testing(clock);
        test_scenario::end(scenario);
    }
}
