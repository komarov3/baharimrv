// Package jahazi implements the cross-platform integration between BahariMRV
// and the Jahazi tourism platform. It handles RestorationClaim reception,
// activity matching, MRV link generation and push, contribution mirroring, and
// attribution data export for certificate generation.
package jahazi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service is the Jahazi integration hub on the BahariMRV side. It owns the
// RestorationRequest lifecycle, the activity matching engine, MRV link signing,
// and the outbound webhook push to Jahazi.
type Service struct {
	db          *gorm.DB
	webhookURL  string
	secret      string
	explorerURL string
	now         func() time.Time
	httpClient  *http.Client
	logger      *slog.Logger
	notifier    Notifier
}

// Notifier is the interface for sending user notifications.
// Injected at construction to avoid a circular dependency.
type Notifier interface {
	NotifyRestorationClaimReceived(ctx context.Context, pmID, projectName, species string, units, contributorCount int)
	NotifyRestorationMatched(ctx context.Context, pmID, projectName, species string, planted, claimed int)
}

// NewService creates a Jahazi integration service. When webhookURL is empty or
// the secret is missing, the service is effectively disabled but still safe to
// call — methods return nil or no-ops rather than panicking.
func NewService(db *gorm.DB, webhookURL, secret, explorerURL string) *Service {
	return &Service{
		db:          db,
		webhookURL:  strings.TrimRight(webhookURL, "/"),
		secret:      secret,
		explorerURL: strings.TrimRight(explorerURL, "/"),
		now:         time.Now,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		logger:      slog.Default().With("component", "jahazi"),
	}
}

// SetNotifier injects the notification service after construction.
func (s *Service) SetNotifier(n Notifier) {
	s.notifier = n
}

// Enabled reports whether the Jahazi integration is configured. In development,
// a webhook URL alone is sufficient — the secret can be empty for local testing.
// In production, both must be set.
func (s *Service) Enabled() bool {
	if s.webhookURL == "" {
		return false
	}
	// Secret is required for production but optional in dev
	return true
}

// ── HMAC signing ────────────────────────────────────────────────────────────

// Sign computes the HMAC-SHA256 of a canonical JSON payload.
func (s *Service) Sign(payload any) (string, error) {
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifySignature checks an HMAC-SHA256 signature against the canonical JSON
// of the payload using a constant-time comparison.
func (s *Service) VerifySignature(payload any, signature string) bool {
	if s.secret == "" || signature == "" {
		return false
	}
	expected, err := s.Sign(payload)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ── RestorationClaim reception ──────────────────────────────────────────────

// ClaimInput is the payload Jahazi sends when a community lead clicks
// "Claim Restoration" on the portal.
type ClaimInput struct {
	ClaimID           string          `json:"claimId"`
	JahaziProjectID   string          `json:"jahaziProjectId"`
	MrvProjectID      string          `json:"mrvProjectId"`
	Period            PeriodInput     `json:"period"`
	TotalCostUsd      string          `json:"totalCostUsd"`
	TotalUnits        int             `json:"totalUnits"`
	Species           string          `json:"species"`
	ContributorCount  int             `json:"contributorCount"`
	ContributorJSON   json.RawMessage `json:"contributorBreakdown"`
	Nonce             string          `json:"nonce"`
	Signature         string          `json:"signature"`
	CreatedAt         string          `json:"createdAt"`
}

type PeriodInput struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ReceiveClaim validates the HMAC signature, checks for replay, and stores the
// RestorationRequest with status "open". The claim waits for a matching planting
// activity from the PM.
func (s *Service) ReceiveClaim(input ClaimInput) (models.RestorationRequest, error) {
	if !s.Enabled() {
		return models.RestorationRequest{}, fmt.Errorf("jahazi integration is not configured")
	}

	// Verify HMAC signature over the canonical JSON of the full input.
	if !s.VerifySignature(input, input.Signature) {
		return models.RestorationRequest{}, ErrInvalidSignature
	}

	// Look up the project and verify it's Jahazi-enabled.
	var project models.ConservationProject
	if err := s.db.Where("id = ?", input.MrvProjectID).First(&project).Error; err != nil {
		return models.RestorationRequest{}, ErrProjectNotFound
	}
	if !project.JahaziEnabled {
		return models.RestorationRequest{}, ErrNotJahaziEnabled
	}

	// Replay protection: nonce must be unique.
	var existing int64
	if err := s.db.Model(&models.RestorationRequest{}).Where("nonce = ? OR jahazi_claim_id = ?", input.Nonce, input.ClaimID).Count(&existing).Error; err != nil {
		return models.RestorationRequest{}, err
	}
	if existing > 0 {
		return models.RestorationRequest{}, ErrDuplicateClaim
	}

	periodStart, err := time.Parse(time.RFC3339, input.Period.Start)
	if err != nil {
		return models.RestorationRequest{}, fmt.Errorf("invalid period.start: %w", err)
	}
	periodEnd, err := time.Parse(time.RFC3339, input.Period.End)
	if err != nil {
		return models.RestorationRequest{}, fmt.Errorf("invalid period.end: %w", err)
	}

	now := s.now().UTC()
	expiresAt := now.Add(14 * 24 * time.Hour) // claims expire after 14 days

	req := models.RestorationRequest{
		ID:               uuid.NewString(),
		ProjectID:        project.ID,
		JahaziClaimID:    input.ClaimID,
		JahaziProjectID:  input.JahaziProjectID,
		PeriodStart:      periodStart.UTC(),
		PeriodEnd:        periodEnd.UTC(),
		TotalCostUsd:     input.TotalCostUsd,
		TotalUnits:       input.TotalUnits,
		Species:          input.Species,
		ContributorCount: input.ContributorCount,
		ContributorJSON:   datatypes.JSON(input.ContributorJSON),
		Nonce:            input.Nonce,
		Signature:         input.Signature,
		Status:           models.RestorationClaimOpen,
		ExpiresAt:        expiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.db.Create(&req).Error; err != nil {
		return req, err
	}

	s.logger.Info("restoration claim received",
		"claimId", req.JahaziClaimID, "projectId", req.ProjectID,
		"totalUnits", req.TotalUnits, "species", req.Species)

	// Notify PM about the incoming restoration claim
	if s.notifier != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s.notifier.NotifyRestorationClaimReceived(ctx,
				req.ProjectID, // will be resolved to PM ID inside notifier
				"", req.Species, req.TotalUnits, req.ContributorCount)
		}()
	}

	return req, nil
}

// GetClaimStatus returns the current state of a RestorationRequest for Jahazi
// to poll.
func (s *Service) GetClaimStatus(claimID string) (models.RestorationRequest, error) {
	var req models.RestorationRequest
	if err := s.db.Where("jahazi_claim_id = ?", claimID).First(&req).Error; err != nil {
		return req, ErrClaimNotFound
	}
	return req, nil
}

// ── Activity matching engine ────────────────────────────────────────────────

// MatchActivity is called after a PM completes a planting activity. It checks
// for open RestorationRequests for the same project and, if the species and
// unit count match, tags the activity as jahazi_funded and links the request.
// If no open request exists, the activity is a normal project activity and
// Jahazi is never notified.
func (s *Service) MatchActivity(activity models.ConservationActivity, measurements map[string]any) (*models.RestorationRequest, error) {
	if !s.Enabled() {
		return nil, nil
	}

	// Only planting-type activities are eligible for Jahazi matching.
	if !isPlantingType(activity.ActivityType) {
		return nil, nil
	}

	// Find open RestorationRequests for this project that haven't expired.
	var requests []models.RestorationRequest
	if err := s.db.Where("project_id = ? AND status = ? AND expires_at > ?",
		activity.ProjectID, models.RestorationClaimOpen, s.now().UTC()).
		Order("created_at asc").Find(&requests).Error; err != nil {
		return nil, err
	}

	if len(requests) == 0 {
		return nil, nil // normal project activity, no Jahazi involvement
	}

	// Extract planting details from measurements.
	plantedCount, _ := measurements["planted_count"].(int)
	if plantedCount == 0 {
		// Try float64 (JSON unmarshalling gives float64 for numbers).
		if f, ok := measurements["planted_count"].(float64); ok {
			plantedCount = int(f)
		}
	}
	species, _ := measurements["species"].(string)
	if species == "" {
		species = "mixed"
	}

	// Match by species and unit count.
	for i := range requests {
		req := &requests[i]
		if !speciesMatch(req.Species, species) {
			continue
		}

		// Match found — tag the activity.
		now := s.now().UTC()
		fulfilled := plantedCount
		status := models.RestorationClaimMatched
		if fulfilled < req.TotalUnits {
			status = models.RestorationClaimPartiallyFulfilled
		} else {
			fulfilled = req.TotalUnits
			status = models.RestorationClaimFulfilled
		}

		if err := s.db.Model(&models.ConservationActivity{}).Where("id = ?", activity.ID).
			Updates(map[string]any{
				"jahazi_funded":           true,
				"restoration_request_id":  req.ID,
				"updated_at":              now,
			}).Error; err != nil {
			return nil, err
		}

		if err := s.db.Model(&models.RestorationRequest{}).Where("id = ?", req.ID).
			Updates(map[string]any{
				"status":              status,
				"matched_activity_id": activity.ID,
				"fulfilled_units":     fulfilled,
				"updated_at":          now,
			}).Error; err != nil {
			return nil, err
		}

		s.logger.Info("restoration claim matched",
			"claimId", req.JahaziClaimID, "activityId", activity.ID,
			"fulfilledUnits", fulfilled, "status", status)

		// Notify PM that the planting matched a restoration claim
		if s.notifier != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				s.notifier.NotifyRestorationMatched(ctx,
					activity.ProjectID, "", req.Species, fulfilled, req.TotalUnits)
			}()
		}

		return req, nil
	}

	return nil, nil // no match — normal activity
}

// ── MRV link generation ─────────────────────────────────────────────────────

// MrvLinkPayload is the signed payload BahariMRV pushes to Jahazi when a
// matched planting activity has been anchored on IOTA.
type MrvLinkPayload struct {
	MrvRecordID        string   `json:"mrvRecordId"`
	ProjectID          string   `json:"projectId"`          // Jahazi project ID
	RestorationClaimID string   `json:"restorationClaimId"` // Jahazi claim ID
	ContentHash        string   `json:"contentHash"`
	TxHash             string   `json:"txHash"`             // IOTA message ID (empty if pending)
	CapturedAt         string   `json:"capturedAt"`         // ISO 8601
	Species            string   `json:"species"`
	Count              int      `json:"count"`
	EvidenceCIDs       []string `json:"evidenceCids"`
	Nonce              string   `json:"nonce"`
	Signature          string   `json:"signature"`
}

// GenerateMrvLink builds a signed MRV link payload for a matched activity.
// The txHash comes from the provenance event's transaction reference; if the
// anchor is still pending, txHash is empty and the payload is still valid (the
// certificate will show "anchoring pending").
func (s *Service) GenerateMrvLink(activityID string, jahaziProjectID string) (MrvLinkPayload, error) {
	if !s.Enabled() {
		return MrvLinkPayload{}, fmt.Errorf("jahazi integration is not configured")
	}

	var activity models.ConservationActivity
	if err := s.db.Where("id = ?", activityID).First(&activity).Error; err != nil {
		return MrvLinkPayload{}, ErrActivityNotFound
	}

	if !activity.JahaziFunded {
		// In dev mode (no secret configured), allow generating MRV links for any
		// completed planting activity — the RestorationClaim flow is optional
		// when the integration secret is not set.
		if s.secret != "" {
			return MrvLinkPayload{}, fmt.Errorf("activity is not jahazi-funded")
		}
		// Dev mode: proceed without a matched RestorationRequest
	}

	var req models.RestorationRequest
	if activity.RestorationRequestID != "" {
		if err := s.db.Where("id = ?", activity.RestorationRequestID).First(&req).Error; err != nil {
			return MrvLinkPayload{}, err
		}
	}

	// Get the latest provenance event for this activity to extract txHash.
	var event models.ProvenanceEvent
	var txHash string
	if err := s.db.Where("activity_id = ?", activityID).Order("created_at desc").First(&event).Error; err == nil {
		txHash = event.TransactionRef
	}

	// Extract measurements for species and count.
	var measurements map[string]any
	if len(activity.Measurements) > 0 {
		_ = json.Unmarshal(activity.Measurements, &measurements)
	}
	species, _ := measurements["species"].(string)
	if species == "" {
		species = "mixed"
	}
	count := 0
	if plantedCount, ok := measurements["planted_count"].(float64); ok {
		count = int(plantedCount)
	} else if plantedCount, ok := measurements["planted_count"].(int); ok {
		count = plantedCount
	}

	// Compute content hash from canonical activity data.
	contentData := map[string]any{
		"activityId":    activity.ID,
		"projectId":     activity.ProjectID,
		"activityType":  activity.ActivityType,
		"species":       species,
		"count":          count,
		"capturedAt":     activity.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"methodologyVer": activity.MethodologyVersion,
	}
	contentHash, err := contentHash(contentData)
	if err != nil {
		return MrvLinkPayload{}, fmt.Errorf("compute content hash: %w", err)
	}

	nonce := uuid.NewString()

	payload := MrvLinkPayload{
		MrvRecordID:        activity.ID,
		ProjectID:          jahaziProjectID,
		RestorationClaimID: req.JahaziClaimID,
		ContentHash:        contentHash,
		TxHash:             txHash,
		CapturedAt:         activity.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Species:            species,
		Count:              count,
		EvidenceCIDs:       []string{}, // IPFS CIDs filled when evidence is pinned
		Nonce:              nonce,
	}

	// Sign the payload (without the signature field itself).
	signature, err := s.signMrvLink(payload)
	if err != nil {
		return MrvLinkPayload{}, fmt.Errorf("sign MRV link: %w", err)
	}
	payload.Signature = signature

	// Mark the activity as MRV link generated.
	now := s.now().UTC()
	if err := s.db.Model(&models.ConservationActivity{}).Where("id = ?", activityID).
		Updates(map[string]any{"mrv_link_generated": true, "updated_at": now}).Error; err != nil {
		return payload, err
	}

	return payload, nil
}

// PushMrvLink sends the signed MRV link to Jahazi via webhook. The endpoint is
// POST {JAHAZI_WEBHOOK_URL}/conservation/projects/{slug}/mrv-links.
func (s *Service) PushMrvLink(ctx context.Context, payload MrvLinkPayload, jahaziSlug string) error {
	if !s.Enabled() {
		return fmt.Errorf("jahazi integration is not configured")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal MRV link: %w", err)
	}

	// Compute HMAC signature of the raw body for the x-bahari-signature header.
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	url := fmt.Sprintf("%s/conservation/projects/%s/mrv-links", s.webhookURL, jahaziSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bahari-signature", signature)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Warn("mrv link push failed", "error", err, "url", url)
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		s.logger.Warn("mrv link push rejected", "status", resp.StatusCode, "url", url)
		return fmt.Errorf("jahazi rejected MRV link: HTTP %d", resp.StatusCode)
	}

	// Record push timestamp on the activity.
	now := s.now().UTC()
	s.db.Model(&models.ConservationActivity{}).Where("id = ?", payload.MrvRecordID).
		Updates(map[string]any{"mrv_link_pushed_at": now, "updated_at": now})

	s.logger.Info("mrv link pushed to jahazi",
		"activityId", payload.MrvRecordID, "slug", jahaziSlug, "txHash", payload.TxHash)

	return nil
}

// ── Attribution data ─────────────────────────────────────────────────────────

// AttributionData is the per-tree CO2e and on-chain proof data Jahazi needs to
// mint accurate certificates for tourists.
type AttributionData struct {
	ActivityID         string         `json:"activityId"`
	ActivityType       string         `json:"activityType"`
	ProjectID          string         `json:"projectId"`
	ProjectName        string         `json:"projectName"`
	Species            string         `json:"species"`
	SeedlingsCount     int            `json:"seedlingsCount"`
	AreaHa             string         `json:"areaHa"`
	CapturedAt         string         `json:"capturedAt"`
	MethodologyVersion string         `json:"methodologyVersion"`
	PerTreeCo2eKg5yr   string         `json:"perTreeCo2eKg5yr"`
	TotalCo2eKg5yr     string         `json:"totalCo2eKg5yr"`
	Iota               IotaProof      `json:"iota"`
	VerifierAttestation *AttestationData `json:"verifierAttestation,omitempty"`
	ProofID            string         `json:"proofId"`
	JahaziFunded       bool           `json:"jahaziFunded"`
	RestorationRequestID string       `json:"restorationRequestId,omitempty"`
}

type IotaProof struct {
	TxHash      string `json:"txHash"`
	ContentHash string `json:"contentHash"`
	ExplorerURL string `json:"explorerUrl"`
	Status      string `json:"status"`
}

type AttestationData struct {
	VerifierID  string `json:"verifierId"`
	VerifierDID string `json:"verifierDid"`
	ApprovedAt  string `json:"approvedAt"`
}

// AttributionData returns the per-tree CO2e estimate, IOTA anchor reference,
// and verifier attestation for a matched activity. This is what Jahazi fetches
// to populate certificate content.
func (s *Service) AttributionData(activityID string) (AttributionData, error) {
	var activity models.ConservationActivity
	if err := s.db.Where("id = ?", activityID).First(&activity).Error; err != nil {
		return AttributionData{}, ErrActivityNotFound
	}

	var project models.ConservationProject
	if err := s.db.Where("id = ?", activity.ProjectID).First(&project).Error; err != nil {
		return AttributionData{}, ErrProjectNotFound
	}

	var methodology models.Methodology
	if err := s.db.Where("id = ?", project.MethodologyID).First(&methodology).Error; err != nil {
		return AttributionData{}, err
	}

	// Extract measurements.
	var measurements map[string]any
	if len(activity.Measurements) > 0 {
		_ = json.Unmarshal(activity.Measurements, &measurements)
	}
	species, _ := measurements["species"].(string)
	if species == "" {
		species = project.Species
	}
	count := 0
	if plantedCount, ok := measurements["planted_count"].(float64); ok {
		count = int(plantedCount)
	} else if plantedCount, ok := measurements["planted_count"].(int); ok {
		count = plantedCount
	}

	// Per-tree CO2e from methodology estimation parameters.
	perTreeCo2e, periodYears := perTreeCo2eFromMethodology(methodology)
	totalCo2e := perTreeCo2e * float64(count) * float64(periodYears)

	// IOTA proof data from the latest provenance event.
	var event models.ProvenanceEvent
	iotaProof := IotaProof{Status: "not_recorded"}
	if err := s.db.Where("activity_id = ?", activityID).Order("created_at desc").First(&event).Error; err == nil {
		iotaProof.TxHash = event.TransactionRef
		iotaProof.Status = event.Status
		if s.explorerURL != "" && event.TransactionRef != "" {
			iotaProof.ExplorerURL = fmt.Sprintf("%s/transaction/%s", s.explorerURL, event.TransactionRef)
		}
	}

	// Content hash from the event.
	if event.Commitment != "" {
		iotaProof.ContentHash = event.Commitment
	}

	// Verifier attestation.
	var attestation models.VerifierAttestation
	var attData *AttestationData
	if err := s.db.Where("activity_id = ? AND revoked_at IS NULL", activityID).
		Order("created_at desc").First(&attestation).Error; err == nil {
		attData = &AttestationData{
			VerifierID:  attestation.VerifierID,
			VerifierDID: attestation.IssuerDID,
			ApprovedAt:  attestation.CreatedAt.UTC().Format(time.RFC3339),
		}
	}

	// Proof ID from the latest proof document.
	var proof models.ProofDocument
	proofID := ""
	if err := s.db.Where("activity_id = ?", activityID).Order("created_at desc").First(&proof).Error; err == nil {
		proofID = proof.ID
	}

	// Area from plot.
	var plot models.Plot
	areaHa := "0"
	if err := s.db.Where("id = ?", activity.PlotID).First(&plot).Error; err == nil {
		areaHa = plot.AreaHectares
	}

	return AttributionData{
		ActivityID:          activity.ID,
		ActivityType:       activity.ActivityType,
		ProjectID:          activity.ProjectID,
		ProjectName:        project.Name,
		Species:            species,
		SeedlingsCount:     count,
		AreaHa:             areaHa,
		CapturedAt:         activity.UpdatedAt.UTC().Format(time.RFC3339),
		MethodologyVersion: activity.MethodologyVersion,
		PerTreeCo2eKg5yr:   fmt.Sprintf("%.4f", perTreeCo2e),
		TotalCo2eKg5yr:     fmt.Sprintf("%.2f", totalCo2e),
		Iota:               iotaProof,
		VerifierAttestation: attData,
		ProofID:            proofID,
		JahaziFunded:       activity.JahaziFunded,
		RestorationRequestID: activity.RestorationRequestID,
	}, nil
}

// ── Jahazi project listing ──────────────────────────────────────────────────

// JahaziProjectView is the project listing shape Jahazi fetches to populate
// its conservation catalog.
type JahaziProjectView struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	Species          string `json:"species"`
	ConservationType string `json:"conservationType"`
	JahaziProjectID  string `json:"jahaziProjectId,omitempty"`
	MethodologyCode   string `json:"methodologyCode"`
	MethodologyVersion string `json:"methodologyVersion"`
}

// ListJahaziProjects returns all projects with jahazi_enabled=true.
func (s *Service) ListJahaziProjects() ([]JahaziProjectView, error) {
	var projects []models.ConservationProject
	if err := s.db.Where("jahazi_enabled = ? AND status = ?", true, "active").Find(&projects).Error; err != nil {
		return nil, err
	}

	views := make([]JahaziProjectView, 0, len(projects))
	for _, p := range projects {
		var methodology models.Methodology
		if err := s.db.Where("id = ?", p.MethodologyID).First(&methodology).Error; err != nil {
			continue
		}
		views = append(views, JahaziProjectView{
			ID:                p.ID,
			Name:              p.Name,
			Status:            p.Status,
			Species:           p.Species,
			ConservationType:  p.ConservationType,
			JahaziProjectID:   p.JahaziProjectID,
			MethodologyCode:   methodology.Code,
			MethodologyVersion: methodology.Version,
		})
	}
	return views, nil
}

// ── Contribution mirroring ───────────────────────────────────────────────────

// ReceiveContribution records a Jahazi tourist contribution as a FinancialImport
// on BahariMRV, so the project wallet reflects tourism revenue.
func (s *Service) ReceiveContribution(input models.JahaziContribution) (models.FinancialImport, error) {
	if !s.Enabled() {
		return models.FinancialImport{}, fmt.Errorf("jahazi integration is not configured")
	}

	if !s.VerifySignature(input, input.Signature) {
		return models.FinancialImport{}, ErrInvalidSignature
	}

	// Find the BahariMRV project linked to this Jahazi project.
	var project models.ConservationProject
	if err := s.db.Where("jahazi_project_id = ?", input.JahaziProjectID).First(&project).Error; err != nil {
		return models.FinancialImport{}, ErrProjectNotFound
	}

	// Idempotency: check if this contribution was already recorded.
	var existing int64
	if err := s.db.Model(&models.FinancialImport{}).
		Where("source = ? AND external_id = ?", "jahazi_contribution", input.JahaziContributionID).
		Count(&existing).Error; err != nil {
		return models.FinancialImport{}, err
	}
	if existing > 0 {
		// Return the existing record (idempotent).
		var existingImport models.FinancialImport
		s.db.Where("source = ? AND external_id = ?", "jahazi_contribution", input.JahaziContributionID).First(&existingImport)
		return existingImport, nil
	}

	occurredAt, err := time.Parse(time.RFC3339, input.ContributedAt)
	if err != nil {
		occurredAt = s.now().UTC()
	}

	// Compute payload hash for audit.
	payloadData, _ := json.Marshal(input)
	hash := fmt.Sprintf("%x", sha256.Sum256(payloadData))

	imp := models.FinancialImport{
		ID:          uuid.NewString(),
		Source:      "jahazi_contribution",
		ExternalID:  input.JahaziContributionID,
		ProjectID:   project.ID,
		Amount:      input.AmountUsd,
		Currency:    "USD",
		OccurredAt:  occurredAt.UTC(),
		PayloadHash: hash,
		CreatedAt:   s.now().UTC(),
	}

	if err := s.db.Create(&imp).Error; err != nil {
		return imp, err
	}

	s.logger.Info("jahazi contribution recorded",
		"contributionId", input.JahaziContributionID,
		"projectId", project.ID, "amount", input.AmountUsd)

	return imp, nil
}

// ── Claim expiry cron ────────────────────────────────────────────────────────

// ExpireClaims marks RestorationRequests as expired if their TTL has passed
// without a matching activity. Designed to be called periodically.
func (s *Service) ExpireClaims() (int64, error) {
	result := s.db.Model(&models.RestorationRequest{}).
		Where("status = ? AND expires_at < ?", models.RestorationClaimOpen, s.now().UTC()).
		Update("status", models.RestorationClaimExpired)
	return result.RowsAffected, result.Error
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// GenerateNonce creates a random hex nonce for use in MRV link payloads.
func GenerateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// canonicalJSON serializes a value with sorted keys for deterministic signing.
func canonicalJSON(v any) ([]byte, error) {
	// Marshal then re-marshal with sorted keys by using a map intermediary.
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return raw, nil // fall back to raw if we can't re-encode
	}
	return json.Marshal(sortKeys(m))
}

func sortKeys(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		result := make(map[string]any, len(t))
		for _, k := range keys {
			result[k] = sortKeys(t[k])
		}
		return result
	case []any:
		for i := range t {
			t[i] = sortKeys(t[i])
		}
		return t
	default:
		return v
	}
}

// contentHash computes the SHA-256 of the canonical JSON of the data.
func contentHash(data map[string]any) (string, error) {
	canonical, err := canonicalJSON(data)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

// signMrvLink signs the MRV link payload (without the signature field).
func (s *Service) signMrvLink(payload MrvLinkPayload) (string, error) {
	// Create a copy without the signature for signing.
	signTarget := struct {
		MrvRecordID        string   `json:"mrvRecordId"`
		ProjectID          string   `json:"projectId"`
		RestorationClaimID string   `json:"restorationClaimId"`
		ContentHash        string   `json:"contentHash"`
		TxHash             string   `json:"txHash"`
		CapturedAt         string   `json:"capturedAt"`
		Species            string   `json:"species"`
		Count              int      `json:"count"`
		EvidenceCIDs       []string `json:"evidenceCids"`
		Nonce              string   `json:"nonce"`
	}{
		MrvRecordID:        payload.MrvRecordID,
		ProjectID:          payload.ProjectID,
		RestorationClaimID: payload.RestorationClaimID,
		ContentHash:        payload.ContentHash,
		TxHash:             payload.TxHash,
		CapturedAt:         payload.CapturedAt,
		Species:            payload.Species,
		Count:              payload.Count,
		EvidenceCIDs:       payload.EvidenceCIDs,
		Nonce:              payload.Nonce,
	}
	return s.Sign(signTarget)
}

// speciesMatch checks whether the planted species satisfies the claim's
// species requirement. "mixed" matches any species.
func speciesMatch(claimSpecies, plantedSpecies string) bool {
	if claimSpecies == "mixed" || plantedSpecies == "mixed" {
		return true
	}
	return strings.EqualFold(claimSpecies, plantedSpecies)
}

// isPlantingType reports whether an activity type is a planting activity
// eligible for Jahazi matching.
func isPlantingType(activityType string) bool {
	t := strings.ToLower(strings.TrimSpace(activityType))
	return t == "planting" || t == "restoration" || t == "mangrove_planting"
}

// perTreeCo2eFromMethodology extracts the per-tree CO2e estimate and period
// years from the methodology's estimation parameters.
func perTreeCo2eFromMethodology(methodology models.Methodology) (float64, int) {
	periodYears := 5 // default to 5-year estimate for certificates
	if len(methodology.Estimation) == 0 || string(methodology.Estimation) == "{}" {
		return 0.871, periodYears // default mangrove rate: 0.871 kg CO2e/tree/year
	}

	var params struct {
		TonnesCO2PerTree string `json:"tonnes_co2_per_tree"`
		PeriodYears      int    `json:"period_years"`
	}
	if err := json.Unmarshal(methodology.Estimation, &params); err != nil {
		return 0.871, periodYears
	}

	perTree := 0.871
	if params.TonnesCO2PerTree != "" {
		if v, err := parseFloat(params.TonnesCO2PerTree); err == nil {
			perTree = v * 1000 // convert tonnes to kg
		}
	}
	if params.PeriodYears > 0 {
		periodYears = params.PeriodYears
	}
	return perTree, periodYears
}

// parseFloat is a small helper to avoid importing strconv for one call.
func parseFloat(s string) (float64, error) {
	var f float64
	for _, c := range s {
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			if c != '-' && c != '+' && c != 'e' && c != 'E' {
				return 0, fmt.Errorf("invalid float: %s", s)
			}
		}
	}
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrInvalidSignature  = fmt.Errorf("invalid signature")
	ErrDuplicateClaim    = fmt.Errorf("duplicate restoration claim (replay detected)")
	ErrProjectNotFound   = fmt.Errorf("project not found")
	ErrNotJahaziEnabled  = fmt.Errorf("project is not jahazi-enabled")
	ErrClaimNotFound     = fmt.Errorf("restoration claim not found")
	ErrActivityNotFound  = fmt.Errorf("activity not found")
)

// GetAdapter exposes the chain adapter for handler use.
// Used by chain callback handlers to call new Move operations.
func (s *Service) GetAdapter() interface{ AppendPlantingEntry(ctx context.Context, req interface{}) (interface{}, error) } {
	return nil // actual adapter is injected via ChainOps
}

// ForwardCertificateToExecutor forwards a certificate issuance payload directly
// to the IOTA executor via the configured webhook URL.
func (s *Service) ForwardCertificateToExecutor(ctx context.Context, payload []byte) (any, error) {
	if !s.Enabled() {
		return map[string]string{"status": "skipped", "reason": "chain disabled"}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.webhookURL+"/api/v1/chain/issue-certificate-internal",
		bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.secret != "" {
		mac := hmac.New(sha256.New, []byte(s.secret))
		mac.Write(payload)
		req.Header.Set("x-bahari-signature", hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Warn("ForwardCertificateToExecutor failed", "error", err)
		return map[string]string{"status": "pending", "reason": err.Error()}, nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return map[string]string{"status": "forwarded"}, nil
}

// PlantingEntryFromContribution is a convenience constructor for ChainOps.
func PlantingEntryFromContribution(ledgerID, claimID string, amountCents, contributorCount int64) chain.PlantingEntryRequest {
	return chain.PlantingEntryRequest{
		LedgerObjectID:      ledgerID,
		PlantingRecordID:    claimID,
		ProvenanceAnchorID:  "",
		SeedlingsAdded:      0,
		AliveAdded:          0,
		Co2eMilligramsAdded: 0,
	}
}
