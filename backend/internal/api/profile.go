package api

import (
	"net/url"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
)

// identityProfile is the projection SPEC section 6 gives both clients: the IOTA
// identity state, the capabilities the backend will actually authorise, and the
// federated links on this account. Nothing here is a claim of on-chain state the
// adapter has not confirmed.
type identityProfile struct {
	User                  models.User      `json:"user"`
	Status                string           `json:"status"`
	DID                   string           `json:"did,omitempty"`
	Address               string           `json:"address,omitempty"`
	DIDDocumentReference  string           `json:"didDocumentReference,omitempty"`
	KeyCustodyReference   string           `json:"keyCustodyReference,omitempty"`
	Network               string           `json:"network"`
	RoleCapabilities      []roleCapability `json:"roleCapabilities"`
	TransactionReferences []string         `json:"transactionReferences"`
	LinkedPlatforms       []linkedPlatform `json:"linkedPlatforms"`
	ProvisioningError     string           `json:"provisioningError,omitempty"`
}

type roleCapability struct {
	Role      string `json:"role"`
	Status    string `json:"status"`
	Reference string `json:"reference,omitempty"`
}

type linkedPlatform struct {
	Issuer   string    `json:"issuer"`
	Platform string    `json:"platform"`
	Subject  string    `json:"subject"`
	LinkedAt time.Time `json:"linkedAt"`
	Status   string    `json:"status"`
}

func (r *router) profile(userID string) (identityProfile, error) {
	var user models.User
	if err := r.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return identityProfile{}, err
	}
	provision, err := r.identities.ForUser(userID)
	if err != nil {
		return identityProfile{}, err
	}
	links, err := r.links.Links(userID)
	if err != nil {
		return identityProfile{}, err
	}
	profile := identityProfile{
		User:            user,
		Status:          provision.State,
		DID:             provision.DID,
		Address:         provision.Address,
		Network:         chainNetwork(provision.Network),
		LinkedPlatforms: platforms(links),
	}
	if provision.State == models.IdentityActive {
		profile.DIDDocumentReference = provision.DIDDocumentRef
		profile.KeyCustodyReference = r.keyCustodyReference(userID)
		profile.TransactionReferences = append(profile.TransactionReferences, provision.ProvisionTxRef)
	} else if provision.LastError != "" {
		profile.ProvisioningError = provision.LastError
	}
	profile.RoleCapabilities = capabilitiesFor(user, provision)
	profile.TransactionReferences = nonEmpty(profile.TransactionReferences)
	return profile, nil
}

// keyCustodyReference names the managed envelope without touching key material.
func (r *router) keyCustodyReference(userID string) string {
	var reference models.KeyReference
	if err := r.db.Where("user_id = ? AND revoked_at IS NULL", userID).Order("created_at desc").First(&reference).Error; err != nil {
		return ""
	}
	short := reference.ID
	if len(short) > 8 {
		short = short[:8]
	}
	return reference.Provider + ":" + short
}

func capabilitiesFor(user models.User, provision models.IdentityProvision) []roleCapability {
	status := "pending"
	reference := ""
	if provision.State == models.IdentityActive && user.Status == models.IdentityActive {
		status = "active"
		reference = provision.DIDDocumentRef
	}
	capabilities := []roleCapability{{Role: user.Role, Status: status, Reference: reference}}
	if status == "active" && (user.Role == models.RoleMonitor || user.Role == models.RoleVerifier) {
		capabilities = append(capabilities, roleCapability{
			Role: "evidence_signature", Status: "active", Reference: provision.DID,
		})
	}
	return capabilities
}

func platforms(links []models.ExternalIdentity) []linkedPlatform {
	platforms := make([]linkedPlatform, 0, len(links))
	for _, link := range links {
		status := "active"
		if link.RevokedAt != nil {
			status = "revoked"
		}
		platforms = append(platforms, linkedPlatform{
			Issuer: link.Issuer, Platform: platformName(link.Issuer),
			Subject: link.Subject, LinkedAt: link.CreatedAt, Status: status,
		})
	}
	return platforms
}

// platformName labels an issuer from its own host, never from a fixed list, so a
// newly configured BahariLink platform appears correctly without a code change.
func platformName(issuer string) string {
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Hostname() == "" {
		return "BahariLink"
	}
	label := strings.Split(parsed.Hostname(), ".")[0]
	if label == "" || label == "www" || label == "localhost" {
		return "BahariLink"
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

// chainNetwork names the IOTA network for clients. A sandbox adapter is never
// reported as a testnet identity.
func chainNetwork(network string) string {
	switch network {
	case "testnet":
		return "iota_testnet"
	case "mainnet":
		return "iota_mainnet"
	default:
		return "sandbox"
	}
}

func nonEmpty(values []string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
