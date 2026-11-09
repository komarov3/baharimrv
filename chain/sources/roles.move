/// BahariMRV — Conservation Role Capabilities
///
/// Capability objects that gate write access to the conservation chain.
/// Pattern from EcoTrack plastic_chain::roles but with conservation-native roles.
///
/// Lifecycle:
///   Deploy → AdminCap issued to deployer
///   Admin → issue_manager_cap(to, did, project_id) → recipient holds ManagerCap
///   Admin → issue_monitor_cap(to, did, project_id, plot_id) → MonitorCap
///   Admin → issue_verifier_cap(to, did, scope) → VerifierCap
///   Admin → revoke_*_cap(cap) → cap deleted, DID can no longer transact
module conservation_chain::roles {
    use iota::object::{Self, UID};
    use iota::tx_context::{Self, TxContext};
    use iota::transfer;
    use iota::event;

    // ─── Capabilities ────────────────────────────────────────────────────────

    /// Platform admin capability. Issued once at deployment to the deployer.
    /// Required to issue and revoke all other caps.
    public struct AdminCap has key, store { id: UID }

    /// Conservation project manager capability.
    /// Required to anchor completion claims and submit verification requests.
    public struct ManagerCap has key, store {
        id: UID,
        manager_did: vector<u8>,
        project_id:  vector<u8>,
    }

    /// Field monitor capability, scoped to one plot.
    /// Required to submit field monitoring records.
    public struct MonitorCap has key, store {
        id: UID,
        monitor_did: vector<u8>,
        project_id:  vector<u8>,
        plot_id:     vector<u8>,
    }

    /// Independent verifier capability.
    /// Required to anchor approved verification decisions and attestations.
    public struct VerifierCap has key, store {
        id: UID,
        verifier_did: vector<u8>,
        scope:        vector<u8>,   // e.g. b"global" or a project_id
    }

    // ─── Events ──────────────────────────────────────────────────────────────

    public struct CapIssued has copy, drop {
        cap_type:  vector<u8>,
        recipient: address,
        did:       vector<u8>,
    }

    public struct CapRevoked has copy, drop {
        cap_type: vector<u8>,
        did:      vector<u8>,
    }

    // ─── Deployment ──────────────────────────────────────────────────────────

    fun init(ctx: &mut TxContext) {
        transfer::transfer(
            AdminCap { id: object::new(ctx) },
            tx_context::sender(ctx),
        );
    }

    // ─── Issue caps ──────────────────────────────────────────────────────────

    public entry fun issue_manager_cap(
        _admin:     &AdminCap,
        to:         address,
        did:        vector<u8>,
        project_id: vector<u8>,
        ctx:        &mut TxContext
    ) {
        event::emit(CapIssued { cap_type: b"manager", recipient: to, did });
        transfer::transfer(
            ManagerCap { id: object::new(ctx), manager_did: did, project_id },
            to,
        );
    }

    public entry fun issue_monitor_cap(
        _admin:     &AdminCap,
        to:         address,
        did:        vector<u8>,
        project_id: vector<u8>,
        plot_id:    vector<u8>,
        ctx:        &mut TxContext
    ) {
        event::emit(CapIssued { cap_type: b"monitor", recipient: to, did });
        transfer::transfer(
            MonitorCap { id: object::new(ctx), monitor_did: did, project_id, plot_id },
            to,
        );
    }

    public entry fun issue_verifier_cap(
        _admin: &AdminCap,
        to:     address,
        did:    vector<u8>,
        scope:  vector<u8>,
        ctx:    &mut TxContext
    ) {
        event::emit(CapIssued { cap_type: b"verifier", recipient: to, did });
        transfer::transfer(
            VerifierCap { id: object::new(ctx), verifier_did: did, scope },
            to,
        );
    }

    // ─── Revoke caps ─────────────────────────────────────────────────────────

    public entry fun revoke_manager_cap(cap: ManagerCap) {
        let ManagerCap { id, manager_did: did, .. } = cap;
        event::emit(CapRevoked { cap_type: b"manager", did });
        object::delete(id);
    }

    public entry fun revoke_monitor_cap(cap: MonitorCap) {
        let MonitorCap { id, monitor_did: did, .. } = cap;
        event::emit(CapRevoked { cap_type: b"monitor", did });
        object::delete(id);
    }

    public entry fun revoke_verifier_cap(cap: VerifierCap) {
        let VerifierCap { id, verifier_did: did, .. } = cap;
        event::emit(CapRevoked { cap_type: b"verifier", did });
        object::delete(id);
    }

    // ─── Getters ─────────────────────────────────────────────────────────────

    public fun manager_did(cap: &ManagerCap): vector<u8>  { cap.manager_did }
    public fun manager_project(cap: &ManagerCap): vector<u8> { cap.project_id }
    public fun monitor_did(cap: &MonitorCap): vector<u8>  { cap.monitor_did }
    public fun monitor_project(cap: &MonitorCap): vector<u8> { cap.project_id }
    public fun monitor_plot(cap: &MonitorCap): vector<u8>    { cap.plot_id }
    public fun verifier_did(cap: &VerifierCap): vector<u8>   { cap.verifier_did }
    public fun verifier_scope(cap: &VerifierCap): vector<u8> { cap.scope }

    // ─── Test helpers ────────────────────────────────────────────────────────

    #[test_only]
    public fun create_admin_for_testing(ctx: &mut TxContext): AdminCap {
        AdminCap { id: object::new(ctx) }
    }
    #[test_only]
    public fun destroy_admin_for_testing(cap: AdminCap) {
        let AdminCap { id } = cap;
        object::delete(id);
    }
    #[test_only]
    public fun create_manager_for_testing(
        project_id: vector<u8>, ctx: &mut TxContext
    ): ManagerCap {
        ManagerCap {
            id: object::new(ctx),
            manager_did: b"did:iota:testnet:test-manager",
            project_id,
        }
    }
    #[test_only]
    public fun destroy_manager_for_testing(cap: ManagerCap) {
        let ManagerCap { id, .. } = cap;
        object::delete(id);
    }
}
