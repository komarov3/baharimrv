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
