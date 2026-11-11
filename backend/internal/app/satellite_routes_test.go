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