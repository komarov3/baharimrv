package models

func All() []any {
	return []any{
		&User{}, &IdentityProvision{}, &KeyReference{}, &LocalSession{},
		&ExternalIdentity{}, &FederatedGrant{}, &OIDCClient{}, &OAuthGrant{}, &AuthorizationCode{}, &AccessToken{},
		&Methodology{}, &ConservationProject{}, &Site{}, &Plot{}, &ConservationActivity{},
		&MonitoringRecord{}, &EvidenceObject{}, &SatelliteObservation{}, &VerificationRequest{},
		&VerifierAttestation{}, &ProvenanceEvent{}, &AnchorAttempt{}, &Correction{}, &Dispute{},
		&SyncAction{}, &AuditEvent{}, &ProofDocument{}, &CreditBatch{}, &MarketplaceListing{},
		&Purchase{}, &CustodyEvent{}, &RevenueAllocation{}, &Retirement{}, &ReconciliationCase{},
		&SatellitePass{}, &FinancialImport{}, &ProjectDocument{}, &StepUpToken{},
		&SystemConfig{}, &Alert{}, &Incident{}, &AdminNote{}, &ExportJob{}, &SavedView{},
		&VerificationOverride{}, &PublicProofVerificationLog{},
		&RestorationRequest{},
	}
}
