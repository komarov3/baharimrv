package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrProvisioning = errors.New("IOTA identity provisioning is not active")

type Service struct {
	db         *gorm.DB
	chain      chain.Adapter
	custody    *EnvelopeCustody
	now        func() time.Time
	hubURL     string
	hubAPIKey  string
	httpClient *http.Client
}

func NewService(db *gorm.DB, chainAdapter chain.Adapter, custody *EnvelopeCustody) *Service {
	return NewServiceWithHub(db, chainAdapter, custody, "", "")
}

// NewServiceWithHub creates a new identity service, optionally wired to the
// baharilink hub for cross-platform identity caching.
func NewServiceWithHub(db *gorm.DB, chainAdapter chain.Adapter, custody *EnvelopeCustody, hubURL, hubAPIKey string) *Service {
	return &Service{
		db:         db,
		chain:      chainAdapter,
		custody:    custody,
		now:        time.Now,
		hubURL:     hubURL,
		hubAPIKey:  hubAPIKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) Network() string { return s.chain.Network() }

func (s *Service) Initialize(userID string) error {
	provision := models.IdentityProvision{
		ID: uuid.NewString(), UserID: userID, State: models.IdentityPending,
		Network: s.chain.Network(), CreatedAt: s.now().UTC(), UpdatedAt: s.now().UTC(),
	}
	return s.db.Create(&provision).Error
}

func (s *Service) EnsureProvision(ctx context.Context, userID string) (models.IdentityProvision, error) {
	var provision models.IdentityProvision
	if err := s.db.Where("user_id = ?", userID).First(&provision).Error; err != nil {
		return provision, ErrProvisioning
	}
	if provision.State == models.IdentityActive && provision.DID != "" && provision.DIDDocumentRef != "" && provision.KeyReferenceID != "" {
		return provision, nil
	}
	if provision.KeyReferenceID == "" {
		material, err := s.custody.Generate()
		if err != nil {
			return provision, s.fail(&provision, "secure key generation failed")
		}
		key := models.KeyReference{
			ID: uuid.NewString(), UserID: userID, Provider: "envelope_kms",
			ExternalRef: material.Reference, WrappedKey: material.WrappedKey,
			Nonce: material.Nonce, PublicKey: material.PublicKey, CreatedAt: s.now().UTC(),
		}
		if err := s.db.Create(&key).Error; err != nil {
			return provision, s.fail(&provision, "secure key storage failed")
		}
		provision.KeyReferenceID = key.ID
	}
	var key models.KeyReference
	if err := s.db.Where("id = ? AND user_id = ? AND revoked_at IS NULL", provision.KeyReferenceID, userID).First(&key).Error; err != nil {
		return provision, s.fail(&provision, "secure key reference unavailable")
	}
	now := s.now().UTC()
	provision.Attempts++
	provision.UpdatedAt = now
	if err := s.db.Model(&models.IdentityProvision{}).Where("id = ?", provision.ID).Updates(map[string]any{
		"key_reference_id": provision.KeyReferenceID, "attempts": provision.Attempts,
		"state": models.IdentityPending, "updated_at": now,
	}).Error; err != nil {
		return provision, ErrProvisioning
	}
	result, err := s.chain.ProvisionIdentity(ctx, userID, key.PublicKey)
	if err != nil {
		return provision, s.fail(&provision, "IOTA identity provisioning failed; retry is available")
	}
	if result.Network != s.chain.Network() || result.DID == "" || result.DIDDocumentRef == "" || result.Address == "" || result.TransactionRef == "" {
		return provision, s.fail(&provision, "IOTA identity provisioning returned an incomplete result")
	}
	provision.State = models.IdentityActive
	provision.DID = result.DID
	provision.DIDDocumentRef = result.DIDDocumentRef
	provision.Address = result.Address
	provision.ProvisionTxRef = result.TransactionRef
	provision.Network = result.Network
	provision.ActivatedAt = &now
	provision.LastProvisionedAt = &now
	provision.LastError = ""
	provision.UpdatedAt = now
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.IdentityProvision{}).Where("id = ?", provision.ID).Updates(map[string]any{
			"state": provision.State, "did": provision.DID, "did_document_ref": provision.DIDDocumentRef,
			"address": provision.Address, "provision_tx_ref": provision.ProvisionTxRef,
			"network": provision.Network, "activated_at": now, "last_provisioned_at": now,
			"last_error": "", "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
			"status": models.IdentityActive, "updated_at": now,
		}).Error
	})
	if err != nil {
		return provision, ErrProvisioning
	}
	return provision, nil
}

func (s *Service) fail(provision *models.IdentityProvision, safeMessage string) error {
	now := s.now().UTC()
	provision.State = models.IdentityFailed
	provision.LastError = safeMessage
	provision.UpdatedAt = now
	_ = s.db.Model(&models.IdentityProvision{}).Where("id = ?", provision.ID).Updates(map[string]any{
		"state": models.IdentityFailed, "last_error": safeMessage, "updated_at": now,
	}).Error
	_ = s.db.Model(&models.User{}).Where("id = ?", provision.UserID).Updates(map[string]any{
		"status": models.IdentityFailed, "updated_at": now,
	}).Error
	return ErrProvisioning
}

func (s *Service) ForUser(userID string) (models.IdentityProvision, error) {
	var provision models.IdentityProvision
	if err := s.db.Where("user_id = ?", userID).First(&provision).Error; err != nil {
		return provision, err
	}
	return provision, nil
}

func (s *Service) VerifyAttestation(userID string, payload []byte, encodedSignature string) (string, error) {
	provision, err := s.ForUser(userID)
	if err != nil || provision.State != models.IdentityActive {
		return "", fmt.Errorf("active identity required")
	}
	var key models.KeyReference
	if err := s.db.Where("id = ? AND user_id = ? AND revoked_at IS NULL", provision.KeyReferenceID, userID).First(&key).Error; err != nil {
		return "", fmt.Errorf("identity key unavailable")
	}
	signature, err := base64.StdEncoding.DecodeString(encodedSignature)
	if err != nil || len(key.PublicKey) != ed25519.PublicKeySize || !ed25519.Verify(ed25519.PublicKey(key.PublicKey), payload, signature) {
		return "", fmt.Errorf("invalid monitor attestation")
	}
	return provision.DID, nil
}

func (s *Service) Recover(ctx context.Context, userID string) (models.IdentityProvision, error) {
	// Recovery resumes the existing custodial reference. It never derives a
	// replacement key from a user identifier or contact field.
	return s.EnsureProvision(ctx, userID)
}

func (s *Service) Rotate(ctx context.Context, userID string) (models.IdentityProvision, error) {
	provision, err := s.ForUser(userID)
	if err != nil || provision.State != models.IdentityActive {
		return provision, fmt.Errorf("active identity required")
	}
	now := s.now().UTC()
	if err := s.db.Model(&models.KeyReference{}).Where("id = ?", provision.KeyReferenceID).Update("revoked_at", now).Error; err != nil {
		return provision, err
	}
	if err := s.db.Model(&models.IdentityProvision{}).Where("id = ?", provision.ID).Updates(map[string]any{
		"key_reference_id": "", "state": models.IdentityPending, "did": "", "did_document_ref": "",
		"address": "", "provision_tx_ref": "", "updated_at": now,
	}).Error; err != nil {
		return provision, err
	}
	return s.EnsureProvision(ctx, userID)
}

// IssueCapForUser issues the appropriate role capability on conservation_chain
// for the given user role, then links it to their IdentityDocument.
// Returns true when the cap was issued (or when the role does not require one).
// Returns false when IssueManagerCap or IssueVerifierCap returns an error.
// Cap can be retried on next login.
//
// Cap mapping (spec §2.3):
//
//	project_manager  → ManagerCap (empty project_id until assigned)
//	independent_verifier → VerifierCap (scope="global")
//	field_monitor    → deferred to plot assignment, not issued at registration
//	carbon_buyer / finance_operator / platform_administrator → DID only, no cap
func (s *Service) IssueCapForUser(ctx context.Context, userID string, role string, provision models.IdentityProvision) bool {
	if provision.DID == "" || provision.DIDDocumentRef == "" {
		return false // identity not yet active
	}
	// Executor wallet address acts as recipient for testnet custodial model.
	// In production with self-sovereign wallets, replace with the user's IOTA address.
	recipient := provision.Address
	if recipient == "" {
		return false
	}

	var capRes chain.CapResult
	var err error

	switch role {
	case models.RoleManager:
		capRes, err = s.chain.IssueManagerCap(ctx, chain.IssueCapRequest{
			Recipient: recipient,
			DID:       provision.DID,
			Scope:     "", // empty project_id at registration; updated on project assignment
		})
	case models.RoleVerifier:
		capRes, err = s.chain.IssueVerifierCap(ctx, chain.IssueCapRequest{
			Recipient: recipient,
			DID:       provision.DID,
			Scope:     "global",
		})
	default:
		// monitor: deferred to plot assignment
		// buyer, finance, admin: DID only
		return true
	}

	if err != nil {
		slog.Warn("IssueCapForUser chain error", "userID", userID, "role", role, "error", err)
		return false
	}

	// Link the cap object to the IdentityDocument in bahari_identity.
	if linkErr := s.chain.LinkRoleObject(ctx, provision.DIDDocumentRef, capRes.CapObjectID); linkErr != nil {
		slog.Warn("IssueCapForUser LinkRoleObject", "userID", userID, "error", linkErr)
		// Cap still issued — just not linked on-chain. Non-fatal.
	}

	// Persist cap object ID.
	if err := s.db.Model(&models.IdentityProvision{}).
		Where("id = ?", provision.ID).
		Update("role_cap_object_id", capRes.CapObjectID).Error; err != nil {
		slog.Warn("failed to persist role_cap_object_id",
			"userID", userID, "capObjectID", capRes.CapObjectID, "error", err)
	}

	// Sync to hub after successful issuance (fire-and-forget).
	provision.RoleCapObjectID = capRes.CapObjectID
	go s.SyncToHub(ctx, userID, role, provision)
	return true
}

// IssueMonitorCapForUser issues a MonitorCap on conservation_chain and links it
// to the user's IdentityDocument. Called on plot assignment for field monitors.
// Non-fatal: logs and returns without error on chain failure.
func (s *Service) IssueMonitorCapForUser(ctx context.Context, userID string, role string, provision models.IdentityProvision, projectID string, plotID string) {
	if provision.DID == "" || provision.DIDDocumentRef == "" {
		return
	}
	if provision.RoleCapObjectID != "" {
		return // idempotent
	}
	recipient := provision.Address
	if recipient == "" {
		return
	}
	capRes, err := s.chain.IssueMonitorCap(ctx, chain.IssueMonitorCapRequest{
		Recipient: recipient,
		DID:       provision.DID,
		ProjectID: projectID,
		PlotID:    plotID,
	})
	if err != nil {
		slog.Warn("IssueMonitorCapForUser chain error", "userID", userID, "error", err)
		return
	}
	if linkErr := s.chain.LinkRoleObject(ctx, provision.DIDDocumentRef, capRes.CapObjectID); linkErr != nil {
		slog.Warn("IssueMonitorCapForUser LinkRoleObject", "userID", userID, "error", linkErr)
	}
	if err := s.db.Model(&models.IdentityProvision{}).Where("id = ?", provision.ID).Update("role_cap_object_id", capRes.CapObjectID).Error; err != nil {
		slog.Warn("failed to persist monitor role_cap_object_id",
			"userID", userID, "capObjectID", capRes.CapObjectID, "error", err)
	}
	provision.RoleCapObjectID = capRes.CapObjectID
	s.SyncToHub(ctx, userID, "monitor", provision)
}

// SyncToHub pushes the user's identity and capability to the baharilink hub.
// Fire-and-forget: errors are silently discarded.
func (s *Service) SyncToHub(ctx context.Context, userID string, role string, provision models.IdentityProvision) {
	if s.hubURL == "" {
		return
	}
	caps := []map[string]string{}
	if provision.RoleCapObjectID != "" {
		capType := mrvRoleToCapType(role)
		if capType != "" {
			caps = append(caps, map[string]string{"type": capType, "objectId": provision.RoleCapObjectID})
		}
	}
	payload := map[string]any{
		"sourceModule":   "baharimrv",
		"externalUserId": userID,
		"role":           role,
		"did":            provision.DID,
		"address":        provision.Address,
		"capabilities":   caps,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.hubURL+"/identity/register", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if s.hubAPIKey != "" {
		req.Header.Set("x-api-key", s.hubAPIKey)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

func mrvRoleToCapType(role string) string {
	switch role {
	case "manager":
		return "ManagerCap"
	case "verifier":
		return "VerifierCap"
	case "monitor":
		return "MonitorCap"
	default:
		return ""
	}
}

// ResolveRoleFromHub calls the baharilink hub to get cross-platform capabilities
// and maps them to the BahariMRV local role. Returns "" if not configured or
// no role can be inferred.
func (s *Service) ResolveRoleFromHub(ctx context.Context, userID string, did string) string {
	if s.hubURL == "" {
		return ""
	}
	for _, src := range []string{"nanga", "jahazi"} {
		role := s.mrvResolveFromSource(ctx, src, userID, did)
		if role != "" {
			return role
		}
	}
	return ""
}

func (s *Service) mrvResolveFromSource(ctx context.Context, sourceModule, userID, did string) string {
	payload := map[string]any{"sourceModule": sourceModule, "externalUserId": userID, "did": did}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.hubURL+"/identity/resolve", bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	if s.hubAPIKey != "" {
		req.Header.Set("x-api-key", s.hubAPIKey)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var result struct {
		Identity struct {
			Capabilities []mrvHubCap `json:"capabilities"`
		} `json:"identity"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result); err != nil {
		return ""
	}
	return MapCapsToMRVRole(result.Identity.Capabilities)
}

type mrvHubCap struct {
	Type     string `json:"type"`
	ObjectID string `json:"objectId"`
}

// MapCapsToMRVRole maps hub capabilities to the BahariMRV local role.
// Exported for testability.
func MapCapsToMRVRole(caps []mrvHubCap) string {
	for _, c := range caps {
		switch c.Type {
		case "ManagerCap":
			return "project_manager"
		case "VerifierCap":
			return "independent_verifier"
		case "OfficerCap":
			return "independent_verifier"
		case "FisherCap":
			return "field_monitor"
		case "MonitorCap":
			return "field_monitor"
		}
	}
	return ""
}
