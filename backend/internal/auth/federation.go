package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FederationService struct {
	db         *gorm.DB
	validator  ExternalValidator
	identities *identity.Service
	sessions   *Service
}

func NewFederationService(db *gorm.DB, validator ExternalValidator, identities *identity.Service, sessions *Service) *FederationService {
	return &FederationService{db: db, validator: validator, identities: identities, sessions: sessions}
}

func (s *FederationService) Login(ctx context.Context, rawIDToken, expectedNonce string) (Result, error) {
	claims, err := s.validator.Validate(ctx, rawIDToken, expectedNonce, "id")
	if err != nil {
		return Result{}, err
	}
	link, err := s.activeLink(claims.Issuer, claims.Subject)
	if err != nil {
		link, err = s.createFederatedAccount(ctx, claims)
		if err != nil {
			return Result{}, err
		}
	}
	var user models.User
	if err := s.db.Where("id = ? AND status = ?", link.UserID, models.IdentityActive).First(&user).Error; err != nil {
		return Result{}, identity.ErrProvisioning
	}
	provision, err := s.identities.ForUser(user.ID)
	if err != nil || provision.State != models.IdentityActive {
		return Result{}, identity.ErrProvisioning
	}
	token, expires, err := s.sessions.NewSession(user.ID)
	if err != nil {
		return Result{}, err
	}
	refresh, refreshExpires, err := s.sessions.IssueRefresh(user.ID)
	if err != nil {
		return Result{}, err
	}
	return Result{
		User: user, Identity: provision, SessionToken: token, ExpiresAt: &expires,
		RefreshToken: refresh, RefreshExpiresAt: &refreshExpires,
	}, nil
}

func (s *FederationService) createFederatedAccount(ctx context.Context, claims ExternalClaims) (models.ExternalIdentity, error) {
	now := time.Now().UTC()
	digest := sha256.Sum256([]byte(claims.Issuer + "\x00" + claims.Subject))
	name := strings.TrimSpace(claims.Name)
	if name == "" {
		name = "Federated BahariLink user"
	}
	user := models.User{
		ID: uuid.NewString(), Email: "fed-" + hex.EncodeToString(digest[:16]) + "@federated.invalid",
		PasswordHash: "!federated-login-only", DisplayName: name, Phone: claims.Phone,
		Role: models.RoleBuyer, Status: models.IdentityPending, CreatedAt: now, UpdatedAt: now,
	}
	provision := models.IdentityProvision{ID: uuid.NewString(), UserID: user.ID, State: models.IdentityPending, Network: s.identities.Network(), CreatedAt: now, UpdatedAt: now}
	link := models.ExternalIdentity{ID: uuid.NewString(), UserID: user.ID, Issuer: claims.Issuer, Subject: claims.Subject, CreatedAt: now, UpdatedAt: now}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if err := tx.Create(&provision).Error; err != nil {
			return err
		}
		return tx.Create(&link).Error
	}); err != nil {
		return link, fmt.Errorf("federated account creation failed")
	}
	if _, err := s.identities.EnsureProvision(ctx, user.ID); err != nil {
		return link, err
	}
	return link, nil
}

func (s *FederationService) Link(ctx context.Context, userID, rawIDToken, expectedNonce string) (models.ExternalIdentity, error) {
	claims, err := s.validator.Validate(ctx, rawIDToken, expectedNonce, "id")
	if err != nil {
		return models.ExternalIdentity{}, err
	}
	var link models.ExternalIdentity
	err = s.db.Where("issuer = ? AND subject = ?", claims.Issuer, claims.Subject).First(&link).Error
	if err == nil {
		if link.UserID != userID || link.RevokedAt == nil {
			return link, fmt.Errorf("external identity is already linked")
		}
		now := time.Now().UTC()
		if err := s.db.Model(&models.ExternalIdentity{}).Where("id = ?", link.ID).Updates(map[string]any{"revoked_at": nil, "updated_at": now}).Error; err != nil {
			return link, err
		}
		link.RevokedAt = nil
		return link, nil
	}
	now := time.Now().UTC()
	link = models.ExternalIdentity{ID: uuid.NewString(), UserID: userID, Issuer: claims.Issuer, Subject: claims.Subject, CreatedAt: now, UpdatedAt: now}
	if err := s.db.Create(&link).Error; err != nil {
		return link, fmt.Errorf("link could not be created")
	}
	return link, nil
}

func (s *FederationService) Unlink(userID, linkID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var link models.ExternalIdentity
		if err := tx.Where("id = ? AND user_id = ? AND revoked_at IS NULL", linkID, userID).First(&link).Error; err != nil {
			return fmt.Errorf("link not found")
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.ExternalIdentity{}).Where("id = ?", link.ID).Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&models.FederatedGrant{}).Where("external_identity_id = ? AND revoked_at IS NULL", link.ID).Update("revoked_at", now).Error
	})
}

func (s *FederationService) Links(userID string) ([]models.ExternalIdentity, error) {
	var links []models.ExternalIdentity
	err := s.db.Where("user_id = ? AND revoked_at IS NULL", userID).Find(&links).Error
	return links, err
}

func (s *FederationService) AuthorizeExternal(ctx context.Context, rawAccessToken, requiredScope string) (models.User, error) {
	claims, err := s.validator.Validate(ctx, rawAccessToken, "", "access")
	if err != nil {
		return models.User{}, err
	}
	if !hasScope(claims.Scope, requiredScope) {
		return models.User{}, fmt.Errorf("required scope missing")
	}
	link, err := s.activeLink(claims.Issuer, claims.Subject)
	if err != nil {
		return models.User{}, fmt.Errorf("active link required")
	}
	var user models.User
	if err := s.db.Where("id = ? AND status = ?", link.UserID, models.IdentityActive).First(&user).Error; err != nil {
		return user, err
	}
	var grant models.FederatedGrant
	err = s.db.Where("token_hash = ? AND external_identity_id = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash(rawAccessToken), link.ID, time.Now().UTC()).First(&grant).Error
	if err != nil {
		grant = models.FederatedGrant{ID: uuid.NewString(), ExternalIdentityID: link.ID, TokenHash: tokenHash(rawAccessToken), Scope: claims.Scope, ExpiresAt: claims.ExpiresAt, CreatedAt: time.Now().UTC()}
		if err := s.db.Create(&grant).Error; err != nil {
			return user, fmt.Errorf("grant unavailable")
		}
	}
	return user, nil
}

func (s *FederationService) activeLink(issuer, subject string) (models.ExternalIdentity, error) {
	var link models.ExternalIdentity
	if err := s.db.Where("issuer = ? AND subject = ? AND revoked_at IS NULL", issuer, subject).First(&link).Error; err != nil {
		return link, fmt.Errorf("active external identity link not found")
	}
	return link, nil
}

func hasScope(scopes, required string) bool {
	for _, scope := range strings.Fields(scopes) {
		if scope == required {
			return true
		}
	}
	return false
}
