package satellite

import (
	"encoding/json"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/database"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// The worker recorded this square during the Kilifi spike, through its own aoi.cell.
// If the Go arithmetic drifts from the Python, the sample moves off the plot a manager
// registered and neither side reports a fault, so the two are pinned to each other here.
func TestCellRingReproducesTheRecordedKilifiSquare(t *testing.T) {
	var recorded polygonGeometry
	if err := json.Unmarshal([]byte(cellPolygon), &recorded); err != nil {
		t.Fatalf("parse recorded cell: %v", err)
	}
	ring := cellRing(-3.64, 39.836, 2.5)
	if len(recorded.Coordinates) != 1 || len(ring) != len(recorded.Coordinates[0]) {
		t.Fatalf("ring %v against recorded %v", ring, recorded.Coordinates)
	}
	for index, point := range ring {
		want := recorded.Coordinates[0][index]
		if math.Abs(point[0]-want[0]) > 1e-9 || math.Abs(point[1]-want[1]) > 1e-9 {
			t.Errorf("vertex %d = %v, want the recorded %v", index, point, want)
		}
	}
	if ring[0] != ring[len(ring)-1] {
		t.Errorf("ring is not closed: %v", ring)
	}
}

// Section 5.1: a 0.5 ha plot at latitude -3.6 is a square about 71 m per side, which at
// 10 m resolution is the 7 by 7 pixel block the method promises.
func TestHalfHectareCellIsAboutSeventyOneMetres(t *testing.T) {
	const lat = -3.6
	ring := cellRing(lat, 39.0, 0.5)
	northSouth := (ring[2][1] - ring[0][1]) * metresPerDegreeLat
	eastWest := (ring[1][0] - ring[0][0]) * metresPerDegreeLat * math.Cos(lat*math.Pi/180)
	if math.Abs(northSouth-70.71) > 0.01 {
		t.Errorf("north-south side = %v m, want about 70.71", northSouth)
	}
	if math.Abs(eastWest-northSouth) > 0.001 {
		t.Errorf("cell is not square on the ground: %v m by %v m", eastWest, northSouth)
	}
}

func TestTinyPlotGetsTheThirtyMetreFloor(t *testing.T) {
	ring := cellRing(-3.6, 39.0, 0.01)
	side := (ring[2][1] - ring[0][1]) * metresPerDegreeLat
	if math.Abs(side-minCellSideMetres) > 1e-6 {
		t.Errorf("side = %v m, want the %v m floor", side, minCellSideMetres)
	}
}

func TestPlotAreaRejectsWhatItCannotRead(t *testing.T) {
	for _, value := range []string{"", "abc", "NaN", "+Inf", "-2.5"} {
		if area := plotArea(value); area != 0 {
			t.Errorf("plotArea(%q) = %v, want 0 so the side floor applies", value, area)
		}
	}
	if area := plotArea(" 2.500000 "); area != 2.5 {
		t.Errorf("plotArea of a stored numeric string = %v, want 2.5", area)
	}
}

// Two plots 40 m apart with 71 m cells overlap on the ground. They must still arrive as
// one analysis geometry, because the scene search and the coverage screen are asked
// about the project, not about each plot in turn.
func TestOverlappingCellsShareOneAnalysisGeometry(t *testing.T) {
	const lat = -3.6
	const lon = 39.0
	apart := 40 / (metresPerDegreeLat * math.Cos(lat*math.Pi/180))
	plots := []models.Plot{
		{ID: "plot-west", Code: "P-1", Latitude: lat, Longitude: lon, AreaHectares: "0.500000"},
		{ID: "plot-east", Code: "P-2", Latitude: lat, Longitude: lon + apart, AreaHectares: "0.500000"},
	}
	aoi := aoiFromPlots(plots)
	if aoi.PlotCount != 2 || len(aoi.Cells) != 2 {
		t.Fatalf("aoi = %+v, want two cells", aoi)
	}
	var geometry multiPolygonGeometry
	if err := json.Unmarshal(aoi.AnalysisGeom, &geometry); err != nil {
		t.Fatalf("parse analysis geometry: %v", err)
	}
	if geometry.Type != "MultiPolygon" || len(geometry.Coordinates) != 2 {
		t.Fatalf("analysis geometry = %s, want one MultiPolygon over both cells", aoi.AnalysisGeom)
	}
	west, east := geometry.Coordinates[0][0], geometry.Coordinates[1][0]
	if west[1][0] <= east[0][0] {
		t.Errorf("cells 40 m apart do not overlap: west ends at %v, east starts at %v", west[1][0], east[0][0])
	}
	if aoi.FramingBBox[0] != west[0][0] || aoi.FramingBBox[2] != east[1][0] {
		t.Errorf("framing bbox %v does not span both cells", aoi.FramingBBox)
	}
	if len(aoi.FramingBBox) != 4 || aoi.FramingBBox[1] > aoi.FramingBBox[3] {
		t.Errorf("framing bbox = %v, want west, south, east, north", aoi.FramingBBox)
	}
}

func TestNoPlotsHasNoGeometry(t *testing.T) {
	aoi := aoiFromPlots(nil)
	if len(aoi.Cells) != 0 || len(aoi.AnalysisGeom) != 0 || len(aoi.FramingBBox) != 0 {
		t.Errorf("empty project produced an analysis area: %+v", aoi)
	}
}

// openAOIDatabase seeds the rows a plot's foreign keys point at, so the test does not
// depend on whether the driver happens to enforce them.
func openAOIDatabase(t *testing.T) (*gorm.DB, string, string) {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "aoi.db") + "?_busy_timeout=5000"
	db, err := database.Open(dsn, false)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	now := time.Now().UTC()
	manager := models.User{
		ID: uuid.NewString(), Email: "manager@aoi.test", PasswordHash: "hash", DisplayName: "Manager",
		Role: models.RoleManager, Status: models.IdentityActive, CreatedAt: now, UpdatedAt: now,
	}
	methodology := models.Methodology{
		ID: uuid.NewString(), Code: "AOI-TEST", Name: "AOI test", Version: "1.0",
		RequiredEvidence: "One photo.", Estimation: datatypes.JSON([]byte(`{}`)), CreatedAt: now, UpdatedAt: now,
	}
	project := models.ConservationProject{
		ID: uuid.NewString(), Name: "Kilifi creek", ManagerID: manager.ID, MethodologyID: methodology.ID,
		Status: "active", CommunityShareBPS: 6000, CreatedAt: now, UpdatedAt: now,
	}
	site := models.Site{ID: uuid.NewString(), ProjectID: project.ID, Name: "Mikoko Park", CreatedAt: now, UpdatedAt: now}
	for _, row := range []any{&manager, &methodology, &project, &site} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed %T: %v", row, err)
		}
	}
	return db, project.ID, site.ID
}

func TestProjectAOIOrdersCellsSoTheHashIsStable(t *testing.T) {
	db, projectID, siteID := openAOIDatabase(t)
	now := time.Now().UTC()
	// Inserted out of order on purpose: the geometry is hashed into the summary, so the
	// query has to fix the order rather than inherit whatever the driver returns.
	second := models.Plot{ID: uuid.NewString(), ProjectID: projectID, SiteID: siteID, Code: "P-2",
		Latitude: -3.64, Longitude: 39.836, AreaHectares: "2.500000", CreatedAt: now, UpdatedAt: now}
	first := models.Plot{ID: uuid.NewString(), ProjectID: projectID, SiteID: siteID, Code: "P-1",
		Latitude: -3.6404, Longitude: 39.8364, AreaHectares: "0.500000", CreatedAt: now, UpdatedAt: now}
	for _, plot := range []models.Plot{second, first} {
		if err := db.Create(&plot).Error; err != nil {
			t.Fatalf("create plot: %v", err)
		}
	}
	service := NewService(db, Options{})
	aoi, err := service.ProjectAOI(projectID)
	if err != nil {
		t.Fatalf("project aoi: %v", err)
	}
	if aoi.Status != "available" || aoi.ProjectID != projectID || aoi.PlotCount != 2 {
		t.Fatalf("aoi = %+v", aoi)
	}
	if aoi.Cells[0].PlotID != first.ID || aoi.Cells[1].PlotID != second.ID {
		t.Errorf("cells are not in code order: %v then %v", aoi.Cells[0].PlotID, aoi.Cells[1].PlotID)
	}
	if aoi.Cells[0].AreaHectares != 0.5 || aoi.Cells[1].AreaHectares != 2.5 {
		t.Errorf("cell areas = %v, %v", aoi.Cells[0].AreaHectares, aoi.Cells[1].AreaHectares)
	}
	again, err := service.ProjectAOI(projectID)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if string(again.AnalysisGeom) != string(aoi.AnalysisGeom) {
		t.Errorf("the same project produced two different geometries:\n%s\n%s", aoi.AnalysisGeom, again.AnalysisGeom)
	}
}

func TestProjectAOIWithoutPlotsIsUnavailable(t *testing.T) {
	db, projectID, _ := openAOIDatabase(t)
	aoi, err := NewService(db, Options{}).ProjectAOI(projectID)
	if err != nil {
		t.Fatalf("project aoi: %v", err)
	}
	if aoi.Status != "unavailable" || len(aoi.Cells) != 0 || len(aoi.AnalysisGeom) != 0 {
		t.Errorf("aoi = %+v, want an unavailable project with no cells to send", aoi)
	}
}
