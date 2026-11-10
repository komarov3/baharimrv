package app_test

import (
	"net/http"
	"testing"

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
