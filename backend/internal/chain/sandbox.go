package chain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// Sandbox is an explicitly non-live, in-memory adapter for local development
// and contract tests. Its identifiers always carry a sandbox prefix.
type Sandbox struct {
	mu         sync.RWMutex
	identities map[string]IdentityResult
	anchors    map[string]AnchorRecord
	byKey      map[string]string
	documents  map[string]bool
}

func NewSandbox() *Sandbox {
	return &Sandbox{
		identities: make(map[string]IdentityResult),
		anchors:    make(map[string]AnchorRecord),
		byKey:      make(map[string]string),
		documents:  make(map[string]bool),
	}
}

func (s *Sandbox) Mode() string    { return "sandbox" }
func (s *Sandbox) Network() string { return "sandbox" }

func (s *Sandbox) ProvisionIdentity(_ context.Context, userID string, publicKey []byte) (IdentityResult, error) {
	if len(publicKey) == 0 {
		return IdentityResult{}, fmt.Errorf("public key is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.identities[userID]; ok {
		return current, nil
	}
	digest := sha256.Sum256(append([]byte(userID), publicKey...))
	identifier := hex.EncodeToString(digest[:16])
	result := IdentityResult{
		DID:            "did:iota:sandbox:" + identifier,
		DIDDocumentRef: "sandbox-document-" + identifier,
		Address:        "sandbox-address-" + identifier,
		TransactionRef: "sandbox-identity-transaction-" + identifier,
		Network:        "sandbox",
	}
	s.identities[userID] = result
	s.documents[result.DIDDocumentRef] = true
	return result, nil
}

func (s *Sandbox) Anchor(_ context.Context, request AnchorRequest) (AnchorRecord, error) {
	if request.Commitment == "" || request.IdempotencyKey == "" {
		return AnchorRecord{}, fmt.Errorf("commitment and idempotency key are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if objectID, ok := s.byKey[request.IdempotencyKey]; ok {
		return s.anchors[objectID], nil
	}
	digest := sha256.Sum256([]byte(request.IdempotencyKey + ":" + request.Commitment))
	identifier := hex.EncodeToString(digest[:16])
	objectID := "sandbox-anchor-" + identifier
	record := AnchorRecord{
		ObjectID:       objectID,
		TransactionRef: "sandbox-anchor-transaction-" + identifier,
		Commitment:     request.Commitment,
		IssuerDID:      request.IssuerDID,
		Status:         "confirmed",
		Network:        "sandbox",
	}
	s.anchors[objectID] = record
	s.byKey[request.IdempotencyKey] = objectID
	return record, nil
}

func (s *Sandbox) ReadAnchor(_ context.Context, objectID string) (AnchorRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.anchors[objectID]
	if !ok {
		return AnchorRecord{}, ErrNotFound
	}
	return record, nil
}

func (s *Sandbox) ResolveIdentity(_ context.Context, didDocumentRef string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.documents[didDocumentRef], nil
}

// ── Sandbox stubs for extended conservation_chain operations ──────────────────
// These return deterministic sandbox IDs so local dev works without an executor.

func (s *Sandbox) CreateLedger(_ context.Context, req LedgerRequest) (LedgerResult, error) {
	digest := sha256.Sum256([]byte("ledger:" + req.MRVProjectID))
	id := "sandbox-ledger-" + hex.EncodeToString(digest[:8])
	return LedgerResult{ObjectID: id, TransactionRef: "sandbox-ledger-tx-" + hex.EncodeToString(digest[:8])}, nil
}

func (s *Sandbox) LinkJahazi(_ context.Context, req LinkJahaziRequest) (LedgerResult, error) {
	return LedgerResult{ObjectID: req.LedgerObjectID, TransactionRef: "sandbox-link-tx"}, nil
}

func (s *Sandbox) IssueContributionCap(_ context.Context, req IssueContributionCapRequest) (LedgerResult, error) {
	digest := sha256.Sum256([]byte("cap:" + req.Recipient + req.LedgerObjectID))
	id := "sandbox-cap-" + hex.EncodeToString(digest[:8])
	return LedgerResult{ObjectID: id, TransactionRef: "sandbox-cap-tx-" + hex.EncodeToString(digest[:8])}, nil
}

func (s *Sandbox) AnchorRestorationClaim(_ context.Context, req RestorationClaimRequest) (AnchorRecord, error) {
	digest := sha256.Sum256([]byte("claim:" + req.JahaziClaimID + req.Nonce))
	id := "sandbox-claim-anchor-" + hex.EncodeToString(digest[:8])
	txRef := "sandbox-claim-tx-" + hex.EncodeToString(digest[:8])
	record := AnchorRecord{ObjectID: id, TransactionRef: txRef, Commitment: req.ContributorMerkleRoot, IssuerDID: req.JahaziPlatformDID, Status: "confirmed", Network: "sandbox"}
	s.mu.Lock()
	s.anchors[id] = record
	s.mu.Unlock()
	return record, nil
}

func (s *Sandbox) CreatePlantingRecord(_ context.Context, req PlantingRecordRequest) (AnchorRecord, error) {
	digest := sha256.Sum256([]byte("planting:" + req.MRVActivityID + req.EvidenceRoot))
	id := "sandbox-planting-" + hex.EncodeToString(digest[:8])
	txRef := "sandbox-planting-tx-" + hex.EncodeToString(digest[:8])
	record := AnchorRecord{ObjectID: id, TransactionRef: txRef, Commitment: req.EvidenceRoot, IssuerDID: req.VerifierDID, Status: "confirmed", Network: "sandbox"}
	s.mu.Lock()
	s.anchors[id] = record
	s.mu.Unlock()
	return record, nil
}

func (s *Sandbox) AppendPlantingEntry(_ context.Context, req PlantingEntryRequest) (LedgerResult, error) {
	digest := sha256.Sum256([]byte("pe:" + req.LedgerObjectID + req.PlantingRecordID))
	id := "sandbox-planting-entry-" + hex.EncodeToString(digest[:8])
	return LedgerResult{ObjectID: id, TransactionRef: "sandbox-pe-tx-" + hex.EncodeToString(digest[:8])}, nil
}

func (s *Sandbox) RecordFulfillment(_ context.Context, req FulfillmentRequest) (AnchorRecord, error) {
	digest := sha256.Sum256([]byte("fulfil:" + req.ClaimAnchorID + req.PlantingRecordID))
	id := "sandbox-fulfil-" + hex.EncodeToString(digest[:8])
	txRef := "sandbox-fulfil-tx-" + hex.EncodeToString(digest[:8])
	record := AnchorRecord{ObjectID: id, TransactionRef: txRef, Commitment: req.PlantingRecordID, Status: "confirmed", Network: "sandbox"}
	s.mu.Lock()
	s.anchors[id] = record
	s.mu.Unlock()
	return record, nil
}

// IssueManagerCap returns a synthetic cap for sandbox use.
func (s *Sandbox) IssueManagerCap(_ context.Context, req IssueCapRequest) (CapResult, error) {
	id := "sandbox-manager-cap-" + req.DID[:min(8, len(req.DID))]
	return CapResult{CapObjectID: id, TransactionRef: "sandbox-tx-" + id, Network: "sandbox"}, nil
}

// IssueMonitorCap returns a synthetic cap for sandbox use.
func (s *Sandbox) IssueMonitorCap(_ context.Context, req IssueMonitorCapRequest) (CapResult, error) {
	id := "sandbox-monitor-cap-" + req.DID[:min(8, len(req.DID))]
	return CapResult{CapObjectID: id, TransactionRef: "sandbox-tx-" + id, Network: "sandbox"}, nil
}

// IssueVerifierCap returns a synthetic cap for sandbox use.
func (s *Sandbox) IssueVerifierCap(_ context.Context, req IssueCapRequest) (CapResult, error) {
	id := "sandbox-verifier-cap-" + req.DID[:min(8, len(req.DID))]
	return CapResult{CapObjectID: id, TransactionRef: "sandbox-tx-" + id, Network: "sandbox"}, nil
}

// LinkRoleObject is a no-op in sandbox mode.
func (s *Sandbox) LinkRoleObject(_ context.Context, _, _ string) error { return nil }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
