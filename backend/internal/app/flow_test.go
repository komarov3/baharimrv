package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/app"
	"github.com/baharilink/baharimrv/internal/config"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const testPassword = "Correct-Horse-1234"

// fixture drives the assembled API exactly as the production binary wires it, so
// these tests cannot pass against a graph real clients never use.
type fixture struct {
	t             *testing.T
	app           *app.App
	methodologyID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i + 1)
	}
	built, err := app.Build(config.Config{
		Environment:         "development",
		Port:                "0",
		DatabaseURL:         fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", filepath.Join(dir, "test.db")),
		CORSOrigins:         []string{"http://localhost:5173"},
		PublicBaseURL:       "https://mrv.example.test",
		SessionTTL:          time.Hour,
		RefreshTTL:          7 * 24 * time.Hour,
		KMSKEK:              kek,
		ChainMode:           "sandbox",
		IOTANetwork:         "testnet",
		SatelliteMode:       "sandbox",
		RegistryMode:        "sandbox",
		PaymentMode:         "sandbox",
		EvidenceRoot:        filepath.Join(dir, "evidence"),
		OIDCIssuer:          "https://mrv.example.test",
		OIDCSigningKeyID:    "baharimrv-test",
		AuthRatePerMinute:   10000,
		PublicRatePerMinute: 10000,
		MaxRequestBytes:     4 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("build the application: %v", err)
	}
	f := &fixture{t: t, app: built}
	estimation, _ := json.Marshal(map[string]any{"tonnes_co2_per_tree": "0.006250", "period_years": 1})
	methodology := models.Methodology{
		ID: uuid.NewString(), Code: "TEST-MANGROVE", Name: "Test methodology", Version: "1.0",
		RequiredEvidence: "Geo-tagged survival count and one photo.",
		Estimation:       datatypes.JSON(estimation), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := built.DB.Create(&methodology).Error; err != nil {
		t.Fatalf("create the methodology: %v", err)
	}
	f.methodologyID = methodology.ID
	return f
}

type response struct {
	status int
	body   map[string]any
	raw    []byte
}

func (r response) str(path ...string) string {
	var current any = r.body
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	text, _ := current.(string)
	return text
}

func (r response) list(key string) []any {
	items, _ := r.body[key].([]any)
	return items
}

func (f *fixture) call(method, path, token string, body any) response {
	f.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			f.t.Fatalf("encode request: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	f.app.Handler.ServeHTTP(recorder, req)
	result := response{status: recorder.Code, raw: recorder.Body.Bytes()}
	if len(result.raw) > 0 {
		_ = json.Unmarshal(result.raw, &result.body)
	}
	return result
}

func (f *fixture) must(method, path, token string, body any, want int) response {
	f.t.Helper()
	result := f.call(method, path, token, body)
	if result.status != want {
		f.t.Fatalf("%s %s: expected %d, got %d: %s", method, path, want, result.status, result.raw)
	}
	return result
}

type account struct {
	id    string
	email string
	token string
}

func (f *fixture) register(role, local string) account {
	f.t.Helper()
	email := local + "@test.baharimrv.local"
	created := f.must(http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"email": email, "password": testPassword, "displayName": local, "role": role,
	}, http.StatusCreated)
	login := f.must(http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": testPassword,
	}, http.StatusOK)
	return account{id: created.str("user", "id"), email: email, token: login.str("sessionToken")}
}

// registerAdmin is the narrow path past the public register handler, which
// refuses the admin role. Tests that need an operator bypass that check by
// writing the user directly and logging in through the normal path.
func (f *fixture) registerAdmin(local string) account {
	f.t.Helper()
	email := local + "@test.baharimrv.local"
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), 12)
	if err != nil {
		f.t.Fatalf("hash admin password: %v", err)
	}
	now := time.Now().UTC()
	user := models.User{
		ID: uuid.NewString(), Email: email, PasswordHash: string(hash),
		DisplayName: local, Role: models.RoleAdmin, Status: models.IdentityActive,
		CreatedAt: now, UpdatedAt: now,
	}
	provision := models.IdentityProvision{
		ID: uuid.NewString(), UserID: user.ID, State: models.IdentityActive,
		Network: "iota:testnet", CreatedAt: now, UpdatedAt: now,
	}
	if err := f.app.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return tx.Create(&provision).Error
	}); err != nil {
		f.t.Fatalf("seed admin: %v", err)
	}
	login := f.must(http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": testPassword,
	}, http.StatusOK)
	return account{id: user.ID, email: email, token: login.str("sessionToken")}
}

func (f *fixture) stepUp(token, action string) string {
	f.t.Helper()
	result := f.must(http.MethodPost, "/api/v1/auth/step-up", token, map[string]any{
		"password": testPassword, "action": action,
	}, http.StatusCreated)
	return result.str("stepUpCode")
}

type work struct {
	manager   account
	monitor   account
	verifier  account
	projectID string
	plotID    string
	activity  string
}

func (f *fixture) newWork() work {
	f.t.Helper()
	manager := f.register(models.RoleManager, "manager")
	monitor := f.register(models.RoleMonitor, "monitor")
	verifier := f.register(models.RoleVerifier, "verifier")
	project := f.must(http.MethodPost, "/api/v1/projects", manager.token, map[string]any{
		"name": "Test restoration", "methodologyId": f.methodologyID,
		"communityGroup": "Test community group", "species": "Rhizophora mucronata",
		"communityShareBasisPoints": 6000, "sites": []string{"Mikoko Park"},
		"plots": []map[string]any{{
			"code": "TEST-P1", "siteName": "Mikoko Park", "latitude": -4.0532,
			"longitude": 39.6741, "areaHectares": "2.500000",
		}},
	}, http.StatusCreated)
	projectID := project.str("id")
	var plot models.Plot
	if err := f.app.DB.Where("project_id = ?", projectID).First(&plot).Error; err != nil {
		f.t.Fatalf("read the created plot: %v", err)
	}
	if err := f.app.DB.Model(&models.Plot{}).Where("id = ?", plot.ID).
		Update("assigned_monitor_id", monitor.id).Error; err != nil {
		f.t.Fatalf("assign the monitor: %v", err)
	}
	now := time.Now().UTC()
	activity := f.must(http.MethodPost, "/api/v1/projects/"+projectID+"/activities", manager.token, map[string]any{
		"plotId": plot.ID, "activityType": "restoration", "methodologyVersion": "1.0",
		"reportingPeriodStart": now.AddDate(0, -6, 0), "reportingPeriodEnd": now,
	}, http.StatusCreated)
	return work{manager: manager, monitor: monitor, verifier: verifier,
		projectID: projectID, plotID: plot.ID, activity: activity.str("id")}
}

func (w work) record(clientID string, alive int) map[string]any {
	return map[string]any{
		"clientId": clientID, "activityId": w.activity, "plotId": w.plotID,
		"methodologyVersion": "1.0", "schemaVersion": "1.0",
		"deviceTimestamp": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano),
		"location":        map[string]any{"latitude": -4.0532, "longitude": 39.6741, "accuracyMetres": 4.5},
		"aliveCount":      alive, "plantedCount": 100, "averageHeightMetres": "1.420000",
		"species":      "Rhizophora mucronata",
		"observations": "Test observation",
		"attestation":  map[string]any{"statement": "I attest this record reflects my field observation."},
	}
}

func (f *fixture) sync(w work, payload map[string]any) response {
	f.t.Helper()
	return f.must(http.MethodPost, "/api/v1/sync/batch", w.monitor.token, map[string]any{
		"actions": []map[string]any{{"type": "monitoring_record", "record": payload}},
	}, http.StatusOK)
}

func firstResult(t *testing.T, result response) map[string]any {
	t.Helper()
	results := result.list("results")
	if len(results) != 1 {
		t.Fatalf("expected one sync result, got %s", result.raw)
	}
	item, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected sync result shape: %s", result.raw)
	}
	return item
}

func (f *fixture) uploadEvidence(w work, recordID string, content []byte) response {
	f.t.Helper()
	digest := sha256.Sum256(content)
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile("file", "plot-photo.jpg")
	if err != nil {
		f.t.Fatalf("create the multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		f.t.Fatalf("write evidence bytes: %v", err)
	}
	_ = writer.WriteField("kind", "photo")
	_ = writer.WriteField("sha256", hex.EncodeToString(digest[:]))
	if err := writer.Close(); err != nil {
		f.t.Fatalf("close the multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitoring-records/"+recordID+"/evidence", &buffer)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+w.manager.token)
	recorder := httptest.NewRecorder()
	f.app.Handler.ServeHTTP(recorder, req)
	result := response{status: recorder.Code, raw: recorder.Body.Bytes()}
	_ = json.Unmarshal(result.raw, &result.body)
	return result
}

// completed builds the shortest honest path to a manager completion claim.
func (f *fixture) completed() work {
	f.t.Helper()
	w := f.newWork()
	photo := []byte("field evidence bytes for the test plot")
	digest := sha256.Sum256(photo)
	payload := w.record(uuid.NewString(), 92)
	payload["evidenceManifest"] = []map[string]any{{
		"name": "plot-photo.jpg", "kind": "photo", "mediaType": "image/jpeg",
		"size": len(photo), "sha256": hex.EncodeToString(digest[:]),
	}}
	result := firstResult(f.t, f.sync(w, payload))
	if result["status"] != models.SyncSubmitted {
		f.t.Fatalf("the record was not accepted: %v", result)
	}
	recordID, _ := result["monitoringRecordId"].(string)
	if upload := f.uploadEvidence(w, recordID, photo); upload.status != http.StatusCreated {
		f.t.Fatalf("evidence upload failed: %d %s", upload.status, upload.raw)
	}
	f.must(http.MethodPost, "/api/v1/activities/"+w.activity+"/complete", w.manager.token, map[string]any{}, http.StatusOK)
	return w
}

func TestHealthNamesTheAdaptersActuallyRunning(t *testing.T) {
	f := newFixture(t)
	health := f.must(http.MethodGet, "/api/v1/health", "", nil, http.StatusOK)
	for _, key := range []string{"chain", "satellite", "registry", "payment"} {
		if got := health.str("modes", key); got != "sandbox" {
			t.Fatalf("mode %s = %q, want the sandbox the test configured", key, got)
		}
	}
	if health.str("serverTime") == "" {
		t.Fatal("health did not report serverTime in the camelCase contract")
	}
}

func TestSessionIsRequiredForConservationRecords(t *testing.T) {
	f := newFixture(t)
	f.must(http.MethodGet, "/api/v1/projects", "", nil, http.StatusUnauthorized)
}

func TestRegistrationProvisionsIdentityWithoutExposingKeyMaterial(t *testing.T) {
	f := newFixture(t)
	manager := f.register(models.RoleManager, "custody")
	profile := f.must(http.MethodGet, "/api/v1/users/me/identity", manager.token, nil, http.StatusOK)
	if got := profile.str("status"); got != models.IdentityActive {
		t.Fatalf("identity status = %q, want active after sandbox provisioning", got)
	}
	if !strings.HasPrefix(profile.str("did"), "did:iota:sandbox:") {
		t.Fatalf("did = %q, want the sandbox network the adapter reports", profile.str("did"))
	}
	if !strings.HasPrefix(profile.str("keyCustodyReference"), "envelope_kms:") {
		t.Fatalf("custody reference = %q, want an opaque envelope reference", profile.str("keyCustodyReference"))
	}
	body := string(profile.raw)
	for _, secret := range []string{"mnemonic", "privateKey", "secretKey", "seedPhrase", "walletPassword"} {
		if strings.Contains(body, secret) {
			t.Fatalf("the identity profile leaked %s", secret)
		}
	}
}

func TestRegistrationRefusesRolesThePublicMayNotClaim(t *testing.T) {
	f := newFixture(t)
	f.must(http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"email": "admin@test.baharimrv.local", "password": testPassword, "displayName": "admin", "role": models.RoleAdmin,
	}, http.StatusBadRequest)
}

func TestCompletionIsRefusedUntilItsEvidenceExists(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	rejected := f.call(http.MethodPost, "/api/v1/activities/"+w.activity+"/complete", w.manager.token, map[string]any{})
	if rejected.status != http.StatusConflict {
		t.Fatalf("completion without records returned %d, want 409: %s", rejected.status, rejected.raw)
	}

	photo := []byte("declared but not yet uploaded")
	digest := sha256.Sum256(photo)
	payload := w.record(uuid.NewString(), 90)
	payload["evidenceManifest"] = []map[string]any{{
		"name": "plot-photo.jpg", "kind": "photo", "mediaType": "image/jpeg",
		"size": len(photo), "sha256": hex.EncodeToString(digest[:]),
	}}
	result := firstResult(t, f.sync(w, payload))
	recordID, _ := result["monitoringRecordId"].(string)
	missing := f.call(http.MethodPost, "/api/v1/activities/"+w.activity+"/complete", w.manager.token, map[string]any{})
	if missing.status != http.StatusConflict {
		t.Fatalf("completion with missing media returned %d, want 409: %s", missing.status, missing.raw)
	}

	if wrong := f.uploadEvidence(w, recordID, []byte("bytes that do not match the manifest")); wrong.status == http.StatusCreated {
		t.Fatal("evidence whose bytes contradict the declared hash was accepted")
	}
	f.mustUpload(w, recordID, photo)

	completed := f.must(http.MethodPost, "/api/v1/activities/"+w.activity+"/complete", w.manager.token, map[string]any{}, http.StatusOK)
	if got := completed.str("activity", "status"); got != models.ActivityAwaitingVerification {
		t.Fatalf("activity status = %q, want awaiting_verification, never verified", got)
	}
	if got := completed.str("event", "claimType"); got != models.ClaimCompletion {
		t.Fatalf("claim type = %q, want the manager completion milestone", got)
	}
	if completed.str("event", "commitment") == "" {
		t.Fatal("the completion event carries no content commitment")
	}
}

func (f *fixture) mustUpload(w work, recordID string, content []byte) {
	f.t.Helper()
	if result := f.uploadEvidence(w, recordID, content); result.status != http.StatusCreated {
		f.t.Fatalf("evidence upload returned %d: %s", result.status, result.raw)
	}
}

func TestSyncReplaysAndConflictsWithoutOverwritingEvidence(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	clientID := uuid.NewString()
	payload := w.record(clientID, 88)

	first := firstResult(t, f.sync(w, payload))
	if first["status"] != models.SyncSubmitted {
		t.Fatalf("first sync status = %v", first["status"])
	}
	if first["signed"] != false {
		t.Fatal("an unsigned capture was reported as signed")
	}
	replay := firstResult(t, f.sync(w, payload))
	if replay["monitoringRecordId"] != first["monitoringRecordId"] {
		t.Fatalf("a retried sync created a different record: %v vs %v", replay, first)
	}

	changed := w.record(clientID, 12)
	conflict := firstResult(t, f.sync(w, changed))
	if conflict["status"] != models.SyncConflict {
		t.Fatalf("a changed record with the same clientId returned %v, want sync_conflict", conflict["status"])
	}
	if conflict["conflict"] == nil {
		t.Fatal("the conflict did not describe the stored server version")
	}

	var count int64
	if err := f.app.DB.Model(&models.MonitoringRecord{}).
		Where("monitor_id = ? AND client_id = ?", w.monitor.id, clientID).Count(&count).Error; err != nil {
		t.Fatalf("count stored records: %v", err)
	}
	if count != 1 {
		t.Fatalf("%d records stored for one clientId; sync must never duplicate or overwrite", count)
	}
	var stored models.MonitoringRecord
	if err := f.app.DB.Where("monitor_id = ? AND client_id = ?", w.monitor.id, clientID).First(&stored).Error; err != nil {
		t.Fatalf("read the stored record: %v", err)
	}
	if stored.AliveCount != 88 {
		t.Fatalf("the stored observation was overwritten to %d by a conflicting upload", stored.AliveCount)
	}
}

func TestForgedMonitorSignatureIsRejected(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	payload := w.record(uuid.NewString(), 88)
	payload["attestation"] = map[string]any{
		"statement":   "I attest this record reflects my field observation.",
		"identityDid": "did:iota:sandbox:somebody-else",
		"signature":   base64.StdEncoding.EncodeToString([]byte("not a real signature")),
	}
	result := firstResult(t, f.sync(w, payload))
	if result["status"] != models.SyncFailed {
		t.Fatalf("a signature against another identity was accepted: %v", result)
	}
}

func TestIndependentVerificationCreatesASecondMilestone(t *testing.T) {
	f := newFixture(t)
	w := f.completed()

	proven := f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/provenance", w.manager.token, nil, http.StatusOK)
	before := countClaims(proven)

	f.must(http.MethodPost, "/api/v1/activities/"+w.activity+"/submit-verification", w.manager.token, map[string]any{
		"verifierId": w.verifier.id,
	}, http.StatusCreated)
	var openRequests int64
	if err := f.app.DB.Model(&models.VerificationRequest{}).
		Where("activity_id = ?", w.activity).Count(&openRequests).Error; err != nil {
		t.Fatalf("count verification requests: %v", err)
	}
	if openRequests != 1 {
		t.Fatalf("one submission created %d verification requests", openRequests)
	}
	resubmitted := f.must(http.MethodPost, "/api/v1/activities/"+w.activity+"/submit-verification", w.manager.token, map[string]any{
		"verifierId": w.verifier.id,
	}, http.StatusCreated)
	if err := f.app.DB.Model(&models.VerificationRequest{}).
		Where("activity_id = ?", w.activity).Count(&openRequests).Error; err != nil {
		t.Fatalf("recount verification requests: %v", err)
	}
	if openRequests != 1 {
		t.Fatalf("resubmitting the same assignment opened a duplicate request: %s", resubmitted.raw)
	}

	queue := f.must(http.MethodGet, "/api/v1/verification-requests", w.verifier.token, nil, http.StatusOK)
	requests := queue.list("items")
	if len(requests) == 0 {
		t.Fatalf("the assigned verifier saw no queue rows: %s", queue.raw)
	}
	requestID, _ := requests[0].(map[string]any)["id"].(string)

	decision := func(token, code, key string) response {
		return f.call(http.MethodPost, "/api/v1/verification-requests/"+requestID+"/decision", token, map[string]any{
			"decision": "approve", "notes": "Counts, media and coordinates agree with the methodology.",
			"methodologyVersion": "1.0", "stepUpCode": code, "idempotencyKey": key,
		})
	}
	if got := decision(w.manager.token, "", uuid.NewString()); got.status != http.StatusForbidden {
		t.Fatalf("the project manager decided their own verification: %d %s", got.status, got.raw)
	}
	if got := decision(w.verifier.token, "", uuid.NewString()); got.status == http.StatusOK {
		t.Fatal("a decision was recorded without recent step-up confirmation")
	}

	key := uuid.NewString()
	approved := decision(w.verifier.token, f.stepUp(w.verifier.token, "verification_decision"), key)
	if approved.status != http.StatusOK {
		t.Fatalf("the independent decision failed: %d %s", approved.status, approved.raw)
	}
	if approved.str("attestationReference") == "" {
		t.Fatal("the decision returned no attestation reference")
	}
	if replay := decision(w.verifier.token, "", key); replay.status != http.StatusOK {
		t.Fatalf("replaying the same idempotency key returned %d: %s", replay.status, replay.raw)
	}

	after := f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/provenance", w.manager.token, nil, http.StatusOK)
	claims := countClaims(after)
	if claims[models.ClaimCompletion] != before[models.ClaimCompletion] {
		t.Fatal("verification rewrote the manager's completion milestone")
	}
	if claims[models.ClaimVerified] != 1 {
		t.Fatalf("verified milestone count = %d, want exactly one separate event", claims[models.ClaimVerified])
	}

	dashboard := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/dashboard", w.manager.token, nil, http.StatusOK)
	if !strings.Contains(string(dashboard.raw), "verified") {
		t.Fatal("the dashboard does not reflect the verified milestone")
	}
}

func countClaims(result response) map[string]int {
	counts := map[string]int{}
	for _, item := range result.list("events") {
		event, ok := item.(map[string]any)
		if !ok {
			continue
		}
		claim, _ := event["type"].(string)
		counts[claim]++
	}
	return counts
}

func TestProofStatesStayHonestAcrossBothMilestones(t *testing.T) {
	f := newFixture(t)
	w := f.completed()

	unverified := f.must(http.MethodPost, "/api/v1/achievements/"+w.activity+"/proofs", w.manager.token,
		map[string]any{"idempotencyKey": uuid.NewString()}, http.StatusCreated)
	if got := unverified.str("proofType"); got != "completion_claim" {
		t.Fatalf("proof type = %q, want the manager completion claim", got)
	}
	if got := unverified.str("chain", "network"); got != "sandbox" {
		t.Fatalf("chain network = %q, want sandbox", got)
	}
	if !strings.Contains(unverified.str("sandboxNotice"), "sandbox") {
		t.Fatal("a sandbox proof carried no sandbox notice")
	}
	if unverified.str("verifierAttestation", "decision") != "" {
		t.Fatal("a completion claim presented a verifier decision that does not exist")
	}

	proofID := unverified.str("id")
	public := f.must(http.MethodGet, "/api/v1/public/proofs/"+proofID, "", nil, http.StatusOK)
	verification := f.must(http.MethodPost, "/api/v1/public/proofs/"+proofID+"/verify", "", map[string]any{}, http.StatusOK)
	if got := verification.str("overallStatus"); got == "valid" {
		t.Fatal("a sandbox-anchored proof reported overallStatus valid")
	}
	if public.str("url") == "" {
		t.Fatal("the public envelope has no permanent URL")
	}

	f.must(http.MethodPost, "/api/v1/activities/"+w.activity+"/submit-verification", w.manager.token,
		map[string]any{"verifierId": w.verifier.id}, http.StatusCreated)
	queue := f.must(http.MethodGet, "/api/v1/verification-requests", w.verifier.token, nil, http.StatusOK)
	items := queue.list("items")
	if len(items) == 0 {
		t.Fatalf("the verifier queue is empty after assignment: %s", queue.raw)
	}
	requestID, _ := items[0].(map[string]any)["id"].(string)
	f.must(http.MethodPost, "/api/v1/verification-requests/"+requestID+"/decision", w.verifier.token, map[string]any{
		"decision": "approve", "notes": "Evidence reviewed against the methodology.",
		"methodologyVersion": "1.0", "stepUpCode": f.stepUp(w.verifier.token, "verification_decision"),
		"idempotencyKey": uuid.NewString(),
	}, http.StatusOK)

	verified := f.must(http.MethodPost, "/api/v1/achievements/"+w.activity+"/proofs", w.manager.token,
		map[string]any{"idempotencyKey": uuid.NewString()}, http.StatusCreated)
	if got := verified.str("proofType"); got != "verified_achievement" {
		t.Fatalf("proof type after approval = %q, want verified_achievement", got)
	}
	if verified.str("verifierAttestation", "decision") != "approve" {
		t.Fatal("the verified proof does not carry the independent decision")
	}
	if verified.str("id") == proofID {
		t.Fatal("the verified milestone reused the completion claim's proof document")
	}
}

func TestStepUpTicketsAreActionBoundAndSingleUse(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	buyer := f.register(models.RoleBuyer, "buyer")

	purchase := func(code string) response {
		return f.call(http.MethodPost, "/api/v1/marketplace/purchases", buyer.token, map[string]any{
			"listingId": uuid.NewString(), "quantityTonnes": "1.000000",
			"stepUpCode": code, "idempotencyKey": uuid.NewString(),
		})
	}
	if got := purchase(""); got.status == http.StatusOK || got.status == http.StatusCreated {
		t.Fatal("a purchase succeeded without step-up confirmation")
	}
	if got := purchase(f.stepUp(buyer.token, "credit_retirement")); got.status == http.StatusOK {
		t.Fatal("a retirement ticket authorised a purchase")
	}

	code := f.stepUp(buyer.token, "marketplace_purchase")
	first := purchase(code)
	if first.status == http.StatusOK || first.status == http.StatusCreated {
		t.Fatalf("a purchase against a non-existent listing succeeded: %s", first.raw)
	}
	if second := purchase(code); second.status != http.StatusForbidden {
		t.Fatalf("a consumed step-up ticket was accepted again: %d %s", second.status, second.raw)
	}

	f.must(http.MethodPost, "/api/v1/auth/step-up", buyer.token,
		map[string]any{"password": "definitely-wrong", "action": "marketplace_purchase"}, http.StatusForbidden)
	_ = w
}

func TestFederationRefusesAnIssuerThatWasNeverConfigured(t *testing.T) {
	f := newFixture(t)
	result := f.call(http.MethodPost, "/api/v1/auth/federation/exchange", "", map[string]any{
		"issuer": "https://attacker.example.test", "token": "forged-assertion",
	})
	if result.status == http.StatusOK || result.status == http.StatusCreated {
		t.Fatalf("an untrusted issuer produced a session: %s", result.raw)
	}
	var sessions int64
	if err := f.app.DB.Model(&models.LocalSession{}).Count(&sessions).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("%d sessions exist after a rejected federation exchange", sessions)
	}
}

func TestAnchorRetryPreservesTheOriginalCommitment(t *testing.T) {
	f := newFixture(t)
	w := f.completed()
	completed := f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/provenance", w.manager.token, nil, http.StatusOK)
	var eventID, commitment string
	for _, item := range completed.list("events") {
		event := item.(map[string]any)
		if event["type"] == models.ClaimCompletion {
			eventID, _ = event["id"].(string)
			commitment, _ = event["commitment"].(string)
		}
	}
	if eventID == "" || commitment == "" {
		t.Fatalf("no completion commitment was recorded: %s", completed.raw)
	}
	// An anchored claim is not re-anchored, so a retry cannot mint a second
	// commitment for evidence that is already confirmed.
	anchored := f.call(http.MethodPost, "/api/v1/activities/"+w.activity+"/anchor/retry", w.manager.token, map[string]any{})
	if anchored.status != http.StatusConflict {
		t.Fatalf("retrying an anchored claim returned %d: %s", anchored.status, anchored.raw)
	}
	// A failed submission is the case retry exists for. The stored event is marked
	// failed to stand in for a chain outage; the retried commitment must be the
	// same bytes, because the evidence did not change.
	if err := f.app.DB.Model(&models.ProvenanceEvent{}).Where("id = ?", eventID).
		Update("status", provenance.StatusFailed).Error; err != nil {
		t.Fatalf("mark the anchor failed: %v", err)
	}
	retried := f.must(http.MethodPost, "/api/v1/activities/"+w.activity+"/anchor/retry", w.manager.token, map[string]any{}, http.StatusOK)
	if got := retried.str("event", "commitment"); got != commitment {
		t.Fatalf("retry changed the commitment from %s to %s", commitment, got)
	}
	status := f.must(http.MethodGet, "/api/v1/anchors/"+eventID+"/status", w.manager.token, nil, http.StatusOK)
	if status.str("network") != "sandbox" {
		t.Fatalf("anchor status network = %q, want sandbox", status.str("network"))
	}
}

func TestLiveChainModeNeverSilentlyDowngrades(t *testing.T) {
	_, err := app.ChainAdapter(config.Config{ChainMode: "iota", IOTANetwork: "testnet"})
	if err == nil {
		t.Fatal("CHAIN_MODE=iota without an executor silently fell back to another adapter")
	}
}

func TestContextCancellationDoesNotFabricateAReceipt(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/batch", strings.NewReader(`{"actions":[]}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.manager.token)
	recorder := httptest.NewRecorder()
	f.app.Handler.ServeHTTP(recorder, req)
	if recorder.Code == http.StatusCreated {
		t.Fatal("a cancelled request produced a successful receipt")
	}
}
