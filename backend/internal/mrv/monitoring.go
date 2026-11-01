// Package mrv owns conservation projects, plots, activities, offline record
// synchronisation and the C1 dashboard read model.
package mrv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/evidence"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/money"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/baharilink/baharimrv/internal/satellite"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// maxAcceptedSkew tolerates field devices running slightly ahead of the server.
const maxAcceptedSkew = 15 * time.Minute

// lowAccuracyMetres is the GPS tolerance beyond which a record is flagged.
const lowAccuracyMetres = 100.0

var (
	ErrForbidden     = errors.New("you do not have access to this conservation resource")
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidRecord = errors.New("the monitoring record failed validation")
	ErrConflict      = errors.New("a different version of this record already exists on the server")
)

type Service struct {
	db         *gorm.DB
	identities *identity.Service
	provenance *provenance.Service
	satellite  *satellite.Service
	evidence   *evidence.Store
	now        func() time.Time
}

func NewService(db *gorm.DB, identities *identity.Service, anchors *provenance.Service, imagery *satellite.Service, store *evidence.Store) *Service {
	return &Service{db: db, identities: identities, provenance: anchors, satellite: imagery, evidence: store, now: time.Now}
}

// EvidenceManifestItem is one declared media file in a captured record.
type EvidenceManifestItem struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

type Location struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AccuracyMetres float64 `json:"accuracyMetres"`
}

type Attestation struct {
	Statement   string `json:"statement"`
	IdentityDID string `json:"identityDid,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

// Payload mirrors the capture client's record shape.
type Payload struct {
	ClientID            string                 `json:"clientId"`
	ActivityID          string                 `json:"activityId"`
	PlotID              string                 `json:"plotId"`
	MethodologyVersion  string                 `json:"methodologyVersion"`
	SchemaVersion       string                 `json:"schemaVersion"`
	DeviceTimestamp     time.Time              `json:"deviceTimestamp"`
	Location            Location               `json:"location"`
	AliveCount          int                    `json:"aliveCount"`
	PlantedCount        int                    `json:"plantedCount"`
	AverageHeightMetres string                 `json:"averageHeightMetres"`
	Species             string                 `json:"species"`
	Observations        string                 `json:"observations"`
	Attestation         Attestation            `json:"attestation"`
	EvidenceManifest    []EvidenceManifestItem `json:"evidenceManifest"`
}

// Action wraps a record so the batch stays extensible without changing clients.
type Action struct {
	Type   string  `json:"type"`
	Record Payload `json:"record"`
}

type Batch struct {
	Actions []Action `json:"actions"`
}

type Conflict struct {
	ServerVersion string `json:"serverVersion"`
	Detail        string `json:"detail"`
}

// Result is one synchronisation outcome per client record ID.
type Result struct {
	ClientID           string    `json:"clientId"`
	MonitoringRecordID string    `json:"monitoringRecordId,omitempty"`
	Status             string    `json:"status"`
	Error              string    `json:"error,omitempty"`
	Conflict           *Conflict `json:"conflict,omitempty"`
	Signed             bool      `json:"signed"`
	LowAccuracy        bool      `json:"lowAccuracy"`
}

type BatchResponse struct {
	Results []Result `json:"results"`
}

func (s *Service) SyncBatch(ctx context.Context, userID string, batch Batch) (BatchResponse, error) {
	if len(batch.Actions) == 0 {
		return BatchResponse{}, fmt.Errorf("%w: the batch is empty", ErrInvalidRecord)
	}
	if len(batch.Actions) > 50 {
		return BatchResponse{}, fmt.Errorf("%w: a batch holds at most 50 records", ErrInvalidRecord)
	}
	response := BatchResponse{Results: make([]Result, 0, len(batch.Actions))}
	for _, action := range batch.Actions {
		if action.Type != "monitoring_record" {
			response.Results = append(response.Results, Result{ClientID: action.Record.ClientID, Status: models.SyncFailed, Error: "Unsupported action type."})
			continue
		}
		result, err := s.syncOne(ctx, userID, action.Record)
		if err != nil {
			status := models.SyncFailed
			if errors.Is(err, ErrConflict) {
				status = models.SyncConflict
			}
			result = &Result{ClientID: action.Record.ClientID, Status: status, Error: publicMessage(err)}
		}
		response.Results = append(response.Results, *result)
	}
	return response, nil
}

func (s *Service) syncOne(ctx context.Context, userID string, payload Payload) (*Result, error) {
	if err := validateShape(payload); err != nil {
		return nil, err
	}
	monitor, activity, err := s.authoriseCapture(userID, payload)
	if err != nil {
		return nil, err
	}
	receipt := s.now().UTC()
	if payload.DeviceTimestamp.After(receipt.Add(maxAcceptedSkew)) {
		return nil, fmt.Errorf("%w: the device timestamp is in the future", ErrInvalidRecord)
	}
	if payload.DeviceTimestamp.Before(activity.ReportingPeriodStart.Add(-365 * 24 * time.Hour)) {
		return nil, fmt.Errorf("%w: the device timestamp is outside the methodology period", ErrInvalidRecord)
	}
	height, err := money.ParseQuantity(payload.AverageHeightMetres)
	if err != nil {
		return nil, fmt.Errorf("%w: average height must be a decimal number of metres", ErrInvalidRecord)
	}
	species, err := json.Marshal(strings.TrimSpace(payload.Species))
	if err != nil {
		return nil, err
	}
	manifest, err := canonicalManifest(payload.EvidenceManifest)
	if err != nil {
		return nil, err
	}
	signed, err := s.verifyAttestation(monitor, payload)
	if err != nil {
		return nil, err
	}
	commitment, err := contentCommitment(payload, activity, manifest, money.Quantity(height))
	if err != nil {
		return nil, err
	}
	lowAccuracy := payload.Location.AccuracyMetres > lowAccuracyMetres

	var existing models.MonitoringRecord
	lookup := s.db.Where("monitor_id = ? AND client_id = ?", userID, payload.ClientID).First(&existing).Error
	switch {
	case errors.Is(lookup, gorm.ErrRecordNotFound):
	case lookup != nil:
		return nil, fmt.Errorf("the record lookup failed")
	case existing.ContentCommitment == commitment:
		return &Result{
			ClientID: payload.ClientID, MonitoringRecordID: existing.ID, Status: existing.SyncStatus,
			Signed: existing.Signed, LowAccuracy: existing.GPSAccuracyMeters > lowAccuracyMetres,
		}, nil
	default:
		return &Result{
			ClientID: payload.ClientID, MonitoringRecordID: existing.ID, Status: models.SyncConflict,
			Error: "A different version of this record is already stored. Submit a corrected record with a new clientId.",
			Conflict: &Conflict{
				ServerVersion: fmt.Sprintf("%d", existing.Version),
				Detail:        fmt.Sprintf("Stored commitment %s differs from the uploaded commitment %s.", short(existing.ContentCommitment), short(commitment)),
			},
		}, nil
	}

	record := models.MonitoringRecord{
		ID: uuid.NewString(), ActivityID: activity.ID, MonitorID: userID, ClientID: payload.ClientID,
		DeviceTimestamp: payload.DeviceTimestamp.UTC(), ServerReceiptTime: receipt,
		MethodologyVersion: activity.MethodologyVersion, SchemaVersion: payload.SchemaVersion,
		Latitude: payload.Location.Latitude, Longitude: payload.Location.Longitude,
		GPSAccuracyMeters: payload.Location.AccuracyMetres, AliveCount: payload.AliveCount,
		PlantedCount: payload.PlantedCount, AverageHeightMetres: money.Quantity(height),
		Species: datatypes.JSON(species), Attestation: payload.Attestation.Statement,
		AttestationHash:  canon.HashBytes([]byte(payload.Attestation.Statement)),
		EvidenceManifest: datatypes.JSON(manifest), ContentCommitment: commitment,
		Signed: signed, Version: 1, SyncStatus: models.SyncSubmitted, CreatedAt: receipt, UpdatedAt: receipt,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return nil, fmt.Errorf("the monitoring record could not be stored")
	}
	if err := s.markActivityMonitoring(activity); err != nil {
		return nil, err
	}
	if err := s.recordEvidenceClaim(ctx, activity, record); err != nil {
		// A missing early anchor must never invalidate an accepted record.
		_ = err
	}
	return &Result{
		ClientID: payload.ClientID, MonitoringRecordID: record.ID, Status: models.SyncSubmitted,
		Signed: signed, LowAccuracy: lowAccuracy,
	}, nil
}

// UploadEvidence stores one media file against an accepted record and refuses it
// when the bytes do not hash to a value in the record's manifest.
func (s *Service) UploadEvidence(userID, recordID string, manifest EvidenceManifestItem, upload io.ReadCloser) (models.EvidenceObject, error) {
	defer upload.Close()
	var record models.MonitoringRecord
	if err := s.db.Where("id = ?", recordID).First(&record).Error; err != nil {
		return models.EvidenceObject{}, ErrNotFound
	}
	if err := s.AssertActivityAccess(userID, record.ActivityID); err != nil {
		return models.EvidenceObject{}, err
	}
	if !declaresHash(record, manifest.SHA256) {
		return models.EvidenceObject{}, fmt.Errorf("%w: the file hash is not in this record's evidence manifest", ErrInvalidRecord)
	}
	if manifest.Kind != "photo" && manifest.Kind != "voice" {
		return models.EvidenceObject{}, fmt.Errorf("%w: evidence kind must be photo or voice", ErrInvalidRecord)
	}
	objectID := uuid.NewString()
	uri, actual, _, err := s.evidence.Put(recordID, objectID, manifest.SHA256, upload)
	if err != nil {
		return models.EvidenceObject{}, evidenceError(err)
	}
	object := models.EvidenceObject{
		ID: objectID, MonitoringRecordID: recordID, Kind: manifest.Kind, StorageURI: uri,
		SHA256: actual, MediaType: manifest.MediaType, AccessClass: "restricted",
		CapturedAt: record.DeviceTimestamp.UTC(), CreatedAt: s.now().UTC(),
	}
	if err := s.db.Create(&object).Error; err != nil {
		return models.EvidenceObject{}, fmt.Errorf("the evidence reference could not be stored")
	}
	return object, nil
}

func (s *Service) EvidenceFor(recordID string) ([]models.EvidenceObject, error) {
	var objects []models.EvidenceObject
	err := s.db.Where("monitoring_record_id = ?", recordID).Order("created_at asc").Find(&objects).Error
	return objects, err
}

func (s *Service) RecordsFor(activityID string) ([]models.MonitoringRecord, error) {
	var records []models.MonitoringRecord
	err := s.db.Where("activity_id = ?", activityID).Order("device_timestamp asc").Find(&records).Error
	return records, err
}

func (s *Service) authoriseCapture(userID string, payload Payload) (models.User, models.ConservationActivity, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return user, models.ConservationActivity{}, ErrForbidden
	}
	var activity models.ConservationActivity
	if err := s.db.Where("id = ?", payload.ActivityID).First(&activity).Error; err != nil {
		return user, activity, fmt.Errorf("%w: the activity does not exist", ErrInvalidRecord)
	}
	if activity.MethodologyVersion != payload.MethodologyVersion {
		return user, activity, fmt.Errorf("%w: methodology version %q is not the version this activity was registered under", ErrInvalidRecord, payload.MethodologyVersion)
	}
	var plot models.Plot
	if err := s.db.Where("id = ? AND project_id = ?", payload.PlotID, activity.ProjectID).First(&plot).Error; err != nil {
		return user, activity, fmt.Errorf("%w: the plot is not part of this activity's project", ErrInvalidRecord)
	}
	if plot.ID != activity.PlotID {
		return user, activity, fmt.Errorf("%w: the plot does not belong to this activity", ErrInvalidRecord)
	}
	switch user.Role {
	case models.RoleAdmin, models.RoleManager:
	case models.RoleMonitor:
		if plot.AssignedMonitorID != "" && plot.AssignedMonitorID != userID {
			return user, activity, ErrForbidden
		}
	default:
		return user, activity, ErrForbidden
	}
	if activity.Status == models.ActivityVerified || activity.Status == models.ActivityRejected {
		return user, activity, fmt.Errorf("%w: this activity is closed to new monitoring records", ErrInvalidRecord)
	}
	return user, activity, nil
}

func validateShape(payload Payload) error {
	if _, err := uuid.Parse(payload.ClientID); err != nil {
		return fmt.Errorf("%w: clientId must be a UUID", ErrInvalidRecord)
	}
	if payload.SchemaVersion == "" || len(payload.SchemaVersion) > 40 {
		return fmt.Errorf("%w: schemaVersion is required", ErrInvalidRecord)
	}
	if payload.AliveCount < 0 || payload.PlantedCount < 0 {
		return fmt.Errorf("%w: counts cannot be negative", ErrInvalidRecord)
	}
	if payload.PlantedCount > 0 && payload.AliveCount > payload.PlantedCount {
		return fmt.Errorf("%w: the alive count exceeds the planted count", ErrInvalidRecord)
	}
	if payload.Location.Latitude < -90 || payload.Location.Latitude > 90 ||
		payload.Location.Longitude < -180 || payload.Location.Longitude > 180 {
		return fmt.Errorf("%w: the coordinates are outside valid ranges", ErrInvalidRecord)
	}
	if payload.Location.AccuracyMetres < 0 || payload.Location.AccuracyMetres > 10000 {
		return fmt.Errorf("%w: the GPS accuracy reading is implausible", ErrInvalidRecord)
	}
	if strings.TrimSpace(payload.Attestation.Statement) == "" {
		return fmt.Errorf("%w: a monitor attestation statement is required", ErrInvalidRecord)
	}
	for _, item := range payload.EvidenceManifest {
		if len(item.SHA256) != 64 || strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("%w: every manifest entry needs a name and a 64-character sha256", ErrInvalidRecord)
		}
		if item.Kind != "photo" && item.Kind != "voice" {
			return fmt.Errorf("%w: evidence kind must be photo or voice", ErrInvalidRecord)
		}
		if item.Size <= 0 {
			return fmt.Errorf("%w: evidence size must be positive", ErrInvalidRecord)
		}
	}
	return nil
}

// verifyAttestation returns true only when the captured signature verifies against
// the monitor's provisioned identity key. Unsigned captures are accepted and are
// reported as unsigned.
func (s *Service) verifyAttestation(monitor models.User, payload Payload) (bool, error) {
	if payload.Attestation.Signature == "" {
		return false, nil
	}
	provision, err := s.identities.ForUser(monitor.ID)
	if err != nil || provision.DID == "" || payload.Attestation.IdentityDID != provision.DID {
		return false, fmt.Errorf("%w: the attestation DID does not match this account's provisioned identity", ErrInvalidRecord)
	}
	canonical, err := attestationPayload(payload)
	if err != nil {
		return false, err
	}
	if _, err := s.identities.VerifyAttestation(monitor.ID, canonical, payload.Attestation.Signature); err != nil {
		return false, fmt.Errorf("%w: the monitor signature could not be verified", ErrInvalidRecord)
	}
	return true, nil
}

func attestationPayload(payload Payload) ([]byte, error) {
	return canon.JSON(map[string]any{
		"clientId": payload.ClientID, "activityId": payload.ActivityID, "plotId": payload.PlotID,
		"deviceTimestamp": payload.DeviceTimestamp.UTC().Format(time.RFC3339Nano),
		"aliveCount":      payload.AliveCount, "plantedCount": payload.PlantedCount,
		"averageHeightMetres": payload.AverageHeightMetres, "species": payload.Species,
		"statement": payload.Attestation.Statement,
	})
}

func canonicalManifest(items []EvidenceManifestItem) ([]byte, error) {
	sorted := append([]EvidenceManifestItem(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].SHA256 == sorted[j].SHA256 {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].SHA256 < sorted[j].SHA256
	})
	return canon.JSON(sorted)
}

func contentCommitment(payload Payload, activity models.ConservationActivity, manifest []byte, height string) (string, error) {
	var evidenceRoot any = json.RawMessage(manifest)
	if len(manifest) == 0 {
		evidenceRoot = []any{}
	}
	hash, _, err := canon.Hash(map[string]any{
		"clientId": payload.ClientID, "activityId": payload.ActivityID, "plotId": payload.PlotID,
		"projectId": activity.ProjectID, "methodologyVersion": payload.MethodologyVersion,
		"schemaVersion":   payload.SchemaVersion,
		"deviceTimestamp": payload.DeviceTimestamp.UTC().Format(time.RFC3339Nano),
		"latitude":        payload.Location.Latitude, "longitude": payload.Location.Longitude,
		"accuracyMetres": payload.Location.AccuracyMetres, "aliveCount": payload.AliveCount,
		"plantedCount": payload.PlantedCount, "averageHeightMetres": height,
		"species": payload.Species, "observations": payload.Observations,
		"attestation": payload.Attestation.Statement, "evidenceManifest": evidenceRoot,
	})
	return hash, err
}

func declaresHash(record models.MonitoringRecord, sha256 string) bool {
	var manifest []EvidenceManifestItem
	if err := json.Unmarshal(record.EvidenceManifest, &manifest); err != nil {
		return false
	}
	for _, item := range manifest {
		if strings.EqualFold(item.SHA256, sha256) {
			return true
		}
	}
	return false
}

// ManifestFor decodes a stored record's declared evidence manifest.
func ManifestFor(record models.MonitoringRecord) []EvidenceManifestItem {
	var manifest []EvidenceManifestItem
	_ = json.Unmarshal(record.EvidenceManifest, &manifest)
	return manifest
}

func evidenceError(err error) error {
	switch {
	case errors.Is(err, evidence.ErrHashMismatch):
		return fmt.Errorf("%w: the uploaded bytes do not match the declared sha256", ErrInvalidRecord)
	case errors.Is(err, evidence.ErrTooLarge):
		return fmt.Errorf("%w: the uploaded file is larger than the permitted size", ErrInvalidRecord)
	case errors.Is(err, evidence.ErrInvalidDigest):
		return fmt.Errorf("%w: the declared sha256 is not a hex digest", ErrInvalidRecord)
	default:
		return fmt.Errorf("the evidence file could not be stored")
	}
}

func publicMessage(err error) string {
	switch {
	case errors.Is(err, ErrConflict):
		return "A different version of this record already exists on the server. Create a corrected record instead of overwriting it."
	case errors.Is(err, ErrForbidden):
		return "This account is not assigned to the plot for this record."
	case errors.Is(err, ErrInvalidRecord):
		return strings.TrimPrefix(err.Error(), ErrInvalidRecord.Error()+": ")
	default:
		return "The record could not be synchronised. Retry and contact support if it persists."
	}
}

func short(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

// DeltaChange is one server-side change an offline client must reconcile: a record
// receipt, a correction, or an activity status transition.
type DeltaChange struct {
	Kind       string    `json:"kind"`
	ID         string    `json:"id"`
	ProjectID  string    `json:"projectId"`
	ActivityID string    `json:"activityId"`
	Status     string    `json:"status"`
	Version    int       `json:"version"`
	ChangedAt  time.Time `json:"changedAt"`
}

// DeltaResponse is the change feed plus the server clock, which the client needs to
// store a cursor it can trust after a local clock drift.
type DeltaResponse struct {
	Changes    []DeltaChange `json:"changes"`
	ServerTime time.Time     `json:"serverTime"`
	NextCursor time.Time     `json:"nextCursor"`
	Truncated  bool          `json:"truncated"`
}

const deltaLimit = 500

// Delta reports what changed on the server since the cursor the client sent. Only
// projects the caller can access are considered, so a device never learns the
// existence of a site it is not assigned to.
func (s *Service) Delta(userID string, since time.Time) (DeltaResponse, error) {
	projects, err := s.ProjectsForUser(userID)
	if err != nil {
		return DeltaResponse{}, err
	}
	if len(projects) == 0 {
		return DeltaResponse{Changes: []DeltaChange{}, ServerTime: s.now().UTC(), NextCursor: since.UTC()}, nil
	}
	projectIDs := make([]string, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ID)
	}
	now := s.now().UTC()

	var activities []models.ConservationActivity
	if err := s.db.Where("project_id IN ? AND updated_at > ?", projectIDs, since).
		Order("updated_at asc").Find(&activities).Error; err != nil {
		return DeltaResponse{}, err
	}
	activityProject := make(map[string]string, len(activities))
	changes := make([]DeltaChange, 0, len(activities))
	for _, activity := range activities {
		activityProject[activity.ID] = activity.ProjectID
		changes = append(changes, DeltaChange{
			Kind: "activity", ID: activity.ID, ProjectID: activity.ProjectID,
			ActivityID: activity.ID, Status: activity.Status, Version: 0, ChangedAt: activity.UpdatedAt.UTC(),
		})
	}

	var records []models.MonitoringRecord
	if err := s.db.Where("sync_status <> ? AND updated_at > ?", "queued", since).
		Order("updated_at asc").Find(&records).Error; err != nil {
		return DeltaResponse{}, err
	}
	truncated := false
	for _, record := range records {
		projectID, ok := activityProject[record.ActivityID]
		if !ok {
			var activity models.ConservationActivity
			if err := s.db.Where("id = ?", record.ActivityID).First(&activity).Error; err != nil {
				continue
			}
			projectID = activity.ProjectID
		}
		changes = append(changes, DeltaChange{
			Kind: "monitoring_record", ID: record.ID, ProjectID: projectID,
			ActivityID: record.ActivityID, Status: record.SyncStatus,
			Version: record.Version, ChangedAt: record.UpdatedAt.UTC(),
		})
		if len(changes) >= deltaLimit {
			truncated = true
			break
		}
	}
	cursor := since.UTC()
	for _, change := range changes {
		if change.ChangedAt.After(cursor) {
			cursor = change.ChangedAt
		}
	}
	return DeltaResponse{Changes: changes, ServerTime: now, NextCursor: cursor, Truncated: truncated}, nil
}
