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
	now        func() time.Time
}

type Result struct {
	User         models.User              `json:"user"`
	Identity     models.IdentityProvision `json:"identity"`
	SessionToken string                   `json:"sessionToken,omitempty"`
	ExpiresAt    *time.Time               `json:"expiresAt,omitempty"`
}

func NewService(db *gorm.DB, identities *identity.Service, sessionTTL time.Duration) *Service {
	return &Service{db: db, identity: identities, sessionTTL: sessionTTL, now: time.Now}
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
	// Issue role capability on-chain (non-blocking: runs in background goroutine,
	// registration succeeds even if cap issuance is temporarily unavailable).
	go s.identity.IssueCapForUser(context.Background(), user.ID, role, provision)
	user.Status = models.IdentityActive
	result.User = user
	token, expires, sessionErr := s.NewSession(user.ID)
	if sessionErr != nil {
		return result, sessionErr
	}
	result.SessionToken, result.ExpiresAt = token, &expires
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
	if err != nil || provision.State != models.IdentityActive || user.Status != models.IdentityActive {
		return Result{User: user, Identity: provision}, identity.ErrProvisioning
	}
	token, expires, err := s.NewSession(user.ID)
	if err != nil {
		return Result{}, err
	}
	return Result{User: user, Identity: provision, SessionToken: token, ExpiresAt: &expires}, nil
}

func (s *Service) NewSession(userID string) (string, time.Time, error) {
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
	if err := s.db.Create(&session).Error; err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
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
