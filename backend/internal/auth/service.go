package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	db         *gorm.DB
	identity   *identity.Service
	sessionTTL time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

type Result struct {
	User              models.User              `json:"user"`
	Identity          models.IdentityProvision `json:"identity"`
	SessionToken      string                   `json:"sessionToken,omitempty"`
	ExpiresAt         *time.Time               `json:"expiresAt,omitempty"`
	RefreshToken      string                   `json:"refreshToken,omitempty"`
	RefreshExpiresAt  *time.Time               `json:"refreshExpiresAt,omitempty"`
	CapIssuanceFailed bool                     `json:"capIssuanceFailed,omitempty"`
}

func NewService(db *gorm.DB, identities *identity.Service, sessionTTL, refreshTTL time.Duration) *Service {
	return &Service{db: db, identity: identities, sessionTTL: sessionTTL, refreshTTL: refreshTTL, now: time.Now}
}

func (s *Service) Register(ctx context.Context, email, password, displayName, phone, role string) (Result, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, "@") {
		return Result{}, fmt.Errorf("valid email required")
	}
	if len(password) < 12 || len(password) > 128 {
		return Result{}, fmt.Errorf("password must contain 12 to 128 characters")
	}
	if strings.TrimSpace(displayName) == "" || len(displayName) > 120 {
		return Result{}, fmt.Errorf("display name is required")
	}
	switch role {
	case models.RoleManager, models.RoleMonitor, models.RoleVerifier, models.RoleBuyer:
	default:
		return Result{}, fmt.Errorf("registration role is not allowed")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return Result{}, err
	}
	now := s.now().UTC()
	user := models.User{
		ID: uuid.NewString(), Email: email, PasswordHash: string(hash),
		DisplayName: strings.TrimSpace(displayName), Phone: strings.TrimSpace(phone),
		Role: role, Status: models.IdentityPending, CreatedAt: now, UpdatedAt: now,
	}
	provision := models.IdentityProvision{
		ID: uuid.NewString(), UserID: user.ID, State: models.IdentityPending,
		Network: s.identity.Network(), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return tx.Create(&provision).Error
	}); err != nil {
		return Result{}, fmt.Errorf("registration could not be completed")
	}
	provision, err = s.identity.EnsureProvision(ctx, user.ID)
	result := Result{User: user, Identity: provision}
	if err != nil {
		return result, err
	}
	// Issue role capability on-chain synchronously. Returns false if cap issuance
	// fails; registration records the identity but does not create a session.
	capOK := s.identity.IssueCapForUser(ctx, user.ID, role, provision)

	// Reload provision so result.Identity carries the freshly written role_cap_object_id.
	if reloaded, reloadErr := s.identity.ForUser(user.ID); reloadErr == nil {
		provision = reloaded
	}
	result.Identity = provision

	if !capOK {
		result.CapIssuanceFailed = true
		return result, nil // no session token; client receives 202
	}

	user.Status = models.IdentityActive
	result.User = user
	token, expires, sessionErr := s.NewSession(user.ID)
	if sessionErr != nil {
		return result, sessionErr
	}
	result.SessionToken, result.ExpiresAt = token, &expires
	refresh, refreshExpires, refreshErr := s.IssueRefresh(user.ID)
	if refreshErr != nil {
		return result, refreshErr
	}
	result.RefreshToken, result.RefreshExpiresAt = refresh, &refreshExpires
	return result, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (Result, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return Result{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return Result{}, ErrInvalidCredentials
	}
	provision, err := s.identity.ForUser(user.ID)
	if err != nil {
		return Result{User: user, Identity: provision}, identity.ErrProvisioning
	}
	// Auto-retry provisioning for identity_failed accounts — the most common
	// failure cause (EDuplicateUserId race or finality timeout) is recoverable.
	// This lets a user log in and trigger recovery without a separate retry call.
	if provision.State == models.IdentityFailed {
		if retried, retryErr := s.identity.EnsureProvision(ctx, user.ID); retryErr == nil {
			provision = retried
		}
	}
	if provision.State != models.IdentityActive || user.Status != models.IdentityActive {
		return Result{User: user, Identity: provision}, identity.ErrProvisioning
	}
	// Cross-platform role healing: if the user has no cap yet, check the hub
	// for a role established on another platform (non-blocking). The outer
	// goroutine keeps login fast; IssueCapForUser runs synchronously within it
	// so the cap write does not race itself when the goroutine is already off
	// the hot path.
	if provision.RoleCapObjectID == "" {
		go func() {
			bgCtx := context.Background()
			crossRole := s.identity.ResolveRoleFromHub(bgCtx, user.ID, provision.DID)
			if crossRole != "" && crossRole != user.Role {
				s.db.Model(&models.User{}).Where("id = ?", user.ID).Update("role", crossRole)
				s.identity.IssueCapForUser(bgCtx, user.ID, crossRole, provision)
			}
		}()
	}
	token, expires, err := s.NewSession(user.ID)
	if err != nil {
		return Result{}, err
	}
	refresh, refreshExpires, err := s.IssueRefresh(user.ID)
	if err != nil {
		return Result{}, err
	}
	return Result{
		User: user, Identity: provision, SessionToken: token, ExpiresAt: &expires,
		RefreshToken: refresh, RefreshExpiresAt: &refreshExpires,
	}, nil
}

func (s *Service) NewSession(userID string) (string, time.Time, error) {
	return s.newSessionRow(s.db, userID)
}

func (s *Service) newSessionRow(tx *gorm.DB, userID string) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now().UTC()
	expires := now.Add(s.sessionTTL)
	session := models.LocalSession{
		ID: uuid.NewString(), UserID: userID, TokenHash: tokenHash(token),
		ExpiresAt: expires, CreatedAt: now,
	}
	if err := tx.Create(&session).Error; err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// IssueRefresh mints the long-lived half of a session. Only the hash is stored.
func (s *Service) IssueRefresh(userID string) (string, time.Time, error) {
	return s.newRefreshRow(s.db, userID)
}

func (s *Service) newRefreshRow(tx *gorm.DB, userID string) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now().UTC()
	expires := now.Add(s.refreshTTL)
	grant := models.RefreshGrant{
		ID: uuid.NewString(), UserID: userID, TokenHash: tokenHash(token),
		ExpiresAt: expires, CreatedAt: now,
	}
	if err := tx.Create(&grant).Error; err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// Refresh rotates a refresh grant into a fresh session plus a fresh grant.
// A revoked grant presented again is treated as theft: the whole family dies.
func (s *Service) Refresh(raw string) (Result, error) {
	if strings.TrimSpace(raw) == "" {
		return Result{}, ErrInvalidCredentials
	}
	now := s.now().UTC()
	var grant models.RefreshGrant
	if err := s.db.Where("token_hash = ?", tokenHash(raw)).First(&grant).Error; err != nil {
		return Result{}, ErrInvalidCredentials
	}
	if grant.RevokedAt != nil {
		s.db.Model(&models.RefreshGrant{}).
			Where("user_id = ? AND revoked_at IS NULL", grant.UserID).
			Update("revoked_at", now)
		return Result{}, ErrInvalidCredentials
	}
	if !now.Before(grant.ExpiresAt) {
		return Result{}, ErrInvalidCredentials
	}
	var user models.User
	if err := s.db.Where("id = ? AND status = ?", grant.UserID, models.IdentityActive).First(&user).Error; err != nil {
		return Result{}, ErrInvalidCredentials
	}
	var token, refresh string
	var expires, refreshExpires time.Time
	err := s.db.Transaction(func(tx *gorm.DB) error {
		rotated := tx.Model(&models.RefreshGrant{}).
			Where("id = ? AND revoked_at IS NULL", grant.ID).
			Update("revoked_at", now)
		if rotated.Error != nil {
			return rotated.Error
		}
		if rotated.RowsAffected != 1 {
			return ErrInvalidCredentials
		}
		var err error
		token, expires, err = s.newSessionRow(tx, user.ID)
		if err != nil {
			return err
		}
		refresh, refreshExpires, err = s.newRefreshRow(tx, user.ID)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	provision, _ := s.identity.ForUser(user.ID)
	return Result{
		User: user, Identity: provision, SessionToken: token, ExpiresAt: &expires,
		RefreshToken: refresh, RefreshExpiresAt: &refreshExpires,
	}, nil
}

func (s *Service) Authenticate(token string) (models.User, error) {
	var session models.LocalSession
	now := s.now().UTC()
	if err := s.db.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash(token), now).First(&session).Error; err != nil {
		return models.User{}, ErrInvalidCredentials
	}
	var user models.User
	if err := s.db.Where("id = ? AND status = ?", session.UserID, models.IdentityActive).First(&user).Error; err != nil {
		return models.User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Service) Logout(token string) error {
	now := s.now().UTC()
	var session models.LocalSession
	if err := s.db.Where("token_hash = ?", tokenHash(token)).First(&session).Error; err == nil {
		if err := s.db.Model(&models.RefreshGrant{}).
			Where("user_id = ? AND revoked_at IS NULL", session.UserID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
	}
	return s.db.Model(&models.LocalSession{}).Where("token_hash = ? AND revoked_at IS NULL", tokenHash(token)).Update("revoked_at", now).Error
}

func (s *Service) CheckPassword(userID, password string) bool {
	var user models.User
	if err := s.db.Select("id,password_hash").Where("id = ?", userID).First(&user).Error; err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
}

func tokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
