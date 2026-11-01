// Package provenance builds canonical achievement commitments and anchors them to
// the configured chain. Nothing is reported as anchored until a chain readback
// returns the same commitment and issuer.
package provenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const SchemaVersion = "baharimrv-anchor/1"

const (
	StatusPending    = "pending"
	StatusAnchored   = "anchored"
	StatusFailed     = "failed"
	StatusSuperseded = "superseded"
	StatusRevoked    = "revoked"
	StatusDisputed   = "disputed"
)

var (
	ErrNotRetryable = errors.New("this achievement has nothing to re-anchor")
	ErrNotFound     = errors.New("provenance event not found")
)

// Claim is the public, versioned commitment payload. It deliberately excludes
// private coordinates, names, phone numbers, raw media and payment details.
type Claim struct {
	SchemaVersion           string    `json:"schema_version"`
	AchievementID           string    `json:"achievement_id"`
	ProjectCommitment       string    `json:"project_commitment"`
	ClaimType               string    `json:"claim_type"`
	ActivityType            string    `json:"activity_type"`
	MethodologyVersionHash  string    `json:"methodology_version_hash"`
	ReportingPeriodStart    time.Time `json:"reporting_period_start"`
	ReportingPeriodEnd      time.Time `json:"reporting_period_end"`
	MeasurementManifestHash string    `json:"measurement_manifest_hash"`
	MeasurementBasis        string    `json:"measurement_basis"`
	EvidenceRoot            string    `json:"evidence_root"`
	SatelliteHash           string    `json:"satellite_corroboration_hash"`
	VerifierAttestationHash string    `json:"verifier_attestation_hash"`
	VerifierDID             string    `json:"verifier_did"`
	PreviousEventID         string    `json:"previous_event_id,omitempty"`
	ApprovedAt              time.Time `json:"approved_at"`
}

// Input describes one achievement claim before canonicalisation.
type Input struct {
	AchievementID           string
	ProjectID               string
	ClaimType               string
	ActivityType            string
	MethodologyVersion      string
	ReportingPeriodStart    time.Time
	ReportingPeriodEnd      time.Time
	Measurements            map[string]any
	MeasurementBasis        string
	EvidenceRoot            string
	SatelliteHash           string
	VerifierAttestationHash string
	VerifierDID             string
	IssuerDID               string
	PreviousEventID         string
	Kind                    string
}

type Service struct {
	db               *gorm.DB
	chain            chain.Adapter
	explorerOverride string
	now              func() time.Time
}

func NewService(db *gorm.DB, adapter chain.Adapter, explorerOverride string) *Service {
	return &Service{db: db, chain: adapter, explorerOverride: explorerOverride, now: time.Now}
}

func (s *Service) Mode() string    { return s.chain.Mode() }
func (s *Service) Network() string { return s.chain.Network() }

// Commitment canonicalises the claim and returns the canonical bytes with their
// SHA-256 hex digest. The same inputs always produce the same commitment.
func Commitment(in Input, measurementsHash string, approvedAt time.Time) (Claim, []byte, string, error) {
	methodologyHash := canon.HashBytes([]byte(in.MethodologyVersion))
	claim := Claim{
		SchemaVersion:           SchemaVersion,
		AchievementID:           in.AchievementID,
		ProjectCommitment:       canon.HashBytes([]byte(in.ProjectID)),
		ClaimType:               in.ClaimType,
		ActivityType:            in.ActivityType,
		MethodologyVersionHash:  methodologyHash,
		ReportingPeriodStart:    in.ReportingPeriodStart.UTC().Truncate(time.Second),
		ReportingPeriodEnd:      in.ReportingPeriodEnd.UTC().Truncate(time.Second),
		MeasurementManifestHash: measurementsHash,
		MeasurementBasis:        in.MeasurementBasis,
		EvidenceRoot:            in.EvidenceRoot,
		SatelliteHash:           in.SatelliteHash,
		VerifierAttestationHash: in.VerifierAttestationHash,
		VerifierDID:             in.VerifierDID,
		PreviousEventID:         in.PreviousEventID,
		ApprovedAt:              approvedAt.UTC().Truncate(time.Second),
	}
	hash, canonical, err := canon.Hash(claim)
	if err != nil {
		return Claim{}, nil, "", err
	}
	return claim, canonical, hash, nil
}

// MeasurementsHash canonicalises the measurement manifest independently so the
// same numbers ordered differently still produce one digest.
func MeasurementsHash(measurements map[string]any) (string, error) {
	if measurements == nil {
		measurements = map[string]any{}
	}
	hash, _, err := canon.Hash(measurements)
	return hash, err
}

// Record persists the claim and attempts one anchor with readback confirmation.
// A chain outage leaves the event pending; it is never reported as anchored.
func (s *Service) Record(ctx context.Context, in Input) (models.ProvenanceEvent, error) {
	if in.AchievementID == "" || in.ProjectID == "" || in.ClaimType == "" || in.IssuerDID == "" {
		return models.ProvenanceEvent{}, fmt.Errorf("achievement, project, claim type and issuer DID are required")
	}
	measurementHash, err := MeasurementsHash(in.Measurements)
	if err != nil {
		return models.ProvenanceEvent{}, err
	}
	now := s.now().UTC()
	claim, canonical, hash, err := Commitment(in, measurementHash, now)
	if err != nil {
		return models.ProvenanceEvent{}, err
	}
	version, err := s.nextVersion(in.AchievementID, in.ClaimType)
	if err != nil {
		return models.ProvenanceEvent{}, err
	}
	event := models.ProvenanceEvent{
		ID: uuid.NewString(), ActivityID: in.AchievementID, ClaimType: in.ClaimType,
		Version: version, Commitment: hash, CanonicalJSON: canonical,
		PreviousEventID: in.PreviousEventID, IssuerDID: in.IssuerDID,
		Status: StatusPending, Network: s.chain.Network(), CreatedAt: now,
	}
	if err := s.db.Create(&event).Error; err != nil {
		return event, fmt.Errorf("provenance event could not be recorded: %w", err)
	}
	kind := in.Kind
	if kind == "" {
		kind = in.ClaimType
	}
	return s.anchor(ctx, event, kind, claim.PreviousEventID)
}

// Retry re-anchors an existing commitment. The original commitment is preserved,
// and every attempt is recorded.
func (s *Service) Retry(ctx context.Context, activityID string) (models.ProvenanceEvent, error) {
	event, err := s.LatestUnanchored(activityID)
	if err != nil {
		return models.ProvenanceEvent{}, err
	}
	return s.anchor(ctx, event, event.ClaimType, event.PreviousEventID)
}

// LatestUnanchored returns the newest event for an activity that still needs an
// anchor confirmation.
func (s *Service) LatestUnanchored(activityID string) (models.ProvenanceEvent, error) {
	var event models.ProvenanceEvent
	err := s.db.Where("activity_id = ? AND status IN ?", activityID, []string{StatusPending, StatusFailed}).
		Order("created_at desc").First(&event).Error
	if err != nil {
		return event, ErrNotRetryable
	}
	return event, nil
}

// Events lists an activity's provenance history in chronological order.
func (s *Service) Events(activityID string) ([]models.ProvenanceEvent, error) {
	var events []models.ProvenanceEvent
	err := s.db.Where("activity_id = ?", activityID).Order("created_at asc").Find(&events).Error
	return events, err
}

func (s *Service) nextVersion(activityID, claimType string) (int, error) {
	var current int64
	err := s.db.Model(&models.ProvenanceEvent{}).Where("activity_id = ? AND claim_type = ?", activityID, claimType).
		Select("COALESCE(MAX(version), 0)").Scan(&current).Error
	return int(current) + 1, err
}

func (s *Service) anchor(ctx context.Context, event models.ProvenanceEvent, kind string, previousEventID string) (models.ProvenanceEvent, error) {
	attempt := s.nextAttemptNumber(event.ID)
	record := models.AnchorAttempt{
		ID: uuid.NewString(), ProvenanceEventID: event.ID, Attempt: attempt,
		Status: StatusPending, CreatedAt: s.now().UTC(),
	}
	previousObject, err := s.previousObject(previousEventID)
	if err != nil {
		return event, err
	}
	result, anchorErr := s.chain.Anchor(ctx, chain.AnchorRequest{
		IdempotencyKey:   event.ID,
		Kind:             kind,
		Commitment:       event.Commitment,
		CanonicalJSON:    []byte(event.CanonicalJSON),
		IssuerDID:        event.IssuerDID,
		PreviousObjectID: previousObject,
	})
	if anchorErr != nil {
		record.Status = StatusFailed
		record.Error = safeError(anchorErr)
		event.Status = StatusFailed
		if errors.Is(anchorErr, chain.ErrUnavailable) {
			event.Status = StatusPending
			record.Status = StatusPending
		}
		_ = s.persist(event, record)
		return event, nil
	}
	record.Status = "submitted"
	record.TransactionRef = result.TransactionRef
	event.TransactionRef = result.TransactionRef
	event.ObjectID = result.ObjectID

	readback, readErr := s.chain.ReadAnchor(ctx, result.ObjectID)
	if readErr == nil && readback.Commitment == event.Commitment && readback.IssuerDID == event.IssuerDID && !readback.Revoked {
		now := s.now().UTC()
		event.Status = StatusAnchored
		event.ConfirmedAt = &now
		record.Status = StatusAnchored
	}
	if readErr != nil && !errors.Is(readErr, chain.ErrNotFound) {
		record.Error = safeError(readErr)
	}
	_ = s.persist(event, record)
	return event, nil
}

func (s *Service) persist(event models.ProvenanceEvent, attempt models.AnchorAttempt) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		return tx.Model(&models.ProvenanceEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
			"status": event.Status, "transaction_ref": event.TransactionRef, "object_id": event.ObjectID,
			"confirmed_at": event.ConfirmedAt, "network": event.Network,
		}).Error
	})
}

func (s *Service) previousObject(previousEventID string) (string, error) {
	if previousEventID == "" {
		return "", nil
	}
	var previous models.ProvenanceEvent
	if err := s.db.Where("id = ?", previousEventID).First(&previous).Error; err != nil {
		return "", fmt.Errorf("previous provenance event is unavailable")
	}
	return previous.ObjectID, nil
}

func (s *Service) nextAttemptNumber(eventID string) int {
	var current int64
	_ = s.db.Model(&models.AnchorAttempt{}).Where("provenance_event_id = ?", eventID).
		Select("COALESCE(MAX(attempt), 0)").Scan(&current)
	return int(current) + 1
}

// ExplorerTransaction derives the explorer link for the configured network.
func (s *Service) ExplorerTransaction(event models.ProvenanceEvent) string {
	return chain.ExplorerTransaction(s.chain.Network(), s.explorerOverride, event.TransactionRef)
}

// ExplorerObject derives the explorer link for the anchored object.
func (s *Service) ExplorerObject(event models.ProvenanceEvent) string {
	return chain.ExplorerObject(s.chain.Network(), s.explorerOverride, event.ObjectID)
}

// ReadAnchor performs a live chain read. Callers use it for independent public
// verification instead of trusting stored rows.
func (s *Service) ReadAnchor(ctx context.Context, objectID string) (chain.AnchorRecord, error) {
	return s.chain.ReadAnchor(ctx, objectID)
}

// ResolveIdentity performs a live DID document read.
func (s *Service) ResolveIdentity(ctx context.Context, didDocumentRef string) (bool, error) {
	return s.chain.ResolveIdentity(ctx, didDocumentRef)
}

// EventView is the client-facing projection of a provenance event. Chain
// references are only present when a readback confirmed them, and the network
// names which ledger the reference came from: a sandbox reference must never
// be rendered as an IOTA testnet claim.
type EventView struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Actor          string    `json:"actor"`
	Timestamp      time.Time `json:"timestamp"`
	Status         string    `json:"status"`
	Network        string    `json:"network,omitempty"`
	Commitment     string    `json:"commitment,omitempty"`
	ChainReference string    `json:"chainReference,omitempty"`
	ExplorerURL    string    `json:"explorerUrl,omitempty"`
	ObjectID       string    `json:"objectId,omitempty"`
	Version        int       `json:"version"`
	Detail         string    `json:"detail"`
}

// View projects a stored event. An unconfirmed event never exposes a chain
// reference as proof, even though the submission may have been accepted.
func (s *Service) View(event models.ProvenanceEvent, actor string) EventView {
	confirmed := event.Status == StatusAnchored
	reference := ""
	if confirmed {
		reference = event.TransactionRef
	}
	return EventView{
		ID: event.ID, Type: event.ClaimType, Actor: actor, Timestamp: event.CreatedAt,
		Status: event.Status, Network: s.PublicNetwork(),
		Commitment: event.Commitment, ChainReference: reference,
		ExplorerURL: s.ExplorerTransaction(event), ObjectID: event.ObjectID,
		Version: event.Version, Detail: detailFor(event, s.chain.Network()),
	}
}

// PublicNetwork names the ledger the API contract publishes for anchored
// references. A sandbox adapter is never reported as an IOTA network.
func (s *Service) PublicNetwork() string {
	return publicNetwork(s.chain.Network())
}

// publicNetwork names the adapter's network in the shape the API contract
// publishes. Sandbox stays sandbox.
func publicNetwork(network string) string {
	switch network {
	case "testnet":
		return "iota_testnet"
	case "mainnet":
		return "iota_mainnet"
	default:
		return "sandbox"
	}
}

// detailFor explains an event status. A sandbox readback is never described as
// confirmation by a public chain.
func detailFor(event models.ProvenanceEvent, network string) string {
	confirmed := "Commitment confirmed by chain readback."
	if network == "sandbox" {
		confirmed = "Commitment confirmed by the sandbox ledger readback. Not a public chain."
	}
	switch event.Status {
	case StatusAnchored:
		return confirmed
	case StatusFailed:
		return "Anchor attempt failed. Retry preserves the original commitment."
	case StatusSuperseded:
		return "Superseded by a later version of this claim."
	case StatusRevoked:
		return "Revoked by an authorised issuer."
	case StatusDisputed:
		return "Under dispute."
	default:
		return "Recorded and awaiting chain confirmation."
	}
}

func safeError(err error) string {
	message := err.Error()
	if len(message) > 300 {
		message = message[:300]
	}
	return message
}

// AnchorStatus reports what the chain actually says about one provenance event.
// A readback failure is returned as a status, never dressed up as confirmation.
type AnchorStatus struct {
	EventID     string `json:"eventId"`
	ActivityID  string `json:"activityId"`
	ClaimType   string `json:"claimType"`
	Version     int    `json:"version"`
	Status      string `json:"status"`
	Network     string `json:"network"`
	Mode        string `json:"mode"`
	Commitment  string `json:"commitment"`
	ObjectID    string `json:"objectId,omitempty"`
	Transaction string `json:"transactionReference,omitempty"`
	ExplorerURL string `json:"explorerUrl,omitempty"`
	ConfirmedAt string `json:"confirmedAt,omitempty"`
	Attempts    int    `json:"attempts"`
	LastError   string `json:"lastError,omitempty"`
	Readback    string `json:"readback"`
	CanRetry    bool   `json:"canRetry"`
}

func (s *Service) AnchorStatus(eventID string) (AnchorStatus, error) {
	var event models.ProvenanceEvent
	if err := s.db.Where("id = ?", eventID).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AnchorStatus{}, ErrNotFound
		}
		return AnchorStatus{}, err
	}
	status := AnchorStatus{
		EventID: event.ID, ActivityID: event.ActivityID, ClaimType: event.ClaimType,
		Version: event.Version, Status: event.Status, Network: event.Network, Mode: s.chain.Mode(),
		Commitment: event.Commitment, ObjectID: event.ObjectID, Transaction: event.TransactionRef,
	}
	if event.ConfirmedAt != nil {
		status.ConfirmedAt = event.ConfirmedAt.UTC().Format(time.RFC3339)
		status.ExplorerURL = s.ExplorerTransaction(event)
	}
	var attempts int64
	if err := s.db.Model(&models.AnchorAttempt{}).Where("provenance_event_id = ?", event.ID).
		Count(&attempts).Error; err != nil {
		return status, err
	}
	status.Attempts = int(attempts)
	var last models.AnchorAttempt
	if err := s.db.Where("provenance_event_id = ?", event.ID).Order("attempt desc").First(&last).Error; err == nil {
		status.LastError = last.Error
	}
	status.CanRetry = event.Status == StatusPending || event.Status == StatusFailed

	// Only a live adapter has something to read back. In sandbox the answer is
	// explicitly "no public chain", never an implied confirmation.
	switch {
	case s.chain.Mode() != "iota":
		status.Readback = "unavailable_in_sandbox"
	case event.ObjectID == "":
		status.Readback = "not_anchored"
	default:
		if _, err := s.chain.ReadAnchor(context.Background(), event.ObjectID); err != nil {
			status.Readback = "failed"
		} else {
			status.Readback = "confirmed"
		}
	}
	return status, nil
}
