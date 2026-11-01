package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// stepUpTTL is deliberately short: the ticket only covers the sensitive request
// the user was just prompted for.
const stepUpTTL = 5 * time.Minute

var ErrStepUpRequired = errors.New("recent confirmation required")

type StepUp struct {
	db       *gorm.DB
	sessions *Service
	now      func() time.Time
}

func NewStepUp(db *gorm.DB, sessions *Service) *StepUp {
	return &StepUp{db: db, sessions: sessions, now: time.Now}
}

type StepUpResponse struct {
	StepUpCode string    `json:"stepUpCode"`
	Action     string    `json:"action"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// Issue verifies the password again and mints a single-use ticket for one action.
func (s *StepUp) Issue(userID, password, action string) (StepUpResponse, error) {
	if action == "" {
		return StepUpResponse{}, fmt.Errorf("the confirmation target is required")
	}
	if !s.sessions.CheckPassword(userID, password) {
		return StepUpResponse{}, ErrInvalidCredentials
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return StepUpResponse{}, err
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now().UTC()
	token := models.StepUpToken{
		ID: uuid.NewString(), UserID: userID, TokenHash: tokenHash(code), Action: action,
		ExpiresAt: now.Add(stepUpTTL), CreatedAt: now,
	}
	if err := s.db.Create(&token).Error; err != nil {
		return StepUpResponse{}, fmt.Errorf("the confirmation could not be recorded")
	}
	return StepUpResponse{StepUpCode: code, Action: action, ExpiresAt: token.ExpiresAt}, nil
}

// Consume marks the ticket used. A missing, expired, wrong-action or already used
// ticket all fail identically so the response cannot be probed.
func (s *StepUp) Consume(userID, code, action string) error {
	if code == "" {
		return ErrStepUpRequired
	}
	now := s.now().UTC()
	var token models.StepUpToken
	err := s.db.Where("token_hash = ? AND user_id = ? AND action = ? AND used_at IS NULL AND expires_at > ?",
		tokenHash(code), userID, action, now).First(&token).Error
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrStepUpRequired
	}
	if err != nil {
		return fmt.Errorf("the confirmation could not be checked")
	}
	claim := s.db.Model(&models.StepUpToken{}).Where("id = ? AND used_at IS NULL", token.ID).
		Update("used_at", now)
	if claim.Error != nil {
		return fmt.Errorf("the confirmation could not be consumed")
	}
	if claim.RowsAffected == 0 {
		return ErrStepUpRequired
	}
	return nil
}

func (h *Handler) StepUp(s *StepUp) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Password string `json:"password"`
			Action   string `json:"action"`
		}
		if !httpx.BindJSON(c, &request) {
			return
		}
		response, err := s.Issue(c.GetString("user_id"), request.Password, request.Action)
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.Error(c, http.StatusForbidden, "step_up_failed", "Your password did not match. Recent confirmation is required for this action.")
			return
		}
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, "step_up_failed", "The confirmation could not be issued.")
			return
		}
		c.JSON(http.StatusCreated, response)
	}
}
