// Package jahazi — on-chain operations for the conservation_chain Move package.
// These methods are called after key lifecycle events and are always non-blocking:
// a chain failure never blocks the database write that precedes it.
package jahazi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/models"
	"gorm.io/gorm"
)

// ChainOps handles all conservation_chain Move operations triggered by Jahazi
// integration events. It wraps the chain.Adapter so callers never touch the
// adapter directly.
type ChainOps struct {
	db      *gorm.DB
	adapter chain.Adapter
	logger  *slog.Logger
}

// NewChainOps creates a ChainOps that issues conservation_chain calls.
// If adapter.Mode() == "sandbox", all calls produce deterministic fake IDs.
func NewChainOps(db *gorm.DB, adapter chain.Adapter) *ChainOps {
	return &ChainOps{db: db, adapter: adapter, logger: slog.Default().With("component", "jahazi.chain")}
}

// ── Project lifecycle ─────────────────────────────────────────────────────────

// EnsureProjectLedger creates a ProjectImpactLedger for the project if it
// doesn't have one yet. Idempotent: does nothing if LedgerObjectID is already set.
func (c *ChainOps) EnsureProjectLedger(ctx context.Context, projectID string) error {
	var project models.ConservationProject
	if err := c.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return err
	}
	if project.LedgerObjectID != "" {
		return nil // already created
	}

	did := fmt.Sprintf("did:iota:testnet:project:%s", project.ID)
	if did == "" {
		did = fmt.Sprintf("did:iota:testnet:project:%s", project.ID)
	}

	res, err := c.adapter.CreateLedger(ctx, chain.LedgerRequest{
		ProjectDID:   did,
		MRVProjectID: project.ID,
	})
	if err != nil {
		c.logger.Warn("CreateLedger failed", "projectId", project.ID, "error", err)
		return err
	}

	return c.db.Model(&models.ConservationProject{}).Where("id = ?", project.ID).
		Update("ledger_object_id", res.ObjectID).Error
}

// LinkJahaziProject sets the Jahazi project DID on the on-chain ledger.
func (c *ChainOps) LinkJahaziProject(ctx context.Context, projectID, jahaziProjectDID string) error {
	var project models.ConservationProject
	if err := c.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return err
	}
	if project.LedgerObjectID == "" {
		return fmt.Errorf("project %s has no LedgerObjectID — call EnsureProjectLedger first", projectID)
	}

	_, err := c.adapter.LinkJahazi(ctx, chain.LinkJahaziRequest{
		LedgerObjectID:   project.LedgerObjectID,
		JahaziProjectDID: jahaziProjectDID,
	})
	if err != nil {
		c.logger.Warn("LinkJahazi failed", "projectId", projectID, "error", err)
	}
	return err
}

// IssueContributionCap issues a ContributionCap to a Jahazi executor wallet.
// The cap allows Jahazi to append ContributionLedgerEntry records to this project's ledger.
func (c *ChainOps) IssueContributionCap(ctx context.Context, projectID, recipientAddress, jahaziPlatformDID string) (string, error) {
	var project models.ConservationProject
	if err := c.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return "", err
	}
	if project.LedgerObjectID == "" {
		return "", fmt.Errorf("project %s has no LedgerObjectID", projectID)
	}

	res, err := c.adapter.IssueContributionCap(ctx, chain.IssueContributionCapRequest{
		Recipient:         recipientAddress,
		LedgerObjectID:    project.LedgerObjectID,
		JahaziPlatformDID: jahaziPlatformDID,
	})
	if err != nil {
		c.logger.Warn("IssueContributionCap failed", "projectId", projectID, "error", err)
		return "", err
	}
	return res.ObjectID, nil
}

// ── Restoration claim ─────────────────────────────────────────────────────────

// AnchorRestorationClaim creates a RestorationClaimAnchor on-chain when a
// RestorationClaim is received from Jahazi.
func (c *ChainOps) AnchorRestorationClaim(ctx context.Context, req *models.RestorationRequest) (string, error) {
	var project models.ConservationProject
	if err := c.db.Where("id = ?", req.ProjectID).First(&project).Error; err != nil {
		return "", err
	}

	projectDID := fmt.Sprintf("did:iota:testnet:project:%s", project.ID)
	if projectDID == "" {
		projectDID = fmt.Sprintf("did:iota:testnet:project:%s", project.ID)
	}

	// Compute locationHash from the project's coordinates (privacy-safe)
	locationHash := ""
	if project.JahaziProjectID != "" {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s", project.JahaziProjectID)))
		locationHash = hex.EncodeToString(h[:])
	}

	// ContributorMerkleRoot: placeholder until Jahazi sends it in the claim payload.
	// The real Merkle root should come from the RestorationRequest.ContributorJSON.
	merkleRoot := contributorMerkleRoot(req)

	res, err := c.adapter.AnchorRestorationClaim(ctx, chain.RestorationClaimRequest{
		JahaziClaimID:         req.JahaziClaimID,
		JahaziProjectDID:      fmt.Sprintf("did:iota:testnet:project:%s", req.JahaziProjectID),
		MRVProjectDID:         projectDID,
		PeriodStartMs:         req.PeriodStart.UnixMilli(),
		PeriodEndMs:           req.PeriodEnd.UnixMilli(),
		TotalCostCents:        costToCents(req.TotalCostUsd),
		TotalUnits:            int64(req.TotalUnits),
		Species:               req.Species,
		ContributorCount:      int64(req.ContributorCount),
		ContributorMerkleRoot: merkleRoot,
		LocationHash:          locationHash,
		JahaziPlatformDID:     "did:iota:testnet:platform:jahazi",
		Nonce:                 req.Nonce,
	})
	if err != nil {
		c.logger.Warn("AnchorRestorationClaim failed", "claimId", req.JahaziClaimID, "error", err)
		return "", err
	}

	// Store the on-chain object ID in the RestorationRequest
	if dbErr := c.db.Model(&models.RestorationRequest{}).Where("id = ?", req.ID).
		Update("chain_anchor_object_id", res.ObjectID).Error; dbErr != nil {
		c.logger.Warn("failed to store claim anchor object ID", "error", dbErr)
	}

	return res.ObjectID, nil
}

// ── Planting record ───────────────────────────────────────────────────────────

// CreatePlantingRecord freezes a PlantingRecord on-chain after provenance is confirmed.
// Called from the provenance service after a ProvenanceAnchor is confirmed.
func (c *ChainOps) CreatePlantingRecord(
	ctx context.Context,
	activity models.ConservationActivity,
	provenanceAnchorObjectID string,
	claimAnchorObjectID string,
	measurements map[string]any,
) (string, error) {
	var project models.ConservationProject
	if err := c.db.Where("id = ?", activity.ProjectID).First(&project).Error; err != nil {
		return "", err
	}
	if project.LedgerObjectID == "" {
		c.logger.Warn("project has no LedgerObjectID — skipping PlantingRecord",
			"projectId", project.ID)
		return "", nil
	}

	projectDID := fmt.Sprintf("did:iota:testnet:project:%s", project.ID)
	if projectDID == "" {
		projectDID = fmt.Sprintf("did:iota:testnet:project:%s", project.ID)
	}

	species, _ := measurements["species"].(string)
	planted := int64Extract(measurements, "planted_count")
	alive := int64Extract(measurements, "alive_count")
	survival := int64(0)
	if planted > 0 {
		survival = int64(float64(alive) / float64(planted) * 10000) // × 100 for hundredths
	}

	evidenceRoot := stringExtract(measurements, "evidence_root")
	if evidenceRoot == "" {
		// Derive from measurements if not explicitly set
		h := sha256.Sum256([]byte(fmt.Sprintf("%v", measurements)))
		evidenceRoot = hex.EncodeToString(h[:])
	}

	methodVersion := activity.MethodologyVersion
	if methodVersion == "" {
		methodVersion = "1.0"
	}

	// Verifier DID from the latest attestation
	var attestation models.VerifierAttestation
	verifierDID := ""
	attestationHash := ""
	isVerified := false
	if err := c.db.Where("activity_id = ? AND revoked_at IS NULL", activity.ID).
		Order("created_at desc").First(&attestation).Error; err == nil {
		verifierDID = attestation.IssuerDID
		attestationHash = attestation.AttestationHash
		isVerified = attestation.Decision == models.DecisionApprove
	}

	res, err := c.adapter.CreatePlantingRecord(ctx, chain.PlantingRecordRequest{
		ProjectDID:              projectDID,
		LedgerObjectID:          project.LedgerObjectID,
		ProvenanceAnchorID:      provenanceAnchorObjectID,
		ClaimAnchorID:           claimAnchorObjectID,
		Species:                 species,
		SeedlingsPlanted:        planted,
		SeedlingsAlive:          alive,
		SurvivalPctHundredths:   survival,
		AreaHaMillionths:        0, // populated from Plot when available
		MethodologyVersion:      methodVersion,
		ReportingPeriodStartMs:  activity.ReportingPeriodStart.UnixMilli(),
		ReportingPeriodEndMs:    activity.ReportingPeriodEnd.UnixMilli(),
		EvidenceRoot:            evidenceRoot,
		SatelliteCorrobHash:     stringExtract(measurements, "satellite_hash"),
		PhotoHash:               "",
		LocationHash:            "",
		VerifierDID:             verifierDID,
		VerifierAttestationHash: attestationHash,
		IsVerified:              isVerified,
		PreviousRecordID:        "",
		MRVActivityID:           activity.ID,
	})
	if err != nil {
		c.logger.Warn("CreatePlantingRecord failed", "activityId", activity.ID, "error", err)
		return "", err
	}

	// Append the ledger entry
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		co2eMg := int64(float64(planted) * 4355) // 4.355 kg/tree × 1000
		if _, err := c.adapter.AppendPlantingEntry(bgCtx, chain.PlantingEntryRequest{
			LedgerObjectID:      project.LedgerObjectID,
			PlantingRecordID:    res.ObjectID,
			ProvenanceAnchorID:  provenanceAnchorObjectID,
			SeedlingsAdded:      planted,
			AliveAdded:          alive,
			Co2eMilligramsAdded: co2eMg,
		}); err != nil {
			c.logger.Warn("AppendPlantingEntry failed", "error", err)
		}
	}()

	return res.ObjectID, nil
}

// RecordFulfillment creates a RestorationFulfillmentRecord linking claim → planting.
func (c *ChainOps) RecordFulfillment(
	ctx context.Context,
	claimAnchorObjectID string,
	plantingRecordObjectID string,
	provenanceAnchorObjectID string,
	actualUnits int,
	claimedUnits int,
	projectDID string,
	certsIssued int,
) (string, error) {
	res, err := c.adapter.RecordFulfillment(ctx, chain.FulfillmentRequest{
		ClaimAnchorID:           claimAnchorObjectID,
		PlantingRecordID:        plantingRecordObjectID,
		ProvenanceAnchorID:      provenanceAnchorObjectID,
		ActualUnitsPlanted:      int64(actualUnits),
		ClaimedUnits:            int64(claimedUnits),
		MRVProjectDID:           projectDID,
		CertificatesIssuedCount: int64(certsIssued),
	})
	if err != nil {
		c.logger.Warn("RecordFulfillment failed", "error", err)
		return "", err
	}
	return res.ObjectID, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func contributorMerkleRoot(req *models.RestorationRequest) string {
	// Hash the contributor JSON as a placeholder Merkle root.
	// A proper implementation would build a full Merkle tree.
	raw, _ := json.Marshal(req.ContributorJSON)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func costToCents(cost string) int64 {
	// Parse a decimal USD string like "45.00" to cents (4500).
	var dollars, cents int64
	n, _ := fmt.Sscanf(cost, "%d.%d", &dollars, &cents)
	if n < 2 {
		fmt.Sscanf(cost, "%d", &dollars)
		return dollars * 100
	}
	return dollars*100 + cents
}

func int64Extract(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func stringExtract(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}
