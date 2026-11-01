package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrProvisioning = errors.New("IOTA identity provisioning is not active")

type Service struct {
	db      *gorm.DB
	chain   chain.Adapter
	custody *EnvelopeCustody
	now     func() time.Time
}

func NewService(db *gorm.DB, chainAdapter chain.Adapter, custody *EnvelopeCustody) *Service {
	return &Service{db: db, chain: chainAdapter, custody: custody, now: time.Now}
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
