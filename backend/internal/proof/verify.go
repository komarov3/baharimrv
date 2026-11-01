package proof

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/models"
	"gorm.io/gorm"
)

// Check is one independent verification result. Statuses stay distinct so a
// missing external integration never reads as a pass or a failure.
type Check struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type Verification struct {
	OverallStatus string    `json:"overallStatus"`
	CheckedAt     time.Time `json:"checkedAt"`
	Checks        []Check   `json:"checks"`
}

const (
	passed        = "passed"
	failed        = "failed"
	pending       = "pending"
	unavailable   = "unavailable"
	notApplicable = "not_applicable"
)

// Verify recomputes every check the backend is able to perform. Checks that
// depend on an integration which is not live are reported as unavailable.
func (s *Service) Verify(ctx context.Context, proofID string) (Verification, error) {
	snapshot, document, err := s.Snapshot(proofID)
	if err != nil {
		return Verification{}, err
	}
	checks := []Check{
		s.checkSchema(document),
		s.checkEvidence(snapshot),
		s.checkChain(ctx, snapshot),
		s.checkIssuer(ctx, snapshot),
		s.checkAttestation(snapshot),
		s.checkRegistry(snapshot),
		s.checkSupersession(snapshot, document),
	}
	return Verification{OverallStatus: overall(checks), CheckedAt: s.now(), Checks: checks}, nil
}

// GetPublic returns the privacy-minimised projection with live checks attached.
func (s *Service) GetPublic(ctx context.Context, proofID string) (View, Verification, error) {
	view, err := s.Get(proofID, true)
	if err != nil {
		return View{}, Verification{}, err
	}
	verification, verifyErr := s.Verify(ctx, proofID)
	if verifyErr != nil {
		return view, Verification{}, nil
	}
	return view, verification, nil
}

func (s *Service) checkSchema(document models.ProofDocument) Check {
	check := Check{Key: "schema_signature", Label: "Schema and manifest integrity", Status: failed}
	canonical, err := canon.Bytes(document.Snapshot)
	if err != nil {
		check.Detail = "The stored snapshot cannot be canonicalized."
		return check
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != document.ManifestHash {
		check.Detail = "The recomputed manifest hash does not match the stored hash; the snapshot has been altered."
		return check
	}
	var schemaOnly struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(document.Snapshot, &schemaOnly); err != nil || schemaOnly.Schema != SchemaVersion {
		check.Detail = fmt.Sprintf("Unsupported proof schema %q.", schemaOnly.Schema)
		return check
	}
	check.Status = passed
	check.Detail = "Canonical JSON re-hashes to the stored manifest hash and matches schema " + SchemaVersion + "."
	return check
}

func (s *Service) checkEvidence(snapshot Snapshot) Check {
	check := Check{Key: "evidence_integrity", Label: "Evidence integrity", Status: passed}
	if len(snapshot.EvidenceManifest) == 0 {
		check.Status = pending
		check.Detail = "No evidence files are declared in this snapshot."
		return check
	}
	missing, mismatched, unverifiable := 0, 0, 0
	for _, entry := range snapshot.EvidenceManifest {
		var object models.EvidenceObject
		if err := s.db.Where("sha256 = ?", entry.SHA256).First(&object).Error; err != nil {
			missing++
			continue
		}
		if object.SHA256 != entry.SHA256 {
			mismatched++
			continue
		}
		if entry.Access != "public" {
			continue
		}
		reader, _, err := s.store.Open(object.StorageURI)
		if err != nil {
			unverifiable++
			continue
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(hasher, reader)
		_ = reader.Close()
		if copyErr != nil || hex.EncodeToString(hasher.Sum(nil)) != entry.SHA256 {
			mismatched++
		}
	}
	switch {
	case mismatched > 0:
		check.Status = failed
		check.Detail = fmt.Sprintf("%d evidence file(s) do not hash to their manifest value.", mismatched)
	case missing > 0:
		check.Status = failed
		check.Detail = fmt.Sprintf("%d declared evidence object(s) are not stored on this server.", missing)
	case unverifiable > 0:
		check.Status = unavailable
		check.Detail = fmt.Sprintf("%d public evidence file(s) could not be re-read from storage.", unverifiable)
	default:
		check.Detail = fmt.Sprintf("All %d declared evidence entries resolve to stored objects with matching digests.", len(snapshot.EvidenceManifest))
	}
	return check
}

func (s *Service) checkChain(ctx context.Context, snapshot Snapshot) Check {
	check := Check{Key: "iota_commitment", Label: "IOTA commitment readback", Status: unavailable}
	if snapshot.Chain == nil || snapshot.Chain.ObjectID == "" {
		check.Status = pending
		check.Detail = "No anchor object has been recorded for this claim."
		return check
	}
	if s.anchors.Mode() != "iota" {
		check.Detail = "Chain anchoring runs in " + s.anchors.Mode() + " mode. The stored commitment has no public IOTA object to read back."
		return check
	}
	record, err := s.anchors.ReadAnchor(ctx, snapshot.Chain.ObjectID)
	switch {
	case errors.Is(err, chain.ErrNotFound):
		check.Status = failed
		check.Detail = "The recorded anchor object cannot be found on the configured network."
	case err != nil:
		check.Status = unavailable
		check.Detail = "The testnet readback is unavailable, so the commitment cannot be confirmed right now."
	case record.Commitment != snapshot.Chain.Commitment:
		check.Status = failed
		check.Detail = "The on-chain commitment differs from the proof snapshot."
	case record.Revoked:
		check.Status = failed
		check.Detail = "The anchor object is marked revoked on-chain."
	default:
		check.Status = passed
		check.Detail = "On-chain commitment matches the snapshot on " + s.anchors.Network() + "."
	}
	return check
}

func (s *Service) checkIssuer(ctx context.Context, snapshot Snapshot) Check {
	check := Check{Key: "issuer_did", Label: "Issuer DID resolution", Status: unavailable}
	if snapshot.IssuerDID == "" {
		check.Status = failed
		check.Detail = "The proof names no issuing DID."
		return check
	}
	if snapshot.IssuerDIDDocument == "" {
		check.Detail = "No DID document reference is stored for this issuer, so it cannot be resolved."
		return check
	}
	resolved, err := s.anchors.ResolveIdentity(ctx, snapshot.IssuerDIDDocument)
	if err != nil {
		check.Detail = "The DID document could not be read from the configured network."
		return check
	}
	if !resolved {
		check.Status = failed
		check.Detail = "The issuer DID document is missing or revoked."
		return check
	}
	check.Status = passed
	check.Detail = "The issuer DID resolves on " + s.anchors.Network() + " and is not revoked."
	return check
}

func (s *Service) checkAttestation(snapshot Snapshot) Check {
	check := Check{Key: "verifier_attestation", Label: "Independent verifier attestation", Status: passed}
	if snapshot.ProofType != TypeVerified {
		check.Status = notApplicable
		check.Detail = "A completion claim is manager-attested and carries no independent verifier decision."
		return check
	}
	if snapshot.Verifier == nil {
		check.Status = failed
		check.Detail = "A verified achievement requires a stored verifier attestation."
		return check
	}
	var attestation models.VerifierAttestation
	err := s.db.Where("activity_id = ? AND attestation_hash = ?", snapshot.AchievementID, snapshot.Verifier.Hash).
		Order("created_at desc").First(&attestation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		check.Status = failed
		check.Detail = "The attestation referenced by this proof is not present in the ledger."
		return check
	}
	if err != nil {
		check.Status = unavailable
		check.Detail = "The attestation ledger could not be read."
		return check
	}
	if attestation.RevokedAt != nil {
		check.Status = failed
		check.Detail = "The verifier revoked this attestation."
		return check
	}
	if attestation.Decision != models.DecisionApprove {
		check.Status = failed
		check.Detail = "The stored verifier decision is " + attestation.Decision + ", not an approval."
		return check
	}
	check.Detail = "An unrevoked approval attestation matches the hash recorded in this proof."
	return check
}

func (s *Service) checkRegistry(snapshot Snapshot) Check {
	check := Check{Key: "registry_reference", Label: "Registry issuance reference", Status: unavailable}
	if len(snapshot.RegistryReferences) == 0 {
		check.Status = notApplicable
		check.Detail = "No registry issuance is claimed for this achievement."
		return check
	}
	var batches []models.CreditBatch
	if err := s.db.Where("project_id = ? AND registry_issuance_id IN ?", snapshot.ProjectID, snapshot.RegistryReferences).
		Find(&batches).Error; err != nil || len(batches) == 0 {
		check.Status = failed
		check.Detail = "The referenced registry issuance cannot be found in the stored batches."
		return check
	}
	live, reconciled := 0, 0
	for _, batch := range batches {
		if batch.AdapterMode == "live" {
			live++
		}
		if batch.ReconciledAt != nil {
			reconciled++
		}
	}
	switch {
	case live == 0:
		check.Detail = "Registry references are sandbox identifiers. They do not evidence issued credits."
	case reconciled < len(batches):
		check.Status = pending
		check.Detail = fmt.Sprintf("%d of %d registry batches are reconciled.", reconciled, len(batches))
	default:
		check.Status = passed
		check.Detail = fmt.Sprintf("%d reconciled registry issuance reference(s) are stored.", reconciled)
	}
	return check
}

func (s *Service) checkSupersession(snapshot Snapshot, document models.ProofDocument) Check {
	check := Check{Key: "supersession_dispute", Label: "Currency, supersession and dispute status", Status: passed}
	var disputes []models.Dispute
	if err := s.db.Where("activity_id = ? AND resolved_at IS NULL", snapshot.AchievementID).Find(&disputes).Error; err != nil {
		check.Status = unavailable
		check.Detail = "Dispute state could not be read."
		return check
	}
	switch {
	case len(disputes) > 0:
		check.Status = failed
		check.Detail = fmt.Sprintf("%d open dispute(s) exist for this achievement.", len(disputes))
	case document.Status == models.ProofRevoked:
		check.Status = failed
		check.Detail = "This proof version has been revoked."
	case document.Status == models.ProofSuperseded:
		check.Status = failed
		check.Detail = "This version is superseded by proof " + document.SupersededByID + "."
	case document.Status == models.ProofDisputed:
		check.Status = failed
		check.Detail = "The underlying claim is disputed."
	default:
		check.Detail = fmt.Sprintf("Version %d is the current %s proof and the claim is not disputed.", document.Version, document.ProofType)
	}
	return check
}

func overall(checks []Check) string {
	result := "valid"
	for _, check := range checks {
		switch check.Status {
		case failed:
			return "invalid"
		case pending:
			result = "pending"
		case unavailable:
			if result != "pending" {
				result = "unavailable"
			}
		}
	}
	return result
}
