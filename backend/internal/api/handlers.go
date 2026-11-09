package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/baharilink/baharimrv/internal/auth"
	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/evidence"
	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/market"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/baharilink/baharimrv/internal/proof"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (r *router) listProjects(c *gin.Context) {
	table, err := r.projects.ProjectsTable(c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, table)
}

// addPlot registers one more parcel on a project the caller manages.
func (r *router) addPlot(c *gin.Context) {
	var input mrv.PlotInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	plot, err := r.projects.AddPlot(c.Param("id"), c.GetString("user_id"), input)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id": plot.ID, "projectId": plot.ProjectID, "siteId": plot.SiteID, "code": plot.Code,
		"latitude": plot.Latitude, "longitude": plot.Longitude, "areaHectares": plot.AreaHectares,
		"assignedMonitorId": plot.AssignedMonitorID,
	})
}

// syncDelta is the offline client's change feed since its last cursor.
func (r *router) syncDelta(c *gin.Context) {
	since, err := time.Parse(time.RFC3339, c.Query("since"))
	if err != nil {
		httpx.Error(c, http.StatusUnprocessableEntity, "invalid_cursor", "A since parameter in RFC3339 format is required.")
		return
	}
	delta, err := r.projects.Delta(c.GetString("user_id"), since)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, delta)
}

// createMonitoringRecord is the online single-record form of the sync batch, so a
// connected client does not have to build a batch envelope.
func (r *router) createMonitoringRecord(c *gin.Context) {
	var payload mrv.Payload
	if !httpx.BindJSON(c, &payload) {
		return
	}
	payload.ActivityID = c.Param("id")
	response, err := r.projects.SyncBatch(c.Request.Context(), c.GetString("user_id"), mrv.Batch{
		Actions: []mrv.Action{{Type: "monitoring_record", Record: payload}},
	})
	if err != nil {
		fail(c, err)
		return
	}
	if len(response.Results) == 0 {
		fail(c, errors.New("the record was not accepted"))
		return
	}
	result := response.Results[0]
	if result.Error != "" {
		httpx.Error(c, http.StatusConflict, "sync_conflict", result.Error)
		return
	}
	c.JSON(http.StatusCreated, result)
}

// anchorStatus answers what the chain currently says about one achievement event.
func (r *router) anchorStatus(c *gin.Context) {
	status, err := r.anchors.AnchorStatus(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	if err := r.projects.AssertActivityAccess(c.GetString("user_id"), status.ActivityID); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (r *router) createProject(c *gin.Context) {
	var input mrv.ProjectInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	project, err := r.projects.CreateProject(c.GetString("user_id"), input, r.config.ChainMode == "sandbox")
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, project)
}

func (r *router) dashboard(c *gin.Context) {
	dashboard, err := r.projects.Dashboard(c.Param("id"), c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func (r *router) createActivity(c *gin.Context) {
	var input mrv.ActivityInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	input.ProjectID = c.Param("id")
	activity, err := r.projects.CreateActivity(c.GetString("user_id"), input)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, activity)
}

func (r *router) wallet(c *gin.Context) {
	wallet, err := r.market.Wallet(c.Param("id"), c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, wallet)
}

func (r *router) reconciliation(c *gin.Context) {
	reconciliation, err := r.market.Reconciliation(c.Param("id"), c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, reconciliation)
}

func (r *router) syncBatch(c *gin.Context) {
	var batch mrv.Batch
	if !httpx.BindJSON(c, &batch) {
		return
	}
	response, err := r.projects.SyncBatch(c.Request.Context(), c.GetString("user_id"), batch)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// uploadEvidence accepts one media file for an already-synchronised record. The
// manifest hash is verified by the store while the bytes stream, never trusted.
func (r *router) uploadEvidence(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "evidence_required", "A multipart file field named \"file\" is required.")
		return
	}
	upload, err := file.Open()
	if err != nil {
		fail(c, err)
		return
	}
	manifest := mrv.EvidenceManifestItem{
		Name:      file.Filename,
		Kind:      strings.TrimSpace(c.PostForm("kind")),
		MediaType: file.Header.Get("Content-Type"),
		Size:      file.Size,
		SHA256:    strings.TrimSpace(c.PostForm("sha256")),
	}
	object, err := r.projects.UploadEvidence(c.GetString("user_id"), c.Param("id"), manifest, upload)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": object.ID, "sha256": object.SHA256, "status": "stored"})
}

func (r *router) activityProvenance(c *gin.Context) {
	view, err := r.projects.ProvenanceView(c.Param("id"), c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

type activityDetailView struct {
	ID                 string                `json:"id"`
	ProjectID          string                `json:"projectId"`
	PlotID             string                `json:"plotId"`
	ActivityType       string                `json:"activityType"`
	Status             string                `json:"status"`
	MethodologyVersion string                `json:"methodologyVersion,omitempty"`
	PeriodStart        *time.Time            `json:"periodStart,omitempty"`
	PeriodEnd          *time.Time            `json:"periodEnd,omitempty"`
	Record             *monitoringRecordView `json:"record,omitempty"`
	Provenance         *provView             `json:"provenance,omitempty"`
}

type monitoringRecordView struct {
	AliveCount           int            `json:"aliveCount"`
	PlantedCount         int            `json:"plantedCount"`
	AverageHeightMetres  string         `json:"averageHeightMetres"`
	Species              string         `json:"species"`
	Observations         string         `json:"observations,omitempty"`
	AttestationStatement string         `json:"attestationStatement,omitempty"`
	Evidence             []evidenceView `json:"evidence"`
}

type evidenceView struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"mediaType"`
}

type provView struct {
	Status      string `json:"status"`
	Network     string `json:"network,omitempty"`
	ExplorerURL string `json:"explorerUrl,omitempty"`
	ObjectID    string `json:"objectId,omitempty"`
}

// firstSpecies decodes a record's species JSON, which may be stored as a JSON
// array of species names, a bare JSON string, or a non-JSON string.
func firstSpecies(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0]
	}
	// Stored as a bare string (old records and seed records).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// activityDetail bundles the monitoring record fields, evidence and the latest
// provenance event for a single activity in one round trip, so a mobile client
// does not have to chase three endpoints to render its detail screen.
func (r *router) activityDetail(c *gin.Context) {
	userID := c.GetString("user_id")
	activityID := c.Param("id")

	if err := r.projects.AssertActivityAccess(userID, activityID); err != nil {
		fail(c, err)
		return
	}

	var activity models.ConservationActivity
	if err := r.db.Where("id = ?", activityID).First(&activity).Error; err != nil {
		fail(c, mrv.ErrNotFound)
		return
	}

	view := activityDetailView{
		ID: activity.ID, ProjectID: activity.ProjectID, PlotID: activity.PlotID,
		ActivityType: activity.ActivityType, Status: activity.Status,
		MethodologyVersion: activity.MethodologyVersion,
		PeriodStart:        &activity.ReportingPeriodStart, PeriodEnd: &activity.ReportingPeriodEnd,
	}

	// Load monitoring records. If the lookup fails the record stays nil.
	if records, err := r.projects.RecordsFor(activityID); err == nil && len(records) > 0 {
		rec := records[0]
		rv := &monitoringRecordView{
			AliveCount:           rec.AliveCount,
			PlantedCount:         rec.PlantedCount,
			AverageHeightMetres:  rec.AverageHeightMetres,
			Species:              firstSpecies(rec.Species),
			AttestationStatement: rec.Attestation,
		}
		var evidenceObjects []models.EvidenceObject
		r.db.Where("monitoring_record_id = ?", rec.ID).Find(&evidenceObjects)
		for _, e := range evidenceObjects {
			rv.Evidence = append(rv.Evidence, evidenceView{
				ID: e.ID, Kind: e.Kind, SHA256: e.SHA256, MediaType: e.MediaType,
			})
		}
		view.Record = rv
	}

	// Load latest provenance event. If the lookup fails provenance stays nil.
	if events, err := r.anchors.Events(activityID); err == nil && len(events) > 0 {
		latest := events[len(events)-1]
		pv := &provView{
			Status:  latest.Status,
			Network: r.anchors.PublicNetwork(),
		}
		if latest.Status == provenance.StatusAnchored {
			pv.ExplorerURL = r.anchors.ExplorerObject(latest)
			pv.ObjectID = latest.ObjectID
		}
		view.Provenance = pv
	}

	c.JSON(http.StatusOK, view)
}

func (r *router) completeActivity(c *gin.Context) {
	activity, event, err := r.projects.Complete(c.Request.Context(), c.Param("id"), c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"activity": activity, "event": event})
}

func (r *router) retryAnchor(c *gin.Context) {
	event, err := r.projects.RetryAnchor(c.Request.Context(), c.Param("id"), c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": event.Status, "event": event})
}

func (r *router) disputeActivity(c *gin.Context) {
	var request struct {
		Reason string `json:"reason"`
	}
	if !httpx.BindJSON(c, &request) {
		return
	}
	if strings.TrimSpace(request.Reason) == "" {
		httpx.Error(c, http.StatusUnprocessableEntity, "reason_required", "A dispute must state what is contested.")
		return
	}
	dispute, err := r.verification.Dispute(c.Param("id"), c.GetString("user_id"), request.Reason)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, dispute)
}

func (r *router) assignVerifier(c *gin.Context) {
	var request struct {
		VerifierID string `json:"verifierId"`
	}
	if !httpx.BindJSON(c, &request) {
		return
	}
	requested, err := r.verification.Assign(c.Request.Context(), c.Param("id"), c.GetString("user_id"), request.VerifierID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, requested)
}

func (r *router) projectDocuments(c *gin.Context) {
	if err := r.projects.AssertProjectAccess(c.GetString("user_id"), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	documents, err := r.projects.Documents(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"documents": documents})
}

func (r *router) downloadDocument(c *gin.Context) {
	var document models.ProjectDocument
	if err := r.db.Where("id = ?", c.Param("id")).First(&document).Error; err != nil {
		fail(c, mrv.ErrNotFound)
		return
	}
	if err := r.projects.AssertProjectAccess(c.GetString("user_id"), document.ProjectID); err != nil {
		fail(c, err)
		return
	}
	body, size, err := r.evidence.Open(document.StoredURI)
	if err != nil {
		fail(c, err)
		return
	}
	defer body.Close()
	c.Header("Content-Disposition", contentDisposition(document.Name))
	c.DataFromReader(http.StatusOK, size, "application/octet-stream", body, map[string]string{
		"X-Content-Sha256": document.SHA256,
	})
}

func (r *router) listings(c *gin.Context) {
	listings, err := r.market.Listings()
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"listings": listings})
}

func (r *router) purchase(c *gin.Context) {
	var input market.PurchaseInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	result, err := r.market.Purchase(c.Request.Context(), c.GetString("user_id"), input)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (r *router) retire(c *gin.Context) {
	var input market.RetirementInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	result, err := r.market.Retire(c.Request.Context(), c.GetString("user_id"), c.Param("id"), input)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (r *router) generateProof(c *gin.Context) {
	var request struct {
		Format         string `json:"format"`
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if !httpx.BindJSON(c, &request) {
		return
	}
	if request.Format != "" && !isProofFormat(request.Format) {
		httpx.Error(c, http.StatusUnprocessableEntity, "invalid_format", "Proofs render as pdf, json, or zip.")
		return
	}
	if request.IdempotencyKey == "" {
		httpx.Error(c, http.StatusUnprocessableEntity, "idempotency_key_required", "A proof request needs an idempotency key.")
		return
	}
	view, err := r.proof.Generate(c.Request.Context(), c.Param("id"), c.GetString("user_id"), request.IdempotencyKey)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (r *router) listProofs(c *gin.Context) {
	views, err := r.proof.ForAchievement(c.Param("id"), c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"proofs": views})
}

func (r *router) downloadProof(c *gin.Context) {
	format := c.Query("format")
	if !isProofFormat(format) {
		httpx.Error(c, http.StatusUnprocessableEntity, "invalid_format", "Proofs render as pdf, json, or zip.")
		return
	}
	name, mediaType, body, err := r.proof.DownloadFor(c.Param("id"), format, c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Disposition", contentDisposition(name))
	c.Data(http.StatusOK, mediaType, body)
}

func (r *router) publicProof(c *gin.Context) {
	view, verification, err := r.proof.GetPublic(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"proof": view, "verification": verification, "url": r.proof.PublicURL(c.Param("id")),
	})
}

func (r *router) verifyProof(c *gin.Context) {
	result, err := r.proof.Verify(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (r *router) identityProfile(c *gin.Context) {
	profile, err := r.profile(c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (r *router) retryIdentity(c *gin.Context) {
	userID := c.GetString("user_id")
	provision, err := r.identities.EnsureProvision(c.Request.Context(), userID)
	if err != nil && provision.State == "" {
		fail(c, err)
		return
	}
	profile, profileErr := r.profile(userID)
	if profileErr != nil {
		fail(c, profileErr)
		return
	}
	if provision.State != models.IdentityActive {
		c.JSON(http.StatusAccepted, profile)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (r *router) linkedPlatforms(c *gin.Context) {
	links, err := r.links.Links(c.GetString("user_id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"links": platforms(links)})
}

// fail maps domain errors onto the HTTP envelope. Anything unclassified loses its
// message so an internal failure can never leak through a response body.
func (r *router) projectSatellitePasses(c *gin.Context) {
	mission := c.Query("mission")
	from, _ := time.Parse(time.RFC3339, c.Query("from"))
	if from.IsZero() {
		from = time.Now().UTC().Add(-24 * time.Hour)
	}
	to, _ := time.Parse(time.RFC3339, c.Query("to"))
	if to.IsZero() {
		to = time.Now().UTC().Add(14 * 24 * time.Hour)
	}
	if err := r.projects.AssertProjectAccess(c.GetString("user_id"), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	passes, err := r.satellite.PredictPasses(c.Param("id"), mission, from, to)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": passes})
}

func (r *router) projectSatelliteLatest(c *gin.Context) {
	mission := c.Query("mission")
	if err := r.projects.AssertProjectAccess(c.GetString("user_id"), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	pass, ok, err := r.satellite.LatestPass(c.Param("id"), mission)
	if err != nil {
		fail(c, err)
		return
	}
	if !ok {
		c.JSON(http.StatusOK, gin.H{"pass": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pass": pass})
}

func (r *router) projectWeather(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"source": "sandbox", "temperatureC": 27.4, "humidityPct": 72,
		"windKph": 12.5, "note": "Sandbox weather; not a live forecast.",
	})
}

func (r *router) projectSalinity(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"source": "sandbox", "salinityPpt": 34.2,
		"note": "Sandbox salinity; not a live measurement.",
	})
}

func (r *router) listActivities(c *gin.Context) {
	if err := r.projects.AssertProjectAccess(c.GetString("user_id"), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	items, err := r.projects.Activities(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) listMonitors(c *gin.Context) {
	items, err := r.projects.Monitors()
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) updatePlot(c *gin.Context) {
	var input struct {
		AssignedMonitorID string `json:"assignedMonitorId"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}
	plot, err := r.projects.UpdatePlot(c.Param("id"), c.Param("plotId"), c.GetString("user_id"), input.AssignedMonitorID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": plot.ID, "projectId": plot.ProjectID, "siteId": plot.SiteID, "code": plot.Code,
		"latitude": plot.Latitude, "longitude": plot.Longitude, "areaHectares": plot.AreaHectares,
		"assignedMonitorId": plot.AssignedMonitorID,
	})
}

func fail(c *gin.Context, err error) {
	if err == nil {
		return
	}
	// A domain error was written for the client, so its own wording and code ship.
	var rejection *domain.Error
	if errors.As(err, &rejection) {
		httpx.Error(c, rejection.Kind.Status, rejection.Code, rejection.Message)
		return
	}
	status, code := classify(err)
	if status == 0 {
		// Nothing mapped this to a client-facing status, so it is a server fault: log it before masking it.
		slog.Error("unmapped request error", "error", err, "method", c.Request.Method, "path", c.Request.URL.Path)
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		return
	}
	httpx.Error(c, status, code, publicMessage(err))
}

func (r *router) methodologies(c *gin.Context) {
	items, err := r.projects.Methodologies()
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func classify(err error) (int, string) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, mrv.ErrNotFound),
		errors.Is(err, market.ErrNotFound), errors.Is(err, proof.ErrNotFound),
		errors.Is(err, provenance.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, auth.ErrStepUpRequired):
		return http.StatusForbidden, "step_up_required"
	case errors.Is(err, identity.ErrProvisioning):
		return http.StatusForbidden, "identity_not_active"
	case errors.Is(err, auth.ErrInvalidCredentials):
		return http.StatusUnauthorized, "invalid_credentials"
	case errors.Is(err, mrv.ErrForbidden), errors.Is(err, market.ErrForbidden), errors.Is(err, proof.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, mrv.ErrConflict):
		return http.StatusConflict, "sync_conflict"
	case errors.Is(err, proof.ErrNotPublishable):
		return http.StatusConflict, "nothing_to_prove"
	case errors.Is(err, evidence.ErrTooLarge):
		return http.StatusRequestEntityTooLarge, "evidence_too_large"
	case errors.Is(err, evidence.ErrHashMismatch), errors.Is(err, evidence.ErrInvalidDigest),
		errors.Is(err, mrv.ErrInvalidRecord), errors.Is(err, market.ErrValidation):
		return http.StatusUnprocessableEntity, "invalid_resource"
	case errors.Is(err, evidence.ErrUnknownFile):
		return http.StatusNotFound, "evidence_unavailable"
	case errors.Is(err, market.ErrUnavailable), errors.Is(err, provenance.ErrNotRetryable):
		return http.StatusServiceUnavailable, "adapter_unavailable"
	default:
		return 0, ""
	}
}

// publicMessage keeps a domain error's own wording when the service wrote it for a
// client; anything else is replaced by a stable sentence.
func publicMessage(err error) string {
	message := strings.TrimSpace(err.Error())
	if message == "" || len(message) > 400 || strings.Contains(message, "\n") {
		return "The request could not be completed."
	}
	for _, internal := range []string{"gorm", "sql:", "driver:", "pq:", "context canceled", "runtime error"} {
		if strings.Contains(strings.ToLower(message), internal) {
			return "The request could not be completed."
		}
	}
	return message
}

func contentDisposition(name string) string {
	cleaned := strings.Map(func(symbol rune) rune {
		if unicode.IsLetter(symbol) || unicode.IsDigit(symbol) || symbol == '.' || symbol == '-' || symbol == '_' {
			return symbol
		}
		return '_'
	}, name)
	if cleaned == "" {
		cleaned = "download"
	}
	return "attachment; filename=\"" + cleaned + "\""
}

func isProofFormat(format string) bool {
	switch format {
	case "pdf", "json", "zip":
		return true
	default:
		return false
	}
}
