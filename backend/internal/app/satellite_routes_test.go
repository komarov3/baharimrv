package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
)

// Both project satellite handlers existed for months without ever being mounted,
// which Android could only ever see as a 404. These assertions fail if either
// path disappears again, and they run against the assembled graph so they cannot
// pass against wiring that no real client uses.
func TestSatelliteProjectRoutesAreMounted(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	passes := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/passes?mission=S2", w.manager.token, nil, http.StatusOK)
	items := passes.list("items")
	if len(items) == 0 {
		t.Fatalf("expected sandbox passes, got %s", passes.raw)
	}
	first, _ := items[0].(map[string]any)
	if mode, _ := first["adapterMode"].(string); mode != "sandbox" {
		t.Fatalf("expected sandbox adapter mode, got %v in %s", first["adapterMode"], passes.raw)
	}

	latest := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/latest?mission=S2", w.manager.token, nil, http.StatusOK)
	if latest.body["pass"] == nil {
		t.Fatalf("expected a latest pass, got %s", latest.raw)
	}
}

// The route is new, so it also has to prove it is not a public window onto
// another manager's project.
func TestSatelliteProjectRoutesEnforceAccess(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	outsider := f.register(models.RoleManager, "outsider")

	for _, path := range []string{"/satellite/passes", "/satellite/latest"} {
		f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+path, outsider.token, nil, http.StatusForbidden)
	}
	f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/passes", "", nil, http.StatusUnauthorized)
}

// A manager who has never asked for a refresh sees the refusal, not an invented
// figure, so the client offers a refresh instead of drawing nothing.
func TestSatelliteChangeIsRefusedUntilComputed(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	outsider := f.register(models.RoleManager, "outsider")

	change := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/change", w.manager.token, nil, http.StatusNotFound)
	if code := change.str("error", "code"); code != "satellite_summary_missing" {
		t.Fatalf("expected the missing-summary code, got %q in %s", code, change.raw)
	}

	f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/change", outsider.token, nil, http.StatusForbidden)
	f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/change", "", nil, http.StatusUnauthorized)
}

// The refresh is manager-only and a second tap does not stack a job on the one
// already open, because the lock lives in the database and not in a spinner.
func TestSatelliteRefreshIsManagerOnlyAndSingleFlight(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	outsider := f.register(models.RoleManager, "refresh-outsider")
	monitor := f.register(models.RoleMonitor, "refresh-monitor")

	first := f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/satellite/refresh", w.manager.token,
		map[string]any{"reason": "first"}, http.StatusAccepted)
	if status, _ := first.body["status"].(string); status != "queued" {
		t.Fatalf("queued job = %s", first.raw)
	}
	jobID, _ := first.body["jobId"].(string)
	if jobID == "" {
		t.Fatalf("missing job id: %s", first.raw)
	}
	// Same manager tapping again.
	second := f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/satellite/refresh", w.manager.token,
		map[string]any{"reason": "again"}, http.StatusConflict)
	if conflictID, _ := second.body["jobId"].(string); conflictID != jobID {
		t.Errorf("409 body named a different job: %s (original %s)", conflictID, jobID)
	}

	for _, token := range []string{outsider.token, monitor.token, ""} {
		want := http.StatusForbidden
		if token == "" {
			want = http.StatusUnauthorized
		}
		f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/satellite/refresh", token, nil, want)
	}
}

// A member who was told about a job can watch it, and a monitor assigned to the
// project is a member too. A manager of another project cannot read it by guessing
// the id.
func TestSatelliteRefreshStatusIsScopedToItsProject(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	outsider := f.register(models.RoleManager, "outsider")

	first := f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/satellite/refresh", w.manager.token, nil, http.StatusAccepted)
	jobID, _ := first.body["jobId"].(string)

	view := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/refreshes/"+jobID, w.manager.token, nil, http.StatusOK)
	if status, _ := view.body["status"].(string); status == "" {
		t.Fatalf("status missing in %s", view.raw)
	}
	f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/refreshes/"+jobID, w.monitor.token, nil, http.StatusOK)
	f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/refreshes/"+jobID, outsider.token, nil, http.StatusForbidden)
	f.must(http.MethodGet, "/api/v1/projects/00000000-0000-0000-0000-000000000000/satellite/refreshes/"+jobID, w.manager.token, nil, http.StatusForbidden)
}

// Once the worker finishes, the headline carries the project figure. Sandbox mode
// is exercised here because the live seam needs a configured worker; what the
// client reads is the same envelope either way.
func TestSatelliteChangeAfterARefresh(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	first := f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/satellite/refresh", w.manager.token, nil, http.StatusAccepted)
	jobID, _ := first.body["jobId"].(string)
	// The test fixture does not run the refresh loop; drive one pass directly so
	// the assertion is about the compute, not about polling a ticker.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	finished, err := f.app.Satellite.ProcessRefreshJobs(ctx)
	if err != nil {
		t.Fatalf("process refresh jobs: %v", err)
	}
	if finished != 1 {
		t.Fatalf("expected one finished job, got %d", finished)
	}
	view := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/refreshes/"+jobID, w.manager.token, nil, http.StatusOK)
	if status, _ := view.body["status"].(string); status != "complete" {
		t.Fatalf("job status = %q: %s", status, view.raw)
	}
	change := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/change", w.manager.token, nil, http.StatusOK)
	for _, key := range []string{"summaryId", "quality", "adapterMode", "resultHash", "coveragePct", "plotCount"} {
		if _, ok := change.body[key]; !ok {
			t.Errorf("change body is missing %s: %s", key, change.raw)
		}
	}
	if mode, _ := change.body["adapterMode"].(string); mode != "sandbox" {
		t.Errorf("adapterMode = %q, want sandbox in this build", mode)
	}
}

// The overlay carries the same envelope for sandbox and live responses so the
// client never branches on structure; a sandbox row drops tileBaseUrl and the
// scene id rather than sending an empty string.
func TestSatelliteOverlaySandboxShape(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	outsider := f.register(models.RoleManager, "overlay-outsider")

	// The refresh path produces a sandbox summary the overlay can read.
	f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/satellite/refresh", w.manager.token, nil, http.StatusAccepted)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := f.app.Satellite.ProcessRefreshJobs(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	overlay := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/overlay", w.manager.token, nil, http.StatusOK)
	for _, key := range []string{"layer", "quality", "adapterMode", "legend", "cacheExpiresAt", "summaryId"} {
		if _, ok := overlay.body[key]; !ok {
			t.Errorf("overlay missing %s: %s", key, overlay.raw)
		}
	}
	if mode, _ := overlay.body["adapterMode"].(string); mode != "sandbox" {
		t.Errorf("adapterMode = %q, want sandbox", mode)
	}
	if _, ok := overlay.body["tileBaseUrl"]; ok {
		t.Errorf("sandbox overlay must not advertise tileBaseUrl: %s", overlay.raw)
	}
	if scene, _ := overlay.body["sceneId"].(string); scene != "" {
		t.Errorf("sandbox overlay must not name a scene: %s", overlay.raw)
	}

	f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/overlay", outsider.token, nil, http.StatusForbidden)
	f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/overlay", "", nil, http.StatusUnauthorized)
}

// A project with no summary answers 404 rather than inventing an envelope,
// so the client offers a refresh instead of drawing nothing.
func TestSatelliteOverlayBeforeComputeIsRefused(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	resp := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/satellite/overlay", w.manager.token, nil, http.StatusNotFound)
	if code := resp.str("error", "code"); code != "satellite_summary_missing" {
		t.Fatalf("expected satellite_summary_missing, got %q in %s", code, resp.raw)
	}
}

// The tile proxy refuses every request the overlay could not have named: a
// missing scene, a bad layer, and a zoom outside the worker's grid all answer
// with typed 422s so the client can log them rather than guessing.
func TestSatelliteTileProxyValidatesInputs(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	base := "/api/v1/projects/" + w.projectID + "/satellite/tiles/14/9012/7234.png"

	f.must(http.MethodGet, base, w.manager.token, nil, http.StatusUnprocessableEntity)
	f.must(http.MethodGet, base+"?scene=SOME_SCENE", w.manager.token, nil, http.StatusUnprocessableEntity)
	f.must(http.MethodGet, base+"?scene=SOME_SCENE&layer=other", w.manager.token, nil, http.StatusUnprocessableEntity)
}

// The tile proxy answers 410 when a caller names a scene the project has moved
// past. The fixture is sandbox (no render ran), so every tile request for any
// scene is "the project moved past that scene" from the proxy's point of view.
func TestSatelliteTileProxySupersedesStaleScene(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/satellite/refresh", w.manager.token, nil, http.StatusAccepted)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := f.app.Satellite.ProcessRefreshJobs(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	url := "/api/v1/projects/" + w.projectID + "/satellite/tiles/14/9012/7234.png?scene=STALE&layer=ndvi"
	resp := f.must(http.MethodGet, url, w.manager.token, nil, http.StatusGone)
	if code := resp.str("error", "code"); code != "satellite_scene_superseded" {
		t.Fatalf("expected satellite_scene_superseded, got %q in %s", code, resp.raw)
	}
}

// The admin route is admin-only and answers the shape the operator reads: a
// provider name, the tile-store flag, the cache counters, and the job backlog.
func TestAdminSatelliteHealthIsAdminOnly(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	f.must(http.MethodGet, "/api/v1/admin/satellite/health", w.manager.token, nil, http.StatusForbidden)
	f.must(http.MethodGet, "/api/v1/admin/satellite/health", "", nil, http.StatusUnauthorized)
	admin := f.registerAdmin("satellite-admin")
	view := f.must(http.MethodGet, "/api/v1/admin/satellite/health", admin.token, nil, http.StatusOK)
	for _, key := range []string{"provider", "adapterMode", "tileStoreAvailable", "tileCache", "queuedJobs", "runningJobs"} {
		if _, ok := view.body[key]; !ok {
			t.Errorf("health missing %s: %s", key, view.raw)
		}
	}
}