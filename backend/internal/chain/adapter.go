package chain

import (
	"context"
	"errors"
	"strings"
)

var ErrNotFound = errors.New("chain object not found")
var ErrUnavailable = errors.New("chain integration unavailable")

type IdentityResult struct {
	DID            string
	DIDDocumentRef string
	Address        string
	TransactionRef string
	Network        string
}

type AnchorRequest struct {
	IdempotencyKey   string `json:"idempotency_key"`
	Kind             string `json:"kind"`
	Commitment       string `json:"commitment"`
	CanonicalJSON    []byte `json:"canonical_json"`
	IssuerDID        string `json:"issuer_did"`
	PreviousObjectID string `json:"previous_object_id,omitempty"`
}

type AnchorRecord struct {
	ObjectID       string `json:"object_id"`
	TransactionRef string `json:"transaction_reference"`
	Commitment     string `json:"commitment"`
	IssuerDID      string `json:"issuer_did"`
	Status         string `json:"status"`
	Network        string `json:"network"`
	SupersededByID string `json:"superseded_by_id,omitempty"`
	Revoked        bool   `json:"revoked"`
}

type Adapter interface {
	Mode() string
	Network() string
	ProvisionIdentity(ctx context.Context, userID string, publicKey []byte) (IdentityResult, error)
	Anchor(ctx context.Context, request AnchorRequest) (AnchorRecord, error)
	ReadAnchor(ctx context.Context, objectID string) (AnchorRecord, error)
	ResolveIdentity(ctx context.Context, didDocumentRef string) (bool, error)
}

// ExplorerBase derives the public explorer root for the adapter's network. Sandbox
// and any unrecognised network have no explorer, so no link is ever invented.
func ExplorerBase(network, override string) string {
	if override != "" {
		return strings.TrimRight(override, "/")
	}
	switch network {
	case "testnet":
		return "https://explorer.testnet.iota.cafe"
	case "mainnet":
		return "https://explorer.iota.cafe"
	default:
		return ""
	}
}

func ExplorerTransaction(network, override, reference string) string {
	base := ExplorerBase(network, override)
	if base == "" || reference == "" {
		return ""
	}
	return base + "/transaction/" + reference
}

func ExplorerObject(network, override, objectID string) string {
	base := ExplorerBase(network, override)
	if base == "" || objectID == "" {
		return ""
	}
	return base + "/object/" + objectID
}
