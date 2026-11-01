// Package proof turns a conservation achievement into a versioned, downloadable
// proof document with independent public verification. Nothing in a proof is
// asserted that the backend cannot re-derive from stored evidence, attestations
// and confirmed chain readbacks.
package proof

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/evidence"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/baharilink/baharimrv/internal/satellite"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SchemaVersion names the snapshot layout so verification can reject foreign documents.
const SchemaVersion = "baharimrv-proof/1"

const (
	TypeCompletion = "completion_claim"
	TypeVerified   = "verified_achievement"

	// proofAction names the idempotency receipts this service writes to sync_actions.
	proofAction = "proof_generation"
)

var (
	ErrNotFound       = errors.New("proof not found")
	ErrForbidden      = errors.New("you do not have access to this achievement")
	ErrNotPublishable = errors.New("this achievement has no claim to prove")
)

type Service struct {
	db           *gorm.DB
	mrv          *mrv.Service
	identities   *identity.Service
	anchors      *provenance.Service
	imagery      *satellite.Service
	store        *evidence.Store
	registryMode string
	publicBase   string
	now          func() time.Time
}

func NewService(db *gorm.DB, projects *mrv.Service, identities *identity.Service, anchors *provenance.Service,
	imagery *satellite.Service, store *evidence.Store, registryMode, publicBaseURL string) *Service {
	return &Service{
		db: db, mrv: projects, identities: identities, anchors: anchors, imagery: imagery,
		store: store, registryMode: registryMode, publicBase: publicBaseURL, now: func() time.Time { return time.Now().UTC() },
	}
}

type VerifierBlock struct {
	Organization string    `json:"organization"`
	Decision     string    `json:"decision"`
	Timestamp    time.Time `json:"timestamp"`
	DID          string    `json:"did,omitempty"`
	Hash         string    `json:"attestation_hash"`
	Revoked      bool      `json:"revoked"`
}

type ChainBlock struct {
	Network              string `json:"network"`
	TransactionReference string `json:"transactionReference,omitempty"`
	Commitment           string `json:"commitment,omitempty"`
	ObjectID             string `json:"objectId,omitempty"`
	ExplorerURL          string `json:"explorerUrl,omitempty"`
	Confirmed            bool   `json:"confirmed"`
}

type EvidenceEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Access string `json:"access"`
	Kind   string `json:"kind"`
}

type ClaimBlock struct {
	Summary            string         `json:"summary"`
	ActivityType       string         `json:"activity_type"`
	MethodologyVersion string         `json:"methodology_version"`
	ReportingStart     time.Time      `json:"reporting_period_start"`
	ReportingEnd       time.Time      `json:"reporting_period_end"`
	Measurements       map[string]any `json:"measurements"`
	EvidenceRoot       string         `json:"evidence_root"`
	SatelliteHash      string         `json:"satellite_hash,omitempty"`
	UnsignedRecords    int            `json:"unsigned_records"`
	LowAccuracyRecords int            `json:"low_accuracy_records"`
}

type ModeBlock struct {
	Chain     string `json:"chain"`
	Satellite string `json:"satellite"`
	Registry  string `json:"registry"`
}

// Snapshot is the immutable proof content. Its canonical form is hashed into the
// manifest and rendered into every download format.
type Snapshot struct {
	Schema             string                 `json:"schema"`
	ProofID            string                 `json:"proof_id"`
	AchievementID      string                 `json:"achievement_id"`
	ProjectID          string                 `json:"project_id"`
	ProjectName        string                 `json:"project_name"`
	ProofType          string                 `json:"proof_type"`
	Version            int                    `json:"version"`
	GeneratedAt        time.Time              `json:"generated_at"`
	IssuerDID          string                 `json:"issuer_did"`
	IssuerDIDDocument  string                 `json:"issuer_did_document"`
	Verifier           *VerifierBlock         `json:"verifier"`
	Claim              ClaimBlock             `json:"claim"`
	EvidenceManifest   []EvidenceEntry        `json:"evidence_manifest"`
	Chain              *ChainBlock            `json:"chain"`
	RegistryReferences []string               `json:"registry_references"`
	Events             []provenance.EventView `json:"events"`
	Modes              ModeBlock              `json:"integration_modes"`
	SandboxNotice      string                 `json:"sandbox_notice"`
}

// View is the client-facing proof projection (camelCase for web and Android).
type View struct {
	ID                  string                 `json:"id"`
	AchievementID       string                 `json:"achievementId"`
	Version             int                    `json:"version"`
	ProofType           string                 `json:"proofType"`
	Status              string                 `json:"status"`
	GeneratedAt         time.Time              `json:"generatedAt"`
	ProjectName         string                 `json:"projectName"`
	ClaimSummary        string                 `json:"claimSummary"`
	IssuerDID           string                 `json:"issuerDid,omitempty"`
	VerifierAttestation *VerifierView          `json:"verifierAttestation,omitempty"`
	Chain               *ChainView             `json:"chain,omitempty"`
	RegistryReferences  []string               `json:"registryReferences"`
	EvidenceManifest    []EvidenceEntry        `json:"evidenceManifest"`
	Events              []provenance.EventView `json:"events"`
	AvailableFormats    []string               `json:"availableFormats"`
	SupersededBy        string                 `json:"supersededBy,omitempty"`
	Modes               ModeBlock              `json:"modes"`
	SandboxNotice       string                 `json:"sandboxNotice,omitempty"`
	ManifestHash        string                 `json:"manifestHash"`
}

type VerifierView struct {
	Organization string    `json:"organization"`
	Decision     string    `json:"decision"`
	Timestamp    time.Time `json:"timestamp"`
	DID          string    `json:"did,omitempty"`
}

type ChainView struct {
	Network              string `json:"network"`
	TransactionReference string `json:"transactionReference,omitempty"`
	Commitment           string `json:"commitment,omitempty"`
	ExplorerURL          string `json:"explorerUrl,omitempty"`
}

// Generate rebuilds the current state and stores a new proof version. The
// idempotency key lets a retried request return the proof it already created
// instead of superseding it with a second version.
func (s *Service) Generate(ctx context.Context, activityID, userID, idempotencyKey string) (View, error) {
	if replay, ok := s.replay(ctx, userID, idempotencyKey); ok {
		return replay, nil
	}
	activity, project, err := s.mrv.Activity(activityID)
	if err != nil {
		return View{}, err
	}
	if err := s.mrv.AssertProjectAccess(userID, project.ID); err != nil {
		return View{}, ErrForbidden
	}
	records, err := s.mrv.RecordsFor(activityID)
	if err != nil {
		return View{}, err
	}
	if len(records) == 0 {
		return View{}, ErrNotPublishable
	}
	events, err := s.anchors.Events(activityID)
	if err != nil {
		return View{}, err
	}
	proofType, event, err := selectClaim(activity, events)
	if err != nil {
		return View{}, err
	}
	attestation, verifierUser, err := s.attestationFor(activityID, proofType)
	if err != nil {
		return View{}, err
	}
	issuerProvision, err := s.provisionForDID(event.IssuerDID, proofType, attestation, activity)
	if err != nil {
		return View{}, err
	}
	manifest, unsigned, lowAccuracy, err := s.manifestFor(records)
	if err != nil {
		return View{}, err
	}
	summary := mrv.Summarise(records, activity)
	satelliteHash, satelliteNote := s.corroboration(activityID)
	registryRefs, registryNote := s.registryReferences(project.ID)
	actorNames, err := s.actorNames(events)
	if err != nil {
		return View{}, err
	}
	views := make([]provenance.EventView, 0, len(events))
	for _, item := range events {
		views = append(views, s.anchors.View(item, actorNames[item.IssuerDID]))
	}
	confirmedChain := event.Status == provenance.StatusAnchored
	chainBlock := &ChainBlock{
		Network: s.anchors.Network(), ObjectID: event.ObjectID, Commitment: event.Commitment,
		Confirmed: confirmedChain,
	}
	if confirmedChain {
		chainBlock.TransactionReference = event.TransactionRef
		chainBlock.ExplorerURL = s.anchors.ExplorerTransaction(event)
	}
	networkLabel := proofNetwork(s.anchors.Network())
	now := s.now()
	snapshot := Snapshot{
		Schema: SchemaVersion, ProofID: uuid.NewString(), AchievementID: activityID,
		ProjectID: project.ID, ProjectName: project.Name, ProofType: proofType,
		GeneratedAt: now, IssuerDID: event.IssuerDID, IssuerDIDDocument: issuerProvision.DIDDocumentRef,
		Verifier: attestationBlock(attestation, verifierUser),
		Claim: ClaimBlock{
			Summary: claimSummary(proofType, project.Name, summary), ActivityType: activity.ActivityType,
			MethodologyVersion: activity.MethodologyVersion, ReportingStart: activity.ReportingPeriodStart,
			ReportingEnd: activity.ReportingPeriodEnd, Measurements: summary,
			EvidenceRoot: mrv.EvidenceRoot(records), SatelliteHash: satelliteHash,
		},
		EvidenceManifest: manifest, Chain: chainBlock, RegistryReferences: registryRefs,
		Events:        views,
		Modes:         ModeBlock{Chain: s.anchors.Mode(), Satellite: s.imagery.Mode(), Registry: s.registryMode},
		SandboxNotice: sandboxNotice(s.anchors.Mode(), s.imagery.Mode(), s.registryMode, satelliteNote, registryNote),
	}
	snapshot.Claim.UnsignedRecords = unsigned
	snapshot.Claim.LowAccuracyRecords = lowAccuracy
	if snapshot.Claim.Measurements == nil {
		snapshot.Claim.Measurements = map[string]any{}
	}
	if snapshot.EvidenceManifest == nil {
		snapshot.EvidenceManifest = []EvidenceEntry{}
	}
	if snapshot.RegistryReferences == nil {
		snapshot.RegistryReferences = []string{}
	}

	digest, canonical, err := canon.Hash(snapshot)
	if err != nil {
		return View{}, err
	}
	version, err := s.nextVersion(activityID, proofType)
	if err != nil {
		return View{}, err
	}
	snapshot.Version = version
	snapshot.ProofType = proofType
	digest, canonical, err = canon.Hash(snapshot)
	if err != nil {
		return View{}, err
	}

	pdf, err := renderPDF(snapshot, networkLabel)
	if err != nil {
		return View{}, err
	}
	qr, err := renderQR(s.publicURL(snapshot.ProofID))
	if err != nil {
		return View{}, err
	}
	status := models.ProofPending
	if confirmedChain {
		status = models.ProofAnchored
	}
	if event.Status == provenance.StatusSuperseded {
		status = models.ProofSuperseded
	}
	if event.Status == provenance.StatusRevoked {
		status = models.ProofRevoked
	}
	if event.Status == provenance.StatusDisputed {
		status = models.ProofDisputed
	}

	document := models.ProofDocument{
		ID: snapshot.ProofID, ActivityID: activityID, Version: version, ProofType: proofType,
		Status: status, Snapshot: datatypes.JSON(canonical), ManifestHash: digest,
		PDF: pdf, QRCode: qr, CreatedBy: userID, CreatedAt: now,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ProofDocument{}).
			Where("activity_id = ? AND proof_type = ? AND status IN ?", activityID, proofType,
				[]string{models.ProofPending, models.ProofAnchored}).
			Updates(map[string]any{"status": models.ProofSuperseded, "superseded_by_id": document.ID}).Error; err != nil {
			return err
		}
		return tx.Create(&document).Error
	})
	if err != nil {
		return View{}, fmt.Errorf("the proof could not be stored")
	}
	view, err := s.view(document, false)
	if err != nil {
		return View{}, err
	}
	if err := s.persistIdempotency(ctx, userID, idempotencyKey, document.ID); err != nil {
		return View{}, err
	}
	return view, nil
}

// persistIdempotency records which proof a key produced. Without the receipt a
// retry would mint a second version, so a failed write is reported rather than
// swallowed.
func (s *Service) persistIdempotency(ctx context.Context, userID, key, proofID string) error {
	if key == "" {
		return nil
	}
	now := s.now().UTC()
	response, err := json.Marshal(map[string]string{"proofId": proofID})
	if err != nil {
		return err
	}
	hash, _, err := canon.Hash(map[string]any{"proofId": proofID})
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(&models.SyncAction{
		ID: uuid.NewString(), UserID: userID, ClientID: key, ActionType: proofAction,
		RequestHash: hash, Status: "generated", Response: datatypes.JSON(response),
		CreatedAt: now, UpdatedAt: now,
	}).Error
}

// replay returns the proof a key already generated for this user.
func (s *Service) replay(ctx context.Context, userID, key string) (View, bool) {
	if key == "" {
		return View{}, false
	}
	var action models.SyncAction
	if err := s.db.WithContext(ctx).Where("user_id = ? AND client_id = ? AND action_type = ?", userID, key, proofAction).
		First(&action).Error; err != nil {
		return View{}, false
	}
	var payload struct {
		ProofID string `json:"proofId"`
	}
	if err := json.Unmarshal(action.Response, &payload); err != nil || payload.ProofID == "" {
		return View{}, false
	}
	view, err := s.Get(payload.ProofID, false)
	if err != nil {
		return View{}, false
	}
	return view, true
}

func (s *Service) provisionForDID(did string, proofType string, attestation *models.VerifierAttestation, activity models.ConservationActivity) (models.IdentityProvision, error) {
	userID := activity.ManagerID
	if proofType == TypeVerified && attestation != nil {
		userID = attestation.VerifierID
	}
	provision, err := s.identities.ForUser(userID)
	if err != nil {
		return provision, ErrNotPublishable
	}
	if provision.DID == "" {
		return provision, ErrNotPublishable
	}
	if did != "" && provision.DID != did {
		return provision, domain.Reject("the anchoring identity no longer matches the provisioned DID")
	}
	return provision, nil
}

// Get returns a stored proof, projected for operators (full manifest) or the public.
func (s *Service) Get(proofID string, publicOnly bool) (View, error) {
	var document models.ProofDocument
	if err := s.db.Where("id = ?", proofID).First(&document).Error; err != nil {
		return View{}, ErrNotFound
	}
	return s.view(document, publicOnly)
}

// ForAchievement lists every proof version issued for an activity.
func (s *Service) ForAchievement(activityID, userID string) ([]View, error) {
	_, project, err := s.mrv.Activity(activityID)
	if err != nil {
		return nil, err
	}
	if err := s.mrv.AssertProjectAccess(userID, project.ID); err != nil {
		return nil, ErrForbidden
	}
	var documents []models.ProofDocument
	if err := s.db.Where("activity_id = ?", activityID).Order("created_at desc").Find(&documents).Error; err != nil {
		return nil, err
	}
	views := make([]View, 0, len(documents))
	for _, document := range documents {
		view, err := s.view(document, false)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// DownloadFor releases artefact bytes only after the requester's role over the
// achievement's project has been re-checked server-side.
func (s *Service) DownloadFor(proofID, format, userID string) (string, string, []byte, error) {
	view, err := s.Get(proofID, false)
	if err != nil {
		return "", "", nil, err
	}
	if err := s.mrv.AssertActivityAccess(userID, view.AchievementID); err != nil {
		return "", "", nil, ErrForbidden
	}
	return s.Download(proofID, format)
}

// Download returns the requested artefact rendered from the stored snapshot.
func (s *Service) Download(proofID, format string) (name string, mediaType string, body []byte, err error) {
	var document models.ProofDocument
	if err := s.db.Where("id = ?", proofID).First(&document).Error; err != nil {
		return "", "", nil, ErrNotFound
	}
	snapshot, err := decodeSnapshot(document.Snapshot)
	if err != nil {
		return "", "", nil, err
	}
	base := fmt.Sprintf("baharimrv-proof-%s-v%d", document.ID, document.Version)
	switch format {
	case "pdf":
		pdf, renderErr := renderPDF(snapshot, proofNetwork(s.anchors.Network()))
		if renderErr != nil {
			return "", "", nil, renderErr
		}
		return base + ".pdf", "application/pdf", pdf, nil
	case "json":
		return base + ".json", "application/json", append(document.Snapshot, '\n'), nil
	case "zip":
		archive, zipErr := s.renderZip(snapshot, document, base)
		if zipErr != nil {
			return "", "", nil, zipErr
		}
		return base + ".zip", "application/zip", archive, nil
	default:
		return "", "", nil, domain.Invalid("unsupported proof format")
	}
}

// PublicURL is the permanent link encoded in the QR and printed on the document.
func (s *Service) PublicURL(proofID string) string { return s.publicURL(proofID) }

func (s *Service) publicURL(proofID string) string {
	if s.publicBase == "" {
		return "/proof/" + proofID
	}
	return s.publicBase + "/proof/" + proofID
}

func (s *Service) view(document models.ProofDocument, publicOnly bool) (View, error) {
	snapshot, err := decodeSnapshot(document.Snapshot)
	if err != nil {
		return View{}, err
	}
	manifest := snapshot.EvidenceManifest
	if publicOnly {
		manifest = publicEntries(manifest)
	}
	view := View{
		ID: document.ID, AchievementID: snapshot.AchievementID, Version: document.Version,
		ProofType: document.ProofType, Status: document.Status, GeneratedAt: snapshot.GeneratedAt,
		ProjectName: snapshot.ProjectName, ClaimSummary: snapshot.Claim.Summary,
		IssuerDID: snapshot.IssuerDID, Chain: chainView(snapshot.Chain),
		RegistryReferences: snapshot.RegistryReferences, EvidenceManifest: manifest,
		Events: snapshot.Events, AvailableFormats: []string{"pdf", "json", "zip"},
		Modes: snapshot.Modes, SandboxNotice: snapshot.SandboxNotice, ManifestHash: document.ManifestHash,
	}
	if view.RegistryReferences == nil {
		view.RegistryReferences = []string{}
	}
	if view.EvidenceManifest == nil {
		view.EvidenceManifest = []EvidenceEntry{}
	}
	if view.Events == nil {
		view.Events = []provenance.EventView{}
	}
	if snapshot.Verifier != nil && (!publicOnly || !snapshot.Verifier.Revoked) {
		view.VerifierAttestation = &VerifierView{
			Organization: snapshot.Verifier.Organization, Decision: snapshot.Verifier.Decision,
			Timestamp: snapshot.Verifier.Timestamp, DID: snapshot.Verifier.DID,
		}
	}
	if document.Status == models.ProofSuperseded && document.SupersededByID != "" {
		view.SupersededBy = document.SupersededByID
	}
	return view, nil
}

func chainView(block *ChainBlock) *ChainView {
	if block == nil || !block.Confirmed {
		return nil
	}
	return &ChainView{
		Network: proofNetwork(block.Network), TransactionReference: block.TransactionReference,
		Commitment: block.Commitment, ExplorerURL: block.ExplorerURL,
	}
}

func publicEntries(entries []EvidenceEntry) []EvidenceEntry {
	out := make([]EvidenceEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Access == "public" {
			out = append(out, entry)
		}
	}
	return out
}

func decodeSnapshot(raw []byte) (Snapshot, error) {
	var snapshot Snapshot
	if len(raw) == 0 {
		return snapshot, ErrNotFound
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, fmt.Errorf("the proof snapshot is unreadable")
	}
	if snapshot.Schema != SchemaVersion {
		return snapshot, fmt.Errorf("unsupported proof schema")
	}
	return snapshot, nil
}

// Snapshot exposes the stored canonical content for verification and downloads.
func (s *Service) Snapshot(proofID string) (Snapshot, models.ProofDocument, error) {
	var document models.ProofDocument
	if err := s.db.Where("id = ?", proofID).First(&document).Error; err != nil {
		return Snapshot{}, document, ErrNotFound
	}
	snapshot, err := decodeSnapshot(document.Snapshot)
	return snapshot, document, err
}

func (s *Service) attestationFor(activityID, proofType string) (*models.VerifierAttestation, *models.User, error) {
	var attestation models.VerifierAttestation
	err := s.db.Where("activity_id = ?", activityID).Order("created_at desc").First(&attestation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if proofType == TypeVerified {
			return nil, nil, ErrNotPublishable
		}
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if proofType == TypeVerified && attestation.RevokedAt != nil {
		return nil, nil, ErrNotPublishable
	}
	var verifier models.User
	if err := s.db.Where("id = ?", attestation.VerifierID).First(&verifier).Error; err != nil {
		return &attestation, nil, nil
	}
	return &attestation, &verifier, nil
}

func (s *Service) manifestFor(records []models.MonitoringRecord) ([]EvidenceEntry, int, int, error) {
	entries := make([]EvidenceEntry, 0)
	unsigned, lowAccuracy := 0, 0
	for _, record := range records {
		if !record.Signed {
			unsigned++
		}
		if record.GPSAccuracyMeters > 100 {
			lowAccuracy++
		}
		objects, err := s.mrv.EvidenceFor(record.ID)
		if err != nil {
			return nil, 0, 0, err
		}
		for _, object := range objects {
			entries = append(entries, EvidenceEntry{
				Name: evidenceName(record, object), SHA256: object.SHA256,
				Access: object.AccessClass, Kind: object.Kind,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, unsigned, lowAccuracy, nil
}

func (s *Service) corroboration(activityID string) (string, string) {
	summary, ok := s.mrv.LatestCorroboration(activityID)
	if !ok {
		return "", "No satellite corroboration is available for this achievement."
	}
	if summary.AdapterMode != "live" {
		return summary.ResultHash, "Corroboration comes from a sandbox model, not live imagery."
	}
	return summary.ResultHash, ""
}

func (s *Service) registryReferences(projectID string) ([]string, string) {
	var batches []models.CreditBatch
	if err := s.db.Where("project_id = ?", projectID).Order("created_at desc").Find(&batches).Error; err != nil {
		return nil, ""
	}
	refs := make([]string, 0, len(batches))
	for _, batch := range batches {
		if batch.RegistryIssuanceID != "" {
			refs = append(refs, batch.RegistryIssuanceID)
		}
	}
	if len(refs) == 0 {
		return nil, ""
	}
	if s.registryMode != "live" {
		return refs, "Registry references are sandbox identifiers, not issued credits."
	}
	return refs, ""
}

func (s *Service) actorNames(events []models.ProvenanceEvent) (map[string]string, error) {
	names := make(map[string]string)
	dids := make([]string, 0, len(events))
	seen := make(map[string]bool)
	for _, event := range events {
		if event.IssuerDID != "" && !seen[event.IssuerDID] {
			seen[event.IssuerDID] = true
			dids = append(dids, event.IssuerDID)
		}
	}
	if len(dids) == 0 {
		return names, nil
	}
	var provisions []models.IdentityProvision
	if err := s.db.Where("did IN ?", dids).Find(&provisions).Error; err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(provisions))
	byDID := make(map[string]string)
	for _, provision := range provisions {
		userIDs = append(userIDs, provision.UserID)
		byDID[provision.DID] = provision.UserID
	}
	var users []models.User
	if len(userIDs) > 0 {
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
	}
	display := make(map[string]string)
	for _, user := range users {
		display[user.ID] = user.DisplayName
	}
	for did, userID := range byDID {
		if name := display[userID]; name != "" {
			names[did] = name
		} else {
			names[did] = did
		}
	}
	return names, nil
}

func (s *Service) nextVersion(activityID, proofType string) (int, error) {
	var current int
	err := s.db.Model(&models.ProofDocument{}).Where("activity_id = ? AND proof_type = ?", activityID, proofType).
		Select("COALESCE(MAX(version), 0)").Scan(&current).Error
	return current + 1, err
}

// selectClaim prefers the independently verified milestone when it exists, and
// otherwise documents the manager's completion claim as exactly that.
func selectClaim(activity models.ConservationActivity, events []models.ProvenanceEvent) (string, models.ProvenanceEvent, error) {
	var completion, verified *models.ProvenanceEvent
	for index := range events {
		event := &events[index]
		switch event.ClaimType {
		case models.ClaimVerified:
			verified = event
		case models.ClaimCompletion:
			completion = event
		}
	}
	switch {
	case verified != nil:
		return TypeVerified, *verified, nil
	case completion != nil:
		return TypeCompletion, *completion, nil
	case activity.Status == models.ActivityDraft || activity.Status == models.ActivityMonitoring:
		return "", models.ProvenanceEvent{}, ErrNotPublishable
	default:
		return "", models.ProvenanceEvent{}, ErrNotPublishable
	}
}

func attestationBlock(attestation *models.VerifierAttestation, verifier *models.User) *VerifierBlock {
	if attestation == nil {
		return nil
	}
	organization := "Independent verifier"
	if verifier != nil {
		organization = verifier.DisplayName
	}
	return &VerifierBlock{
		Organization: organization, Decision: attestation.Decision, Timestamp: attestation.CreatedAt,
		DID: attestation.IssuerDID, Hash: attestation.AttestationHash, Revoked: attestation.RevokedAt != nil,
	}
}

func evidenceName(record models.MonitoringRecord, object models.EvidenceObject) string {
	var manifest []mrv.EvidenceManifestItem
	_ = json.Unmarshal(record.EvidenceManifest, &manifest)
	for _, item := range manifest {
		if item.SHA256 == object.SHA256 {
			return item.Name
		}
	}
	return fmt.Sprintf("%s-%s", object.Kind, object.ID)
}

func claimSummary(proofType, projectName string, summary map[string]any) string {
	alive, _ := summary["alive_count"].(int)
	planted, _ := summary["planted_count"].(int)
	records, _ := summary["record_count"].(int)
	if proofType == TypeVerified {
		return fmt.Sprintf("Independently verified: %d of %d recorded %s plants counted alive across %d monitoring records in %s.",
			alive, planted, "conservation", records, projectName)
	}
	return fmt.Sprintf("Manager-attested completion claim (not independently verified): %d of %d plants recorded alive across %d monitoring records in %s.",
		alive, planted, records, projectName)
}

func sandboxNotice(chainMode, satelliteMode, registryMode, satelliteNote, registryNote string) string {
	notices := make([]string, 0, 3)
	if chainMode != "iota" {
		notices = append(notices, "Chain anchoring runs in a labelled sandbox; no public IOTA commitment exists for this proof.")
	}
	if satelliteNote != "" {
		notices = append(notices, satelliteNote)
	}
	if registryNote != "" {
		notices = append(notices, registryNote)
	}
	if satelliteMode != "live" && registryMode == "" {
		notices = append(notices, "Sandbox integration modes are in use.")
	}
	return joinNotices(notices)
}

func joinNotices(notes []string) string {
	out := ""
	for _, note := range notes {
		if note == "" {
			continue
		}
		if out != "" {
			out += " "
		}
		out += note
	}
	return out
}

func proofNetwork(network string) string {
	if network == "testnet" {
		return "iota_testnet"
	}
	return network
}
