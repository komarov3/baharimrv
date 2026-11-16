package mrv

import (
	"encoding/json"
	"testing"

	"github.com/baharilink/baharimrv/internal/geo"
	"github.com/baharilink/baharimrv/internal/models"
	"gorm.io/datatypes"
)

func TestResolveProjectBoundaryAbsentIsEmpty(t *testing.T) {
	raw, ring, err := resolveProjectBoundary(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw != nil || ring != nil {
		t.Fatalf("expected nil boundary and ring, got %v %v", raw, ring)
	}
	if _, ring, err := resolveProjectBoundary(json.RawMessage("null")); err != nil || ring != nil {
		t.Fatalf("a null boundary must be absent, got ring=%v err=%v", ring, err)
	}
}

func TestResolveProjectBoundaryValidSquare(t *testing.T) {
	raw, ring, err := resolveProjectBoundary(boundaryJSON(-4.05, 39.67, 200))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raw) == 0 || ring == nil {
		t.Fatal("expected stored bytes and a parsed ring")
	}
}

func TestResolveProjectBoundaryRejectsBadRing(t *testing.T) {
	if _, _, err := resolveProjectBoundary(json.RawMessage(`{"type":"Point","coordinates":[0,0]}`)); err == nil {
		t.Error("expected a non-Polygon to be rejected")
	}
	if _, _, err := resolveProjectBoundary(json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[0,0],[0,0],[0,0]]]}`)); err == nil {
		t.Error("expected a zero-area boundary to be rejected")
	}
}

func TestAssertPlotInsideBoundary(t *testing.T) {
	_, ring, err := resolveProjectBoundary(boundaryJSON(-4.05, 39.67, 200))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := assertPlotInsideBoundary("P1", ring, true, -4.05, 39.67); err != nil {
		t.Errorf("centroid at the centre must be inside: %v", err)
	}
	if err := assertPlotInsideBoundary("P1", ring, true, -4.06, 39.67); err == nil {
		t.Error("a centroid well outside the outline must be rejected")
	}
	// No boundary: nothing to enforce, even for a far-away point.
	if err := assertPlotInsideBoundary("P1", geo.Ring(nil), false, 80, 80); err != nil {
		t.Errorf("a project with no boundary must not restrict plots: %v", err)
	}
}

func TestMergePlotPatchKeepsStoredValues(t *testing.T) {
	storedBoundary := datatypes.JSON(boundaryJSON(-4.05, 39.67, 100))
	acc := 12.5
	plot := models.Plot{
		Code: "P1", Latitude: -4.05, Longitude: 39.67, AreaHectares: "1.000000",
		AssignedMonitorID: "monitor-1", Boundary: storedBoundary, GpsAccuracyM: &acc,
	}
	// A monitor-assignment body keeps everything else and applies the ID verbatim.
	assigned := "monitor-2"
	input := mergePlotPatch(plot, PlotPatch{AssignedMonitorID: &assigned})
	if input.Code != "P1" || input.Latitude != -4.05 || input.Longitude != 39.67 {
		t.Errorf("identity fields must survive the merge: %+v", input)
	}
	if input.AreaHectares != "1.000000" || input.AssignedMonitorID != "monitor-2" {
		t.Errorf("area kept, monitor replaced: %+v", input)
	}
	if string(input.Boundary) != string(storedBoundary) || input.GpsAccuracyM != &acc {
		t.Error("boundary and accuracy must be carried over untouched")
	}
	// An empty assignedMonitorId clears the assignment.
	if got := mergePlotPatch(plot, PlotPatch{AssignedMonitorID: strptr("")}); got.AssignedMonitorID != "" {
		t.Error("an explicit empty monitor id must clear the assignment")
	}
	// An absent boundary key keeps the outline; an explicit null clears it.
	if got := mergePlotPatch(plot, PlotPatch{}); string(got.Boundary) != string(storedBoundary) {
		t.Error("an absent boundary key must keep the stored outline")
	}
	if got := mergePlotPatch(plot, PlotPatch{Boundary: json.RawMessage("null")}); string(got.Boundary) != "null" {
		t.Error("an explicit null boundary must be passed through for clearing")
	}
	// Overriding coordinates.
	if got := mergePlotPatch(plot, PlotPatch{Latitude: fptr(-4.10), Longitude: fptr(39.70)}); got.Latitude != -4.10 || got.Longitude != 39.70 {
		t.Error("provided coordinates must override the stored point")
	}
}

func strptr(s string) *string { return &s }
func fptr(v float64) *float64 { return &v }
