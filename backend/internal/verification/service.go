// Package verification runs the independent third-party audit stage. Only a
// verifier with an active provisioned identity can sign a decision, and approval
// anchors a separate achievement_verified commitment.
package verification

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/auth"
	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	StepUpAction = "verification_decision"
)

var (
	ErrForbidden = errors.New("this decision belongs to the assigned verifier")
	ErrNotFound  = errors.New("verification request not found")
	ErrClosed    = errors.New("this request has already been decided")
)

type Service struct {
	db         *gorm.DB
	identities *identity.Service
	provenance *provenance.Service
	mrv        *mrv.Service
	stepUp     *auth.StepUp
	now        func() time.Time
}

func NewService(db *gorm.DB, identities *identity.Service, anchors *provenance.Service, projects *mrv.Service, stepUp *auth.StepUp) *Service {
	return &Service{db: db, identities: identities, provenance: anchors, mrv: projects, stepUp: stepUp, now: time.Now}
}

type Input struct {
	Decision           string `json:"decision"`
	Notes              string `json:"notes"`
	MethodologyVersion string `json:"methodologyVersion"`
	StepUpCode         string `json:"stepUpCode"`
	IdempotencyKey     string `json:"idempotencyKey"`
}

type Decision struct {
	Status               string `json:"status"`
	AttestationReference string `json:"attestationReference,omitempty"`
	ProvenanceStatus     string `json:"provenanceStatus,omitempty"`
	ExplorerURL          string `json:"explorerUrl,omitempty"`
}

// Queue lists the requests awaiting this verifier.
type QueueItem struct {
	ID                 string    `json:"id"`
	ActivityID         string    `json:"activityId"`
	ProjectName        string    `json:"projectName"`
	PlotCode           string    `json:"plotCode"`
	Status             string    `json:"status"`
	RequestedAt        time.Time `json:"requestedAt"`
	MethodologyVersion string    `json:"methodologyVersion"`
	EvidenceCount      int       `json:"evidenceCount"`
	RecordsCount       int       `json:"recordsCount"`
	UnsignedRecords    int       `json:"unsignedRecords"`
}

func (s *Service) Queue(verifierID string) ([]QueueItem, error) {
	var requests []models.VerificationRequest
	if err := s.db.Where("verifier_id = ? AND status IN ?", verifierID, []string{"requested", "in_review"}).
		Order("requested_at asc").Find(&requests).Error; err != nil {
		return nil, err
	}
	items := make([]QueueItem, 0, len(requests))
	for _, request := range requests {
		activity, project, err := s.mrv.Activity(request.ActivityID)
		if err != nil {
			continue
		}
		records, err := s.mrv.RecordsFor(activity.ID)
		if err != nil {
			return nil, err
		}
		var evidenceCount int64
		if len(records) > 0 {
			ids := make([]string, 0, len(records))
			unsigned := 0
			for _, record := range records {
				ids = append(ids, record.ID)
				if !record.Signed {
					unsigned++
				}
			}
			if err := s.db.Model(&models.EvidenceObject{}).Where("monitoring_record_id IN ?", ids).Count(&evidenceCount).Error; err != nil {
				return nil, err
			}
			items = append(items, QueueItem{
				ID: request.ID, ActivityID: activity.ID, ProjectName: project.Name, PlotCode: plotCode(s.db, activity.PlotID),
				Status: request.Status, RequestedAt: request.RequestedAt, MethodologyVersion: activity.MethodologyVersion,
				EvidenceCount: int(evidenceCount), RecordsCount: len(records), UnsignedRecords: unsigned,
			})
			continue
		}
		items = append(items, QueueItem{
			ID: request.ID, ActivityID: activity.ID, ProjectName: project.Name, PlotCode: plotCode(s.db, activity.PlotID),
			Status: request.Status, RequestedAt: request.RequestedAt, MethodologyVersion: activity.MethodologyVersion,
		})
	}
	return items, nil
}

// Decide records the verifier's signed decision.
func (s *Service) Decide(ctx context.Context, requestID, verifierID string, in Input) (Decision, error) {
	var request models.VerificationRequest
	if err := s.db.Where("id = ?", requestID).First(&request).Error; err != nil {
		return Decision{}, ErrNotFound
	}
	if request.VerifierID != verifierID {
		return Decision{}, ErrForbidden
	}
	// The replay is checked before the closed state so a retried request returns
	// the decision it already produced instead of a conflict.
	if in.IdempotencyKey != "" {
		if replay, ok := s.replay(verifierID, in.IdempotencyKey); ok {
			return replay, nil
		}
	}
	if request.Status != "requested" && request.Status != "in_review" {
		return Decision{}, ErrClosed
	}
	switch in.Decision {
	case models.DecisionApprove, models.DecisionReject, models.DecisionRequestCorrections:
	default:
		return Decision{}, domain.Invalid("the decision must be approve, reject or request_corrections")
	}
	if strings.TrimSpace(in.Notes) == "" {
		return Decision{}, domain.Invalid("a written rationale is required with every decision")
	}
	if in.IdempotencyKey == "" {
		return Decision{}, domain.Invalid("an idempotency key is required")
	}
	if err := s.stepUp.Consume(verifierID, in.StepUpCode, StepUpAction); err != nil {
		return Decision{}, err
	}
	activity, _, err := s.mrv.Activity(request.ActivityID)
	if err != nil {
		return Decision{}, err
	}
	if activity.MethodologyVersion != in.MethodologyVersion {
		return Decision{}, domain.Reject("the methodology version you are deciding against is no longer the one registered on this activity")
	}
	records, err := s.mrv.RecordsFor(activity.ID)
	if err != nil {
		return Decision{}, err
	}
	if len(records) == 0 {
		return Decision{}, domain.Reject("there is nothing to verify for this activity")
	}
	gap, err := s.mrv.EvidenceGap(records)
	if err != nil {
		return Decision{}, err
	}

	provision, err := s.identities.ForUser(verifierID)
	if err != nil || provision.DID == "" || provision.State != models.IdentityActive {
		return Decision{}, domain.Reject("an active provisioned identity is required to sign a decision")
	}

	now := s.now().UTC()
	attestation := models.VerifierAttestation{
		ID: uuid.NewString(), RequestID: request.ID, ActivityID: activity.ID, VerifierID: verifierID,
		IssuerDID: provision.DID, Decision: in.Decision, Statement: strings.TrimSpace(in.Notes),
		AttestationHash: attestationHash(request, activity, in, gap, now), CreatedAt: now,
	}

	switch in.Decision {
	case models.DecisionRequestCorrections:
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&attestation).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.VerificationRequest{}).Where("id = ?", request.ID).
				Updates(map[string]any{"status": "corrections_requested", "decided_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			previous, err := s.completionEvent(activity.ID)
			if err != nil {
				return err
			}
			correction := models.Correction{
				ID: uuid.NewString(), ActivityID: activity.ID, Reason: strings.TrimSpace(in.Notes),
				RequestedBy: verifierID, CreatedAt: now,
			}
			if previous != nil {
				correction.PreviousEventID = previous.ID
			}
			return tx.Create(&correction).Error
		}); err != nil {
			return Decision{}, err
		}
		if err := s.persistIdempotency(ctx, verifierID, in.IdempotencyKey, Decision{Status: "corrections_requested", AttestationReference: attestation.ID}); err != nil {
			return Decision{}, err
		}
		return Decision{Status: "corrections_requested", AttestationReference: attestation.ID}, nil

	case models.DecisionReject:
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&attestation).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.VerificationRequest{}).Where("id = ?", request.ID).
				Updates(map[string]any{"status": "rejected", "decided_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			return tx.Model(&models.ConservationActivity{}).Where("id = ?", activity.ID).
				Updates(map[string]any{"status": models.ActivityRejected, "updated_at": now}).Error
		}); err != nil {
			return Decision{}, err
		}
		if err := s.persistIdempotency(ctx, verifierID, in.IdempotencyKey, Decision{Status: "rejected", AttestationReference: attestation.ID}); err != nil {
			return Decision{}, err
		}
		return Decision{Status: "rejected", AttestationReference: attestation.ID}, nil
	}

	// Approval: the verified achievement is its own commitment, linked to the
	// earlier completion claim when one exists.
	completion, err := s.completionEvent(activity.ID)
	if err != nil {
		return Decision{}, err
	}
	verifiedEvent, err := s.provenance.Record(ctx, provenance.Input{
		AchievementID: activity.ID, ProjectID: activity.ProjectID, ClaimType: models.ClaimVerified,
		ActivityType: activity.ActivityType, MethodologyVersion: activity.MethodologyVersion,
		ReportingPeriodStart: activity.ReportingPeriodStart, ReportingPeriodEnd: activity.ReportingPeriodEnd,
		Measurements: mrv.Summarise(records, activity), MeasurementBasis: "independently_approved",
		EvidenceRoot: mrv.EvidenceRoot(records), SatelliteHash: s.satelliteHash(activity.ID),
		VerifierAttestationHash: attestation.AttestationHash, VerifierDID: provision.DID,
		IssuerDID: provision.DID, PreviousEventID: previousEventID(completion),
	})
	if err != nil {
		return Decision{}, err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&attestation).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.VerificationRequest{}).Where("id = ?", request.ID).
			Updates(map[string]any{"status": "approved", "decided_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&models.ConservationActivity{}).Where("id = ?", activity.ID).Updates(map[string]any{
			"status": models.ActivityVerified, "current_event_id": verifiedEvent.ID, "updated_at": now,
		}).Error
	}); err != nil {
		return Decision{}, err
	}
	decision := Decision{
		Status: "approved", AttestationReference: attestation.ID, ProvenanceStatus: verifiedEvent.Status,
		ExplorerURL: s.provenance.ExplorerTransaction(verifiedEvent),
	}
	if err := s.persistIdempotency(ctx, verifierID, in.IdempotencyKey, decision); err != nil {
		return Decision{}, err
	}
	return decision, nil
}

// Assign lets a project manager or administrator set the independent verifier.
func (s *Service) Assign(ctx context.Context, activityID, managerID, verifierID string) (models.VerificationRequest, error) {
	activity, project, err := s.mrv.Activity(activityID)
	if err != nil {
		return models.VerificationRequest{}, err
	}
	var manager models.User
	if err := s.db.Where("id = ?", managerID).First(&manager).Error; err != nil {
		return models.VerificationRequest{}, ErrForbidden
	}
	if manager.Role != models.RoleAdmin && (manager.Role != models.RoleManager || project.ManagerID != managerID) {
		return models.VerificationRequest{}, ErrForbidden
	}
	if verifierID == managerID {
		return models.VerificationRequest{}, domain.Invalid("the verifier must be independent of the project manager")
	}
	var verifier models.User
	if err := s.db.Where("id = ? AND role = ? AND status = ?", verifierID, models.RoleVerifier, models.IdentityActive).First(&verifier).Error; err != nil {
		return models.VerificationRequest{}, domain.Invalid("the verifier is not an active independent verifier")
	}
	// Completion already opens a request, and a manager may resubmit the same
	// assignment. One activity holds at most one open request: a second row would
	// let two independent decisions stand against the same evidence.
	var open models.VerificationRequest
	lookup := s.db.Where("activity_id = ? AND status IN ?", activityID, []string{"requested", "in_review"}).
		Order("requested_at asc").First(&open).Error
	if lookup != nil && !errors.Is(lookup, gorm.ErrRecordNotFound) {
		return models.VerificationRequest{}, lookup
	}
	if open.ID != "" {
		if open.VerifierID != verifier.ID {
			return models.VerificationRequest{}, domain.Reject("this activity already has an open verification request with another verifier")
		}
		return open, nil
	}
	now := s.now().UTC()
	request := models.VerificationRequest{
		ID: uuid.NewString(), ActivityID: activity.ID, VerifierID: verifier.ID, Status: "requested",
		RequestedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.Create(&request).Error; err != nil {
		return request, err
	}
	return request, nil
}

// Dispute opens a dispute over an activity and marks its events disputed.
func (s *Service) Dispute(activityID, userID, reason string) (models.Dispute, error) {
	if strings.TrimSpace(reason) == "" {
		return models.Dispute{}, domain.Invalid("a dispute needs a reason")
	}
	if err := s.mrv.AssertProjectAccess(userID, activityIDProject(s.db, activityID)); err != nil {
		return models.Dispute{}, err
	}
	now := s.now().UTC()
	dispute := models.Dispute{
		ID: uuid.NewString(), ActivityID: activityID, Status: "open",
		Reason: strings.TrimSpace(reason), CreatedAt: now,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&dispute).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ProvenanceEvent{}).Where("activity_id = ? AND status <> ?", activityID, provenance.StatusSuperseded).
			Update("status", provenance.StatusDisputed).Error; err != nil {
			return err
		}
		return tx.Model(&models.ConservationActivity{}).Where("id = ?", activityID).
			Update("status", models.ActivityDisputed).Error
	})
	return dispute, err
}

func (s *Service) satelliteHash(activityID string) string {
	summary, ok := s.mrv.LatestCorroboration(activityID)
	if !ok {
		return ""
	}
	return summary.ResultHash
}

func (s *Service) completionEvent(activityID string) (*models.ProvenanceEvent, error) {
	var event models.ProvenanceEvent
	err := s.db.Where("activity_id = ? AND claim_type = ?", activityID, models.ClaimCompletion).
		Order("created_at desc").First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *Service) persistIdempotency(ctx context.Context, userID, key string, decision Decision) error {
	hash, _, err := canon.Hash(map[string]any{"status": decision.Status, "attestation": decision.AttestationReference})
	if err != nil {
		return err
	}
	response, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	return s.db.WithContext(ctx).Create(&models.SyncAction{
		ID: uuid.NewString(), UserID: userID, ClientID: key, ActionType: "verification_decision",
		RequestHash: hash, Status: decision.Status, Response: datatypes.JSON(response),
		CreatedAt: now, UpdatedAt: now,
	}).Error
}

func (s *Service) replay(userID, key string) (Decision, bool) {
	var action models.SyncAction
	if err := s.db.Where("user_id = ? AND client_id = ? AND action_type = ?", userID, key, "verification_decision").
		First(&action).Error; err != nil {
		return Decision{}, false
	}
	var decision Decision
	if err := json.Unmarshal(action.Response, &decision); err != nil {
		return Decision{}, false
	}
	return decision, true
}

func attestationHash(request models.VerificationRequest, activity models.ConservationActivity, in Input, gap mrv.EvidenceGap, at time.Time) string {
	hash, _, err := canon.Hash(map[string]any{
		"requestId": request.ID, "activityId": activity.ID, "decision": in.Decision,
		"notes": strings.TrimSpace(in.Notes), "methodologyVersion": in.MethodologyVersion,
		"declaredEvidence": gap.Declared, "storedEvidence": len(gap.Stored),
		"decidedAt": at.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return ""
	}
	return hash
}

func previousEventID(event *models.ProvenanceEvent) string {
	if event == nil {
		return ""
	}
	return event.ID
}

func plotCode(db *gorm.DB, plotID string) string {
	var plot models.Plot
	if err := db.Where("id = ?", plotID).First(&plot).Error; err != nil {
		return ""
	}
	return plot.Code
}

func activityIDProject(db *gorm.DB, activityID string) string {
	var activity models.ConservationActivity
	if err := db.Where("id = ?", activityID).First(&activity).Error; err != nil {
		return ""
	}
	return activity.ProjectID
}

// Handler exposes the decision endpoint.
type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Decision(c *gin.Context) {
	var input Input
	if !httpx.BindJSON(c, &input) {
		return
	}
	decision, err := h.service.Decide(c.Request.Context(), c.Param("id"), c.GetString("user_id"), input)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, decision)
	case errors.Is(err, auth.ErrStepUpRequired):
		httpx.Error(c, http.StatusForbidden, "step_up_required", "Recent password confirmation is required to sign a decision.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(c, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, ErrForbidden):
		httpx.Error(c, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, ErrClosed):
		httpx.Error(c, http.StatusConflict, "already_decided", err.Error())
	default:
		var rejection *domain.Error
		if errors.As(err, &rejection) {
			httpx.Error(c, rejection.Kind.Status, rejection.Code, rejection.Message)
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "The decision could not be recorded.")
	}
}

func (h *Handler) Queue(c *gin.Context) {
	items, err := h.service.Queue(c.GetString("user_id"))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "queue_unavailable", "The verification queue could not be loaded.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
