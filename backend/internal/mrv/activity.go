package mrv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/baharilink/baharimrv/internal/satellite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ActivityRef identifies the achievement a provenance view describes.
type ActivityRef struct {
	ID                    string `json:"id"`
	ProjectName           string `json:"projectName"`
	AchievementID         string `json:"achievementId,omitempty"`
	VerificationRequestID string `json:"verificationRequestId,omitempty"`
	Status                string `json:"status"`
}

// Stage is one gate of the verification pipeline.
type Stage struct {
	ID                 string     `json:"id"`
	Key                string     `json:"key"`
	Label              string     `json:"label"`
	Owner              string     `json:"owner"`
	Status             string     `json:"status"`
	Timestamp          *time.Time `json:"timestamp,omitempty"`
	EvidenceReferences []string   `json:"evidenceReferences"`
	Exception          string     `json:"exception,omitempty"`
	NextAction         string     `json:"nextAction,omitempty"`
}

// DocumentView tracks a required project document.
type DocumentView struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Version     string     `json:"version,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	DownloadURL string     `json:"downloadUrl,omitempty"`
}

// ProvenanceView is the C3 verification pipeline payload.
type ProvenanceView struct {
	Activity  ActivityRef            `json:"activity"`
	Stages    []Stage                `json:"stages"`
	Documents []DocumentView         `json:"documents"`
	Events    []provenance.EventView `json:"events"`
}

// EvidenceGap compares each record's declared manifest with what is actually stored.
type EvidenceGap struct {
	Declared int
	Missing  int
	Stored   []models.EvidenceObject
}

func (s *Service) EvidenceGap(records []models.MonitoringRecord) (EvidenceGap, error) {
	gap := EvidenceGap{Stored: []models.EvidenceObject{}}
	for _, record := range records {
		gap.Declared += len(ManifestFor(record))
	}
	if len(records) == 0 {
		return gap, nil
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	if err := s.db.Where("monitoring_record_id IN ?", ids).Order("created_at asc").Find(&gap.Stored).Error; err != nil {
		return gap, err
	}
	present := make(map[string]bool, len(gap.Stored))
	for _, object := range gap.Stored {
		present[object.MonitoringRecordID+"|"+strings.ToLower(object.SHA256)] = true
	}
	for _, record := range records {
		for _, item := range ManifestFor(record) {
			if !present[record.ID+"|"+strings.ToLower(item.SHA256)] {
				gap.Missing++
			}
		}
	}
	return gap, nil
}

// AssertActivityAccess resolves the activity's project, then applies that
// project's role rules.
func (s *Service) AssertActivityAccess(userID, activityID string) error {
	var activity models.ConservationActivity
	if err := s.db.Where("id = ?", activityID).First(&activity).Error; err != nil {
		return ErrNotFound
	}
	return s.AssertProjectAccess(userID, activity.ProjectID)
}

// markActivityMonitoring moves a draft activity into monitoring once a field
// record lands. Later states are driven by completion and verification only.
func (s *Service) markActivityMonitoring(activity models.ConservationActivity) error {
	if activity.Status != models.ActivityDraft {
		return nil
	}
	return s.db.Model(&models.ConservationActivity{}).Where("id = ?", activity.ID).
		Updates(map[string]any{"status": models.ActivityMonitoring, "updated_at": s.now().UTC()}).Error
}

// Activity loads an achievement with its project.
func (s *Service) Activity(activityID string) (models.ConservationActivity, models.ConservationProject, error) {
	var activity models.ConservationActivity
	if err := s.db.Where("id = ?", activityID).First(&activity).Error; err != nil {
		return activity, models.ConservationProject{}, ErrNotFound
	}
	var project models.ConservationProject
	if err := s.db.Where("id = ?", activity.ProjectID).First(&project).Error; err != nil {
		return activity, project, ErrNotFound
	}
	return activity, project, nil
}

// Complete records the project manager's signed completion claim. The claim is
// anchored as achieved_unverified: it proves the claim existed, not that it is
// independently valid.
func (s *Service) Complete(ctx context.Context, activityID, userID string) (models.ConservationActivity, models.ProvenanceEvent, error) {
	activity, project, err := s.Activity(activityID)
	if err != nil {
		return activity, models.ProvenanceEvent{}, err
	}
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return activity, models.ProvenanceEvent{}, ErrForbidden
	}
	if user.Role != models.RoleAdmin && (user.Role != models.RoleManager || project.ManagerID != userID) {
		return activity, models.ProvenanceEvent{}, ErrForbidden
	}
	if activity.Status == models.ActivityVerified || activity.Status == models.ActivityRejected {
		return activity, models.ProvenanceEvent{}, domain.Reject("this activity is already closed")
	}
	records, err := s.RecordsFor(activityID)
	if err != nil {
		return activity, models.ProvenanceEvent{}, err
	}
	if len(records) == 0 {
		return activity, models.ProvenanceEvent{}, domain.Reject("no monitoring records have been synchronised for this activity")
	}
	gap, err := s.EvidenceGap(records)
	if err != nil {
		return activity, models.ProvenanceEvent{}, err
	}
	if gap.Missing > 0 {
		return activity, models.ProvenanceEvent{}, domain.Reject("%d declared evidence files have not been uploaded yet", gap.Missing)
	}
	provision, err := s.identities.ForUser(userID)
	if err != nil || provision.DID == "" {
		return activity, models.ProvenanceEvent{}, domain.Reject("an active provisioned identity is required to claim completion")
	}

	summary := Summarise(records, activity)
	previous, err := s.latestEvent(activityID, models.ClaimCompletion)
	if err != nil {
		return activity, models.ProvenanceEvent{}, err
	}
	event, err := s.provenance.Record(ctx, provenance.Input{
		AchievementID: activity.ID, ProjectID: activity.ProjectID, ClaimType: models.ClaimCompletion,
		ActivityType: activity.ActivityType, MethodologyVersion: activity.MethodologyVersion,
		ReportingPeriodStart: activity.ReportingPeriodStart, ReportingPeriodEnd: activity.ReportingPeriodEnd,
		Measurements: summary, MeasurementBasis: "claimed",
		EvidenceRoot: EvidenceRoot(records), IssuerDID: provision.DID,
		PreviousEventID: previousID(previous),
	})
	if err != nil {
		return activity, event, err
	}
	now := s.now().UTC()
	if err := s.db.Model(&models.ConservationActivity{}).Where("id = ?", activity.ID).Updates(map[string]any{
		"status": models.ActivityAwaitingVerification, "current_event_id": event.ID, "updated_at": now,
	}).Error; err != nil {
		return activity, event, err
	}
	activity.Status = models.ActivityAwaitingVerification
	activity.CurrentEventID = event.ID

	if len(gap.Stored) > 0 && s.satellite.Mode() != "live" {
		if _, err := s.satellite.AttachSandbox(activity, ndviBaseline(summary)); err != nil {
			return activity, event, err
		}
	}
	if err := s.assignVerifier(activity); err != nil {
		return activity, event, err
	}

	// Jahazi restoration matching: if a RestorationMatcher is configured and this
	// is a planting activity, check for open RestorationRequests. When matched,
	// the activity is tagged jahazi_funded and a RestorationRequest moves to
	// matched/fulfilled. Normal activities (no open claim) are unaffected.
	if s.matcher != nil {
		summary := Summarise(records, activity)
		if matchErr := s.matcher.MatchActivity(activity, summary); matchErr != nil {
			slog.Warn("restoration matching failed", "activityId", activity.ID, "error", matchErr)
		}
	}

	return activity, event, nil
}

// RetryAnchor re-anchors the newest unconfirmed claim for an activity.
func (s *Service) RetryAnchor(ctx context.Context, activityID, userID string) (models.ProvenanceEvent, error) {
	if err := s.AssertActivityAccess(userID, activityID); err != nil {
		return models.ProvenanceEvent{}, err
	}
	event, err := s.provenance.Retry(ctx, activityID)
	if errors.Is(err, provenance.ErrNotRetryable) {
		return event, domain.Reject("this achievement is already anchored or has no pending claim")
	}
	return event, err
}

// ProvenanceView builds the C3 pipeline projection for an activity.
func (s *Service) ProvenanceView(activityID, userID string) (ProvenanceView, error) {
	activity, project, err := s.Activity(activityID)
	if err != nil {
		return ProvenanceView{}, err
	}
	if err := s.AssertProjectAccess(userID, project.ID); err != nil {
		return ProvenanceView{}, err
	}
	records, err := s.RecordsFor(activityID)
	if err != nil {
		return ProvenanceView{}, err
	}
	events, err := s.provenance.Events(activityID)
	if err != nil {
		return ProvenanceView{}, err
	}
	actors, err := s.actorNames(events)
	if err != nil {
		return ProvenanceView{}, err
	}
	request, attestation, err := s.verificationFor(activityID)
	if err != nil {
		return ProvenanceView{}, err
	}
	documents, err := s.Documents(project.ID)
	if err != nil {
		return ProvenanceView{}, err
	}
	batch, err := s.reconciledBatch(project.ID)
	if err != nil {
		return ProvenanceView{}, err
	}
	gap, err := s.EvidenceGap(records)
	if err != nil {
		return ProvenanceView{}, err
	}
	view := ProvenanceView{
		Activity: ActivityRef{
			ID: activity.ID, ProjectName: project.Name, AchievementID: activity.ID,
			VerificationRequestID: requestID(request), Status: activity.Status,
		},
		Stages:    stages(s.satellite, activity, records, gap, request, attestation, batch),
		Documents: documents,
		Events:    make([]provenance.EventView, 0, len(events)),
	}
	for _, event := range events {
		view.Events = append(view.Events, s.provenance.View(event, actors[event.IssuerDID]))
	}
	return view, nil
}

func stages(imagery *satellite.Service, activity models.ConservationActivity, records []models.MonitoringRecord, gap EvidenceGap, request *models.VerificationRequest, attestation *models.VerifierAttestation, batch *models.CreditBatch) []Stage {
	fieldRefs := make([]string, 0, len(records))
	for _, record := range records {
		fieldRefs = append(fieldRefs, record.ID)
	}
	field := Stage{
		ID: activity.ID + ":field_data", Key: "field_data", Label: "Field data complete",
		Owner: "Field Monitor", Status: "pending", EvidenceReferences: fieldRefs,
	}
	switch {
	case len(records) == 0:
		field.NextAction = "Synchronise at least one monitoring record from the field app."
	case gap.Missing > 0:
		field.Status = "in_progress"
		field.Exception = fmt.Sprintf("%d of %d declared evidence files are still missing.", gap.Missing, gap.Declared)
		field.NextAction = "Upload the remaining evidence files."
	default:
		receipt := records[len(records)-1].ServerReceiptTime
		field.Status = "complete"
		field.Timestamp = &receipt
		field.NextAction = "Claim completion to anchor the achievement."
	}

	corroboration := Stage{
		ID: activity.ID + ":satellite", Key: "satellite", Label: "Satellite corroboration",
		Owner: "Satellite adapter", Status: "pending", EvidenceReferences: []string{},
	}
	if summary, ok := imagery.Latest(activity.ID); ok {
		corroboration.Status = "complete"
		corroboration.EvidenceReferences = []string{summary.ID}
		acquired := summary.AcquiredAt
		corroboration.Timestamp = &acquired
		if !summary.Live {
			corroboration.Owner = "Satellite sandbox model"
			corroboration.Exception = "Sandbox corroboration only. No live imagery source is configured, so this value is not evidence."
		}
	} else if imagery.Mode() != "live" {
		corroboration.NextAction = "Attach corroboration once field data is complete."
	} else {
		corroboration.Status = "unavailable"
		corroboration.Exception = "The live satellite adapter returned no observation for this reporting period."
	}

	audit := Stage{
		ID: activity.ID + ":third_party_audit", Key: "third_party_audit", Label: "Third-party audit",
		Owner: "Independent Verifier", Status: "pending", EvidenceReferences: []string{},
	}
	switch {
	case attestation != nil && attestation.RevokedAt == nil:
		audit.Status = "complete"
		audit.EvidenceReferences = []string{attestation.ID}
		audit.Timestamp = &attestation.CreatedAt
		audit.Owner = attestation.IssuerDID
	case attestation != nil:
		audit.Status = "failed"
		audit.Exception = "The verifier attestation was revoked."
		audit.EvidenceReferences = []string{attestation.ID}
	case request != nil && request.Status == "rejected":
		audit.Status = "failed"
		audit.EvidenceReferences = []string{request.ID}
		audit.NextAction = "Submit a corrected monitoring record."
	case request != nil:
		audit.Status = "in_progress"
		audit.EvidenceReferences = []string{request.ID}
		audit.NextAction = "Await the verifier decision."
	default:
		audit.NextAction = "Claim completion to request an independent verification."
	}

	registry := Stage{
		ID: activity.ID + ":registry_issuance", Key: "registry_issuance", Label: "Registry issuance",
		Owner: "Registry adapter", Status: "pending", EvidenceReferences: []string{},
	}
	switch {
	case attestation == nil || attestation.RevokedAt != nil:
		registry.Exception = "A completion claim alone cannot supply registry-issued credits."
		registry.NextAction = "Issuance is only possible after an independent verification decision."
	case batch != nil:
		registry.Status = "complete"
		registry.EvidenceReferences = []string{batch.RegistryIssuanceID}
		registry.Owner = batch.RegistryName
		if batch.AdapterMode != "live" {
			registry.Exception = "Sandbox registry import. These serials are not real issued credits."
		}
		registry.Timestamp = batch.ReconciledAt
	default:
		registry.Status = "in_progress"
		registry.NextAction = "Import and reconcile the registry issuance for this project."
	}
	return []Stage{field, corroboration, audit, registry}
}

// Documents lists the project's tracked documents.
func (s *Service) Documents(projectID string) ([]DocumentView, error) {
	var documents []models.ProjectDocument
	if err := s.db.Where("project_id = ?", projectID).Order("created_at asc").Find(&documents).Error; err != nil {
		return nil, err
	}
	views := make([]DocumentView, 0, len(documents))
	for _, document := range documents {
		views = append(views, DocumentView{
			ID: document.ID, Kind: document.Kind, Name: document.Name, Status: document.Status,
			Version: document.Version, UpdatedAt: &document.UpdatedAt,
			DownloadURL: "/api/v1/documents/" + document.ID,
		})
	}
	return views, nil
}

// LatestCorroboration exposes the newest satellite summary for an activity.
func (s *Service) LatestCorroboration(activityID string) (satellite.Summary, bool) {
	return s.satellite.Latest(activityID)
}

func (s *Service) latestEvent(activityID, claimType string) (models.ProvenanceEvent, error) {
	var event models.ProvenanceEvent
	err := s.db.Where("activity_id = ? AND claim_type = ?", activityID, claimType).
		Order("created_at desc").First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return event, nil
	}
	return event, err
}

// recordEvidenceClaim optionally anchors that a bundle existed at a point in time.
// It never implies completion or verification.
func (s *Service) recordEvidenceClaim(ctx context.Context, activity models.ConservationActivity, record models.MonitoringRecord) error {
	provision, err := s.identities.ForUser(record.MonitorID)
	if err != nil || provision.DID == "" {
		return nil
	}
	if len(ManifestFor(record)) == 0 {
		return nil
	}
	_, err = s.provenance.Record(ctx, provenance.Input{
		AchievementID: activity.ID, ProjectID: activity.ProjectID, ClaimType: "evidence_submitted",
		ActivityType: activity.ActivityType, MethodologyVersion: record.MethodologyVersion,
		ReportingPeriodStart: activity.ReportingPeriodStart, ReportingPeriodEnd: activity.ReportingPeriodEnd,
		Measurements:     map[string]any{"clientId": record.ClientID, "contentCommitment": record.ContentCommitment},
		MeasurementBasis: "claimed", EvidenceRoot: EvidenceRoot([]models.MonitoringRecord{record}),
		IssuerDID: provision.DID,
	})
	return err
}

func (s *Service) verificationFor(activityID string) (*models.VerificationRequest, *models.VerifierAttestation, error) {
	var request models.VerificationRequest
	var out *models.VerificationRequest
	if err := s.db.Where("activity_id = ?", activityID).Order("requested_at desc").First(&request).Error; err == nil {
		out = &request
	}
	var attestation models.VerifierAttestation
	var latest *models.VerifierAttestation
	if err := s.db.Where("activity_id = ?", activityID).Order("created_at desc").First(&attestation).Error; err == nil {
		latest = &attestation
	}
	return out, latest, nil
}

func (s *Service) reconciledBatch(projectID string) (*models.CreditBatch, error) {
	var batch models.CreditBatch
	err := s.db.Where("project_id = ? AND reconciled_at IS NOT NULL", projectID).
		Order("created_at desc").First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

func (s *Service) actorNames(events []models.ProvenanceEvent) (map[string]string, error) {
	names := map[string]string{}
	for _, event := range events {
		names[event.IssuerDID] = "Unattributed issuer"
	}
	if len(names) == 0 {
		return names, nil
	}
	dids := make([]string, 0, len(names))
	for did := range names {
		dids = append(dids, did)
	}
	var provisions []models.IdentityProvision
	if err := s.db.Where("did IN ?", dids).Find(&provisions).Error; err != nil {
		return nil, err
	}
	if len(provisions) == 0 {
		return names, nil
	}
	userIDs := make([]string, 0, len(provisions))
	didByUser := map[string]string{}
	for _, provision := range provisions {
		userIDs = append(userIDs, provision.UserID)
		didByUser[provision.UserID] = provision.DID
	}
	var users []models.User
	if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		if did, ok := didByUser[user.ID]; ok {
			names[did] = user.DisplayName
		}
	}
	return names, nil
}

func (s *Service) assignVerifier(activity models.ConservationActivity) error {
	var pending int64
	err := s.db.Model(&models.VerificationRequest{}).
		Where("activity_id = ? AND status IN ?", activity.ID, []string{"requested", "in_review"}).
		Count(&pending).Error
	if err != nil || pending > 0 {
		return err
	}
	verifier, err := s.pickVerifier(activity.ProjectID)
	if err != nil {
		return nil
	}
	now := s.now().UTC()
	return s.db.Create(&models.VerificationRequest{
		ID: uuid.NewString(), ActivityID: activity.ID, VerifierID: verifier.ID, Status: "requested",
		RequestedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
}

func (s *Service) pickVerifier(projectID string) (models.User, error) {
	var project models.ConservationProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return models.User{}, err
	}
	var verifier models.User
	err := s.db.Where("role = ? AND status = ? AND id <> ?", models.RoleVerifier, models.IdentityActive, project.ManagerID).
		Order("created_at asc").First(&verifier).Error
	if err != nil {
		return models.User{}, domain.Unavailable("no independent verifier is available")
	}
	return verifier, nil
}

func EvidenceRoot(records []models.MonitoringRecord) string {
	leaves := make([]string, 0, len(records))
	for _, record := range records {
		leaves = append(leaves, record.ContentCommitment)
	}
	hash, _, err := canon.Hash(leaves)
	if err != nil {
		return ""
	}
	return hash
}

func Summarise(records []models.MonitoringRecord, activity models.ConservationActivity) map[string]any {
	latest := records[len(records)-1]
	var planted int
	for _, record := range records {
		if record.PlantedCount > planted {
			planted = record.PlantedCount
		}
	}
	species := ""
	_ = json.Unmarshal(latest.Species, &species)
	return map[string]any{
		"activity_type": activity.ActivityType, "record_count": len(records),
		"alive_count": latest.AliveCount, "planted_count": latest.PlantedCount,
		"peak_planted_count": planted, "average_height_metres": latest.AverageHeightMetres,
		"species": species, "schema_version": latest.SchemaVersion,
		"methodology_version":  activity.MethodologyVersion,
		"low_accuracy_records": countLowAccuracy(records),
	}
}

func countLowAccuracy(records []models.MonitoringRecord) int {
	var count int
	for _, record := range records {
		if record.GPSAccuracyMeters > lowAccuracyMetres {
			count++
		}
	}
	return count
}

// ndviBaseline derives the sandbox model's starting NDVI from the claim's own
// planting figure so repeated claims on the same activity stay stable.
func ndviBaseline(measurements map[string]any) float64 {
	planted, _ := measurements["peak_planted_count"].(int)
	if planted <= 0 {
		return 0.2
	}
	baseline := 0.18 + float64(planted%70)/1000.0
	if baseline > 0.6 {
		baseline = 0.6
	}
	return baseline
}

func previousID(event models.ProvenanceEvent) string {
	return event.ID
}

func requestID(request *models.VerificationRequest) string {
	if request == nil {
		return ""
	}
	return request.ID
}
