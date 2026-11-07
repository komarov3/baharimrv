package chain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// IOTA targets IOTA Rebased testnet. Transaction construction and custodial
// signing are delegated to an explicitly configured execution service; this
// client never claims success from an HTTP submission alone.
type IOTA struct {
	rpcURL      string
	executorURL string
	packageID   string
	httpClient  *http.Client
}

func NewIOTA(rpcURL, executorURL, packageID string) *IOTA {
	return &IOTA{
		rpcURL:      strings.TrimRight(rpcURL, "/"),
		executorURL: strings.TrimRight(executorURL, "/"),
		packageID:   packageID,
		httpClient:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (i *IOTA) Mode() string    { return "iota" }
func (i *IOTA) Network() string { return "testnet" }

func (i *IOTA) ProvisionIdentity(ctx context.Context, userID string, publicKey []byte) (IdentityResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return IdentityResult{}, fmt.Errorf("%w: identity executor or package is not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":       "provision_identity",
		"package_id":      i.packageID,
		"idempotency_key": userID,
		"public_key":      hex.EncodeToString(publicKey),
		"network":         "testnet",
	}
	var response struct {
		DID            string `json:"did"`
		DIDDocumentRef string `json:"did_document_reference"`
		Address        string `json:"address"`
		TransactionRef string `json:"transaction_reference"`
	}
	if err := i.execute(ctx, payload, &response); err != nil {
		return IdentityResult{}, err
	}
	if response.DID == "" || response.DIDDocumentRef == "" || response.Address == "" || response.TransactionRef == "" {
		return IdentityResult{}, fmt.Errorf("identity executor returned an incomplete result")
	}
	valid, err := i.ResolveIdentity(ctx, response.DIDDocumentRef)
	if err != nil || !valid {
		return IdentityResult{}, fmt.Errorf("identity document readback did not confirm provisioning")
	}
	return IdentityResult{response.DID, response.DIDDocumentRef, response.Address, response.TransactionRef, "testnet"}, nil
}

func (i *IOTA) Anchor(ctx context.Context, request AnchorRequest) (AnchorRecord, error) {
	if i.executorURL == "" || i.packageID == "" {
		return AnchorRecord{}, fmt.Errorf("%w: anchor executor or package is not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":          "anchor",
		"package_id":         i.packageID,
		"idempotency_key":    request.IdempotencyKey,
		"kind":               request.Kind,
		"commitment":         request.Commitment,
		"canonical_json":     request.CanonicalJSON,
		"issuer_did":         request.IssuerDID,
		"previous_object_id": request.PreviousObjectID,
		"network":            "testnet",
	}
	var response AnchorRecord
	if err := i.execute(ctx, payload, &response); err != nil {
		return AnchorRecord{}, err
	}
	if response.ObjectID == "" || response.TransactionRef == "" {
		return AnchorRecord{}, fmt.Errorf("anchor executor returned an incomplete result")
	}
	response.Network = "testnet"
	return response, nil
}

func (i *IOTA) ReadAnchor(ctx context.Context, objectID string) (AnchorRecord, error) {
	fields, err := i.objectFields(ctx, objectID)
	if err != nil {
		return AnchorRecord{}, err
	}
	record := AnchorRecord{
		ObjectID:       objectID,
		Commitment:     fieldString(fields, "commitment"),
		IssuerDID:      fieldString(fields, "issuer_did"),
		Status:         fieldString(fields, "status"),
		SupersededByID: fieldString(fields, "superseded_by_id"),
		Revoked:        fieldBool(fields, "revoked"),
		Network:        "testnet",
	}
	if record.Commitment == "" {
		if raw, ok := fields["commitment"].([]any); ok {
			bytesValue := make([]byte, 0, len(raw))
			for _, item := range raw {
				number, ok := item.(float64)
				if !ok || number < 0 || number > 255 {
					return AnchorRecord{}, fmt.Errorf("invalid chain commitment encoding")
				}
				bytesValue = append(bytesValue, byte(number))
			}
			record.Commitment = hex.EncodeToString(bytesValue)
		}
	}
	if record.Commitment == "" || record.IssuerDID == "" {
		return AnchorRecord{}, fmt.Errorf("chain object is not a BahariMRV anchor")
	}
	return record, nil
}

func (i *IOTA) ResolveIdentity(ctx context.Context, didDocumentRef string) (bool, error) {
	fields, err := i.objectFields(ctx, didDocumentRef)
	if err != nil {
		return false, err
	}
	if revoked := fieldBool(fields, "revoked"); revoked {
		return false, nil
	}
	return fieldString(fields, "id") != "" || fieldString(fields, "did") != "" || len(fields) > 0, nil
}

func (i *IOTA) execute(ctx context.Context, payload any, output any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, i.executorURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := i.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("execute testnet transaction: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
		return fmt.Errorf("testnet executor returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(output); err != nil {
		return fmt.Errorf("decode testnet executor response: %w", err)
	}
	return nil
}

func (i *IOTA) objectFields(ctx context.Context, objectID string) (map[string]any, error) {
	if objectID == "" {
		return nil, ErrNotFound
	}
	payload := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "iota_getObject",
		"params": []any{objectID, map[string]any{"showContent": true}},
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, i.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := i.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: testnet readback failed", ErrUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: testnet readback HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var rpc struct {
		Result struct {
			Data struct {
				Content struct {
					Fields map[string]any `json:"fields"`
				} `json:"content"`
			} `json:"data"`
			Error *struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4*1024*1024)).Decode(&rpc); err != nil {
		return nil, fmt.Errorf("decode testnet readback: %w", err)
	}
	if rpc.Error != nil || rpc.Result.Error != nil || rpc.Result.Data.Content.Fields == nil {
		return nil, ErrNotFound
	}
	fields := rpc.Result.Data.Content.Fields
	if nested, ok := fields["fields"].(map[string]any); ok {
		return nested, nil
	}
	return fields, nil
}

func fieldString(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func fieldBool(fields map[string]any, key string) bool {
	value, _ := fields[key].(bool)
	return value
}

// ── Extended conservation_chain operations ────────────────────────────────────

// LedgerRequest carries the data for creating a ProjectImpactLedger.
type LedgerRequest struct {
	ProjectDID    string
	MRVProjectID  string
}

// LedgerResult is the shared object and transaction for a created ledger.
type LedgerResult struct {
	ObjectID      string `json:"objectId"`
	TransactionRef string `json:"transaction_reference"`
}

// CreateLedger creates and shares a ProjectImpactLedger for a conservation project.
func (i *IOTA) CreateLedger(ctx context.Context, req LedgerRequest) (LedgerResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return LedgerResult{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":       "create_ledger",
		"package_id":      i.packageID,
		"project_did":     req.ProjectDID,
		"mrv_project_id":  req.MRVProjectID,
	}
	var res LedgerResult
	if err := i.execute(ctx, payload, &res); err != nil {
		return LedgerResult{}, err
	}
	return res, nil
}

// LinkJahaziRequest carries data for linking a Jahazi project to the ledger.
type LinkJahaziRequest struct {
	LedgerObjectID   string
	JahaziProjectDID string
}

// LinkJahazi sets the Jahazi project DID on an existing ProjectImpactLedger.
func (i *IOTA) LinkJahazi(ctx context.Context, req LinkJahaziRequest) (LedgerResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return LedgerResult{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":          "link_jahazi",
		"package_id":         i.packageID,
		"ledger_object_id":   req.LedgerObjectID,
		"jahazi_project_did": req.JahaziProjectDID,
	}
	var res LedgerResult
	if err := i.execute(ctx, payload, &res); err != nil {
		return LedgerResult{}, err
	}
	return res, nil
}

// IssueContributionCapRequest carries data for issuing a ContributionCap to Jahazi.
type IssueContributionCapRequest struct {
	Recipient          string
	LedgerObjectID     string
	JahaziPlatformDID  string
}

// IssueContributionCap issues a ContributionCap to the Jahazi executor address.
func (i *IOTA) IssueContributionCap(ctx context.Context, req IssueContributionCapRequest) (LedgerResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return LedgerResult{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":            "issue_contribution_cap",
		"package_id":           i.packageID,
		"recipient":            req.Recipient,
		"ledger_object_id":     req.LedgerObjectID,
		"jahazi_platform_did":  req.JahaziPlatformDID,
	}
	var res LedgerResult
	if err := i.execute(ctx, payload, &res); err != nil {
		return LedgerResult{}, err
	}
	return res, nil
}

// RestorationClaimRequest carries the Jahazi claim data for on-chain anchoring.
type RestorationClaimRequest struct {
	JahaziClaimID          string
	JahaziProjectDID       string
	MRVProjectDID          string
	PeriodStartMs          int64
	PeriodEndMs            int64
	TotalCostCents         int64
	TotalUnits             int64
	Species                string
	ContributorCount       int64
	ContributorMerkleRoot  string
	LocationHash           string
	JahaziPlatformDID      string
	Nonce                  string
}

// AnchorRestorationClaim freezes a RestorationClaimAnchor on-chain.
func (i *IOTA) AnchorRestorationClaim(ctx context.Context, req RestorationClaimRequest) (AnchorRecord, error) {
	if i.executorURL == "" || i.packageID == "" {
		return AnchorRecord{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":                "anchor_restoration_claim",
		"package_id":               i.packageID,
		"jahazi_claim_id":          req.JahaziClaimID,
		"jahazi_project_did":       req.JahaziProjectDID,
		"mrv_project_did":          req.MRVProjectDID,
		"period_start_ms":          req.PeriodStartMs,
		"period_end_ms":            req.PeriodEndMs,
		"total_cost_cents":         req.TotalCostCents,
		"total_units":              req.TotalUnits,
		"species":                  req.Species,
		"contributor_count":        req.ContributorCount,
		"contributor_merkle_root":  req.ContributorMerkleRoot,
		"location_hash":            req.LocationHash,
		"jahazi_platform_did":      req.JahaziPlatformDID,
		"nonce":                    req.Nonce,
	}
	var res AnchorRecord
	if err := i.execute(ctx, payload, &res); err != nil {
		return AnchorRecord{}, err
	}
	res.Network = "testnet"
	return res, nil
}

// PlantingRecordRequest carries field data for creating a PlantingRecord on-chain.
type PlantingRecordRequest struct {
	ProjectDID               string
	LedgerObjectID           string
	ProvenanceAnchorID       string
	ClaimAnchorID            string // empty if not Jahazi-funded
	Species                  string
	SeedlingsPlanted         int64
	SeedlingsAlive           int64
	SurvivalPctHundredths    int64
	AreaHaMillionths         int64
	MethodologyVersion       string
	ReportingPeriodStartMs   int64
	ReportingPeriodEndMs     int64
	EvidenceRoot             string
	SatelliteCorrobHash      string
	PhotoHash                string
	LocationHash             string
	VerifierDID              string
	VerifierAttestationHash  string
	IsVerified               bool
	PreviousRecordID         string // empty for first record
	MRVActivityID            string
}

// CreatePlantingRecord freezes a PlantingRecord on-chain after provenance is confirmed.
func (i *IOTA) CreatePlantingRecord(ctx context.Context, req PlantingRecordRequest) (AnchorRecord, error) {
	if i.executorURL == "" || i.packageID == "" {
		return AnchorRecord{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":                   "planting_record",
		"package_id":                  i.packageID,
		"project_did":                 req.ProjectDID,
		"ledger_object_id":            req.LedgerObjectID,
		"provenance_anchor_id":        req.ProvenanceAnchorID,
		"claim_anchor_id":             req.ClaimAnchorID,
		"species":                     req.Species,
		"seedlings_planted":           req.SeedlingsPlanted,
		"seedlings_alive":             req.SeedlingsAlive,
		"survival_pct_hundredths":     req.SurvivalPctHundredths,
		"area_ha_millionths":          req.AreaHaMillionths,
		"methodology_version":         req.MethodologyVersion,
		"period_start_ms":             req.ReportingPeriodStartMs,
		"period_end_ms":               req.ReportingPeriodEndMs,
		"evidence_root":               req.EvidenceRoot,
		"satellite_corrob_hash":       req.SatelliteCorrobHash,
		"photo_hash":                  req.PhotoHash,
		"location_hash":               req.LocationHash,
		"verifier_did":                req.VerifierDID,
		"verifier_attestation_hash":   req.VerifierAttestationHash,
		"is_verified":                 req.IsVerified,
		"previous_record_id":          req.PreviousRecordID,
		"mrv_activity_id":             req.MRVActivityID,
	}
	var res AnchorRecord
	if err := i.execute(ctx, payload, &res); err != nil {
		return AnchorRecord{}, err
	}
	res.Network = "testnet"
	return res, nil
}

// PlantingEntryRequest carries data for appending a PlantingLedgerEntry.
type PlantingEntryRequest struct {
	LedgerObjectID     string
	PlantingRecordID   string
	ProvenanceAnchorID string
	SeedlingsAdded     int64
	AliveAdded         int64
	Co2eMilligramsAdded int64
}

// AppendPlantingEntry adds a PlantingLedgerEntry to the ProjectImpactLedger.
func (i *IOTA) AppendPlantingEntry(ctx context.Context, req PlantingEntryRequest) (LedgerResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return LedgerResult{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":             "planting_entry",
		"package_id":            i.packageID,
		"ledger_object_id":      req.LedgerObjectID,
		"planting_record_id":    req.PlantingRecordID,
		"provenance_anchor_id":  req.ProvenanceAnchorID,
		"seedlings_added":       req.SeedlingsAdded,
		"alive_added":           req.AliveAdded,
		"co2e_mg_added":         req.Co2eMilligramsAdded,
	}
	var res LedgerResult
	if err := i.execute(ctx, payload, &res); err != nil {
		return LedgerResult{}, err
	}
	return res, nil
}

// FulfillmentRequest carries data for creating a RestorationFulfillmentRecord.
type FulfillmentRequest struct {
	ClaimAnchorID           string
	PlantingRecordID        string
	ProvenanceAnchorID      string
	ActualUnitsPlanted      int64
	ClaimedUnits            int64
	MRVProjectDID           string
	CertificatesIssuedCount int64
}

// RecordFulfillment creates a RestorationFulfillmentRecord linking claim to planting.
func (i *IOTA) RecordFulfillment(ctx context.Context, req FulfillmentRequest) (AnchorRecord, error) {
	if i.executorURL == "" || i.packageID == "" {
		return AnchorRecord{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":                   "claim_fulfillment",
		"package_id":                  i.packageID,
		"claim_anchor_id":             req.ClaimAnchorID,
		"planting_record_id":          req.PlantingRecordID,
		"provenance_anchor_id":        req.ProvenanceAnchorID,
		"actual_units":                req.ActualUnitsPlanted,
		"claimed_units":               req.ClaimedUnits,
		"mrv_project_did":             req.MRVProjectDID,
		"certificates_issued_count":   req.CertificatesIssuedCount,
	}
	var res AnchorRecord
	if err := i.execute(ctx, payload, &res); err != nil {
		return AnchorRecord{}, err
	}
	res.Network = "testnet"
	return res, nil
}

// ── Cap issuance types ────────────────────────────────────────────────────────

// CapResult is returned after a role capability object is issued on-chain.
type CapResult struct {
	CapObjectID    string
	TransactionRef string
	Network        string
}

// IssueCapRequest carries params for manager/verifier cap issuance.
type IssueCapRequest struct {
	Recipient string // IOTA address receiving the cap
	DID       string // DID of the user receiving the cap
	// Scope: project_id for manager, scope string for verifier
	Scope string
}

// IssueMonitorCapRequest carries params for monitor cap issuance.
type IssueMonitorCapRequest struct {
	Recipient string
	DID       string
	ProjectID string
	PlotID    string
}

// ── Cap issuance methods ──────────────────────────────────────────────────────

// IssueManagerCap calls the executor to issue a ManagerCap on conservation_chain.
func (i *IOTA) IssueManagerCap(ctx context.Context, req IssueCapRequest) (CapResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return CapResult{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":   "issue_manager_cap",
		"package_id":  i.packageID,
		"recipient":   req.Recipient,
		"manager_did": req.DID,
		"project_id":  req.Scope,
		"network":     "testnet",
	}
	var response struct {
		ObjectID       string `json:"objectId"`
		TransactionRef string `json:"transaction_reference"`
	}
	if err := i.execute(ctx, payload, &response); err != nil {
		return CapResult{}, fmt.Errorf("issue_manager_cap: %w", err)
	}
	if response.ObjectID == "" {
		return CapResult{}, fmt.Errorf("issue_manager_cap: executor returned empty object ID")
	}
	return CapResult{CapObjectID: response.ObjectID, TransactionRef: response.TransactionRef, Network: "testnet"}, nil
}

// IssueMonitorCap calls the executor to issue a MonitorCap on conservation_chain.
func (i *IOTA) IssueMonitorCap(ctx context.Context, req IssueMonitorCapRequest) (CapResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return CapResult{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":   "issue_monitor_cap",
		"package_id":  i.packageID,
		"recipient":   req.Recipient,
		"monitor_did": req.DID,
		"project_id":  req.ProjectID,
		"plot_id":     req.PlotID,
		"network":     "testnet",
	}
	var response struct {
		ObjectID       string `json:"objectId"`
		TransactionRef string `json:"transaction_reference"`
	}
	if err := i.execute(ctx, payload, &response); err != nil {
		return CapResult{}, fmt.Errorf("issue_monitor_cap: %w", err)
	}
	if response.ObjectID == "" {
		return CapResult{}, fmt.Errorf("issue_monitor_cap: executor returned empty object ID")
	}
	return CapResult{CapObjectID: response.ObjectID, TransactionRef: response.TransactionRef, Network: "testnet"}, nil
}

// IssueVerifierCap calls the executor to issue a VerifierCap on conservation_chain.
func (i *IOTA) IssueVerifierCap(ctx context.Context, req IssueCapRequest) (CapResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return CapResult{}, fmt.Errorf("%w: executor or package not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":    "issue_verifier_cap",
		"package_id":   i.packageID,
		"recipient":    req.Recipient,
		"verifier_did": req.DID,
		"scope":        req.Scope,
		"network":      "testnet",
	}
	var response struct {
		ObjectID       string `json:"objectId"`
		TransactionRef string `json:"transaction_reference"`
	}
	if err := i.execute(ctx, payload, &response); err != nil {
		return CapResult{}, fmt.Errorf("issue_verifier_cap: %w", err)
	}
	if response.ObjectID == "" {
		return CapResult{}, fmt.Errorf("issue_verifier_cap: executor returned empty object ID")
	}
	return CapResult{CapObjectID: response.ObjectID, TransactionRef: response.TransactionRef, Network: "testnet"}, nil
}

// LinkRoleObject calls the executor to link a role cap to an IdentityDocument
// in bahari_identity.
func (i *IOTA) LinkRoleObject(ctx context.Context, docObjectID string, roleObjectID string) error {
	if i.executorURL == "" {
		return fmt.Errorf("%w: executor not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":      "link_role_object",
		"doc_object_id":  docObjectID,
		"role_object_id": roleObjectID,
		"network":        "testnet",
	}
	var response map[string]any
	return i.execute(ctx, payload, &response)
}
