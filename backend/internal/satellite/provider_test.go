package satellite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures in testdata are the recorded answers of s2-worker, produced by
// services/s2-worker/tests/record_route_fixtures.py from the same job functions the
// routes call. These tests are the provider contract in SATELLITE_NDVI_SPEC.md 13: a
// reply the seam cannot read, or a request the worker would refuse, fails a build here
// rather than in a live refresh.
const (
	fixtureSceneID   = "S2C_37MER_20261105_1_L2A"
	fixtureOmittedID = "S2B_37MER_20261110_0_L2A"
	fixtureProject   = "5f0c8fa4-6f3c-4f2c-9b41-7d8e0c1a2b3c"
	fixtureToken     = "worker-test-token"
	fixtureCellID    = "plot-1"
)

// fixtureNow is when the test deployment thinks it is, so the baseline lookback is a
// checked number and not a race with the clock.
var fixtureNow = time.Date(2026, 11, 10, 6, 0, 0, 0, time.UTC)

// The 2.5 ha cell at Kilifi creek, 158 m per side, as the spike recorded it.
const (
	cellPolygon = `{"type":"Polygon","coordinates":[[[39.835288387086955,-3.6407101773401385],` +
		`[39.83671161291304,-3.6407101773401385],[39.83671161291304,-3.639289822659862],` +
		`[39.835288387086955,-3.639289822659862],[39.835288387086955,-3.6407101773401385]]]}`
	cellBBox = "[39.835288387086955,-3.6407101773401385,39.83671161291304,-3.639289822659862]"
)

func fixtureAOI() AOI {
	return AOI{
		ProjectID:    fixtureProject,
		PlotCount:    1,
		Cells:        []Cell{{PlotID: fixtureCellID, Geometry: json.RawMessage(cellPolygon), AreaHectares: 2.5}},
		AnalysisGeom: json.RawMessage(cellPolygon),
		FramingBBox:  []float64{39.8352884, -3.6407102, 39.8367116, -3.6392898},
	}
}

func fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return body
}

// capturedCall is one request the seam made, so a test can assert what the worker was
// told as well as what the seam did with the answer.
type capturedCall struct {
	path string
	auth string
	body map[string]any
	raw  string
}

type fakeWorker struct {
	t       *testing.T
	replies map[string]any
	status  map[string]int
	server  *httptest.Server
	calls   []capturedCall
}

func newFakeWorker(t *testing.T, replies map[string]any) *fakeWorker {
	worker := &fakeWorker{t: t, replies: replies, status: map[string]int{}}
	worker.server = httptest.NewServer(worker)
	t.Cleanup(worker.server.Close)
	return worker
}

func (f *fakeWorker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	call := capturedCall{path: r.URL.Path, auth: r.Header.Get("Authorization")}
	decoder := json.NewDecoder(r.Body)
	body := map[string]any{}
	if err := decoder.Decode(&body); err == nil {
		call.body = body
		payload, _ := json.Marshal(body)
		call.raw = string(payload)
	}
	f.calls = append(f.calls, call)

	reply, offered := f.replies[r.URL.Path]
	if !offered {
		f.t.Errorf("the seam called %s, which this test did not offer an answer for", r.URL.Path)
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	if status := f.status[r.URL.Path]; status != 0 {
		w.WriteHeader(status)
	}
	if text, isString := reply.(string); isString {
		fmt.Fprint(w, text)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reply)
}

func (f *fakeWorker) provider() *OpenDataStacProvider {
	worker := NewOpenDataStac(f.server.URL, fixtureToken)
	worker.now = func() time.Time { return fixtureNow }
	return worker
}

// call returns the single request made to a path, failing when the seam called it a
// different number of times: one granule search and one measurement is the contract.
func (f *fakeWorker) call(path string) capturedCall {
	f.t.Helper()
	var found []capturedCall
	for _, call := range f.calls {
		if call.path == path {
			found = append(found, call)
		}
	}
	if len(found) != 1 {
		f.t.Fatalf("%s was called %d time(s), want exactly 1", path, len(found))
	}
	return found[0]
}

func (f *fakeWorker) count(path string) int {
	total := 0
	for _, call := range f.calls {
		if call.path == path {
			total++
		}
	}
	return total
}

func number(t *testing.T, value *float64, field string) float64 {
	t.Helper()
	if value == nil {
		t.Fatalf("%s is null, but this test expects a measured figure", field)
	}
	return *value
}

func TestLatestObservationReadsTheRecordedWorker(t *testing.T) {
	sceneFixture := fixture(t, "scene.json")
	worker := newFakeWorker(t, map[string]any{
		"/v1/scene": sceneFixture,
		"/v1/stats": fixture(t, "stats.json"),
	})

	scene, err := worker.provider().LatestObservation(context.Background(), fixtureAOI(), 45*24*time.Hour)
	if err != nil {
		t.Fatalf("a real granule must not be an error: %v", err)
	}
	if scene.SceneID != fixtureSceneID {
		t.Errorf("scene = %q, want %q", scene.SceneID, fixtureSceneID)
	}
	if scene.Quality != QualityComplete {
		t.Errorf("quality = %q, want %q", scene.Quality, QualityComplete)
	}
	// The golden figures of SATELLITE_NDVI_SPEC.md 5.3, through the seam.
	if got := number(t, scene.MeanNDVI, "mean_ndvi"); got != 0.837 {
		t.Errorf("mean = %v, want 0.837", got)
	}
	if got := number(t, scene.MedianNDVI, "median_ndvi"); got != 0.8476 {
		t.Errorf("median = %v, want 0.8476", got)
	}
	if scene.ValidPixels != 289 || scene.ExpectedPixels != 289 || scene.CoveragePct != 100.0 {
		t.Errorf("pixels = %d/%d, coverage %v, want 289/289 at 100",
			scene.ValidPixels, scene.ExpectedPixels, scene.CoveragePct)
	}
	if len(scene.PerCell) != 1 || scene.PerCell[0].PlotID != fixtureCellID {
		t.Errorf("per_cell = %+v, want the one cell that was asked for", scene.PerCell)
	}
	// Provenance the anchored payload has to carry.
	if scene.ClassTable != "msk-0509/1" || scene.PaletteVersion != "ndvi-5band/1" {
		t.Errorf("method = %q / %q, want msk-0509/1 and ndvi-5band/1", scene.ClassTable, scene.PaletteVersion)
	}
	if scene.ProcessorBaseline != "05.13" || scene.DiscoverySource != "element84-stac" {
		t.Errorf("granule came from %q at baseline %q", scene.DiscoverySource, scene.ProcessorBaseline)
	}
	if scene.Grid != "37MER" || scene.EPSG != 32737 || scene.Platform != "sentinel-2c" {
		t.Errorf("grid %q epsg %d platform %q", scene.Grid, scene.EPSG, scene.Platform)
	}
	if !scene.AcquiredAt.Equal(time.Date(2026, 11, 5, 7, 50, 55, 647000000, time.UTC)) {
		t.Errorf("acquired = %s", scene.AcquiredAt)
	}
	if scene.AdapterMode != ModeLive || scene.ProviderName != ProviderOpenData {
		t.Errorf("labelled %s/%s, want live/opendata-s2", scene.AdapterMode, scene.ProviderName)
	}
	if scene.ProcessingVersion != LiveProcessingVersion {
		t.Errorf("processing version = %q, want %q", scene.ProcessingVersion, LiveProcessingVersion)
	}
	if scene.PlotCount != 1 {
		t.Errorf("plot count = %d, want 1", scene.PlotCount)
	}
	// The walk is what makes an older granule auditable against a newer one.
	if len(scene.Screened) != 2 || scene.Screened[0].SceneID != fixtureOmittedID {
		t.Fatalf("screened = %+v, want both candidates the recording saw", scene.Screened)
	}
	if scene.Screened[0].UsablePct != 0.0 {
		t.Errorf("the freshest granule was %v percent usable, want the recorded 0", scene.Screened[0].UsablePct)
	}
	if scene.Window.Days != 45 || len(scene.Window.CloudCeilings) != 2 {
		t.Errorf("window = %+v", scene.Window)
	}
	if len(scene.Notes) != len(sceneFixture["notes"].([]any)) {
		t.Errorf("notes = %q, want the recorded walk", scene.Notes)
	}
	if !strings.Contains(strings.Join(scene.Notes, " "), "copernicus-stac:") {
		t.Errorf("notes %q do not say which catalog refused", scene.Notes)
	}

	search := worker.call("/v1/scene")
	if want := "Bearer " + fixtureToken; search.auth != want {
		t.Errorf("authorization = %q, want %q", search.auth, want)
	}
	if days, _ := search.body["window_days"].(float64); days != 45 {
		t.Errorf("window_days = %v, want 45", days)
	}
	// A date-only upper bound is midnight UTC and silently drops that morning's pass.
	endTime, _ := search.body["end_time"].(string)
	parsed, err := time.Parse(time.RFC3339, endTime)
	if err != nil || !parsed.Equal(fixtureNow) {
		t.Errorf("end_time = %q, want %s as a full timestamp", endTime, fixtureNow)
	}
	if shape, _ := search.body["aoi_geojson"].(map[string]any); shape["type"] != "Polygon" {
		t.Errorf("aoi_geojson = %v, want the cell union Polygon", shape)
	}

	measure := worker.call("/v1/stats")
	if weighted, _ := measure.body["weight_by_area"].(bool); !weighted {
		t.Error("the project mean must be area-weighted")
	}
	cells, _ := measure.body["cells"].([]any)
	if len(cells) != 1 {
		t.Fatalf("cells = %v, want the one analysis cell", measure.body["cells"])
	}
	sent := cells[0].(map[string]any)
	if sent["plot_id"] != fixtureCellID {
		t.Errorf("cell id = %v", sent["plot_id"])
	}
	if _, isGeometry := sent["geometry"].(map[string]any); !isGeometry {
		t.Errorf("cell geometry did not arrive as an object: %v", sent["geometry"])
	}
	// The worker keeps no state, so the band references have to come back unchanged.
	sentScene, _ := measure.body["scene"].(map[string]any)
	bands, _ := sentScene["bands"].(map[string]any)
	wantRed := sceneFixture["bands"].(map[string]any)["red"]
	if bands["red"] != wantRed {
		t.Errorf("scene.bands.red = %v, want it sent back as %v", bands["red"], wantRed)
	}
	if sentScene["scene_id"] != fixtureSceneID || sentScene["processor_baseline"] != "05.13" {
		t.Errorf("scene reference sent = %v", sentScene)
	}
}

func TestBaselineObservationLooksBackOneYear(t *testing.T) {
	worker := newFakeWorker(t, map[string]any{
		"/v1/scene": fixture(t, "scene.json"),
		"/v1/stats": fixture(t, "stats.json"),
	})
	if _, err := worker.provider().BaselineObservation(context.Background(), fixtureAOI(), 45*24*time.Hour); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	endTime, _ := worker.call("/v1/scene").body["end_time"].(string)
	parsed, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		t.Fatalf("end_time %q: %v", endTime, err)
	}
	// Section 5.2: the same calendar window one year earlier, so start = end - 45d.
	if want := fixtureNow.AddDate(-1, 0, 0); !parsed.Equal(want) {
		t.Errorf("baseline window ends %s, want %s", parsed, want)
	}
	if days, _ := worker.call("/v1/scene").body["window_days"].(float64); days != 45 {
		t.Errorf("window_days = %v, want the same 45 day season", days)
	}
}

func TestQualityIsTheWorseOfTheTwoAnswers(t *testing.T) {
	for _, test := range []struct {
		name, scene, stats, want string
	}{
		{"clean granule read in full", QualityComplete, QualityComplete, QualityComplete},
		{"adopted on the hazy rung", QualityCloudy, QualityComplete, QualityCloudy},
		{"pixel budget blown", QualityComplete, QualityPartial, QualityPartial},
		{"hazy granule, incomplete read", QualityCloudy, QualityPartial, QualityPartial},
	} {
		t.Run(test.name, func(t *testing.T) {
			scene := fixture(t, "scene.json")
			scene["quality"] = test.scene
			stats := fixture(t, "stats.json")
			stats["quality"] = test.stats
			worker := newFakeWorker(t, map[string]any{"/v1/scene": scene, "/v1/stats": stats})

			got, err := worker.provider().LatestObservation(context.Background(), fixtureAOI(), 45*24*time.Hour)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if got.Quality != test.want {
				t.Errorf("quality = %q, want %q", got.Quality, test.want)
			}
		})
	}
}

func TestNoSceneIsAnUnavailableResultAndNotAnError(t *testing.T) {
	worker := newFakeWorker(t, map[string]any{
		"/v1/scene": `{"error":"unavailable","detail":"no granule in 2026-09-26/2026-11-10",` +
			`"notes":["copernicus-stac: https://stac.dataspace.copernicus.eu/v1: timeout"]}`,
	})
	worker.status["/v1/scene"] = http.StatusNotFound

	scene, err := worker.provider().LatestObservation(context.Background(), fixtureAOI(), 45*24*time.Hour)
	if err != nil {
		t.Fatalf("an empty window is a state, not a fault: %v", err)
	}
	if scene.Quality != QualityUnavailable {
		t.Errorf("quality = %q, want %q", scene.Quality, QualityUnavailable)
	}
	if scene.Measured() || scene.SceneID != "" {
		t.Errorf("unavailable result carries a figure: %+v", scene)
	}
	if worker.count("/v1/stats") != 0 {
		t.Error("there is no granule, so nothing should have been measured")
	}
	note := strings.Join(scene.Notes, " ")
	if !strings.Contains(note, "no granule in") || !strings.Contains(note, "timeout") {
		t.Errorf("notes %q do not record why the window was empty", scene.Notes)
	}
}

func TestAFailedWorkerIsAFaultNotAnEmptyPeriod(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"busy", `{"error":"busy","detail":"2 job(s) already running","notes":[]}`, http.StatusServiceUnavailable},
		{"store down", `{"error":"store_unavailable","detail":"volume not mounted","notes":[]}`, http.StatusServiceUnavailable},
		{"band gone", `{"error":"band_unavailable","detail":"B04.tif 404","notes":[]}`, http.StatusBadGateway},
		{"wrong token", `{"error":"unauthorized","detail":"missing or wrong bearer token","notes":[]}`, http.StatusUnauthorized},
		{"not json at all", `<html>502 Bad Gateway</html>`, http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := newFakeWorker(t, map[string]any{"/v1/scene": test.body})
			worker.status["/v1/scene"] = test.status

			scene, err := worker.provider().LatestObservation(context.Background(), fixtureAOI(), 45*24*time.Hour)
			if err == nil {
				t.Fatalf("an outage must surface as an error, got %+v", scene)
			}
			if scene.Measured() || scene.Quality == QualityComplete {
				t.Errorf("a fault produced a publishable figure: %+v", scene)
			}
		})
	}
}

func TestUnreachableWorkerIsAnError(t *testing.T) {
	worker := newFakeWorker(t, map[string]any{})
	address := worker.server.URL
	worker.server.Close()

	p := NewOpenDataStac(address, fixtureToken)
	if _, err := p.LatestObservation(context.Background(), fixtureAOI(), 45*24*time.Hour); err == nil {
		t.Fatal("a closed worker must return an error")
	}
}

func TestUnconfiguredAndCelllessAOIsNeverReachOut(t *testing.T) {
	worker := newFakeWorker(t, map[string]any{})
	for _, test := range []struct {
		name    string
		worker  *OpenDataStacProvider
		aoi     AOI
		needles string
	}{
		{"no url", NewOpenDataStac("", fixtureToken), fixtureAOI(), "S2_WORKER_URL"},
		{"no token", NewOpenDataStac(worker.server.URL, ""), fixtureAOI(), "S2_WORKER_TOKEN"},
		{"no plots", worker.provider(), AOI{ProjectID: fixtureProject, AnalysisGeom: json.RawMessage(cellPolygon)},
			"no analysis cells"},
		{"no geometry", worker.provider(), AOI{ProjectID: fixtureProject,
			Cells: []Cell{{PlotID: fixtureCellID, Geometry: json.RawMessage(cellPolygon)}}}, "no analysis cells"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The handler errors on any call, so a request that escaped would fail loudly.
			scene, err := test.worker.LatestObservation(context.Background(), test.aoi, 45*24*time.Hour)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if scene.Quality != QualityUnavailable || scene.Measured() {
				t.Errorf("got %+v, want an unavailable result", scene)
			}
			if !strings.Contains(strings.Join(scene.Notes, " "), test.needles) {
				t.Errorf("notes %q do not name %q", scene.Notes, test.needles)
			}
		})
	}
	if len(worker.calls) != 0 {
		t.Errorf("the seam made %d request(s) with nothing configured or nothing to measure", len(worker.calls))
	}
}

func TestTileSetIsTheStoreAddressTheWorkerPainted(t *testing.T) {
	recorded := fixture(t, "render.json")
	worker := newFakeWorker(t, map[string]any{"/v1/tiles/render": recorded})
	scene := sceneFromFixture(t)

	tiles, err := worker.provider().TileSet(context.Background(), fixtureAOI(), scene)
	if err != nil {
		t.Fatalf("%v", err)
	}
	prefix := fixtureProject + "/" + fixtureSceneID
	if tiles.KeyPrefix != prefix {
		t.Errorf("prefix = %q, want %q", tiles.KeyPrefix, prefix)
	}
	if tiles.SceneID != fixtureSceneID || tiles.PaletteVersion != "ndvi-5band/1" {
		t.Errorf("tile set = %+v", tiles)
	}
	if !tiles.Valid() || tiles.ZoomMin > tiles.ZoomMax {
		t.Errorf("tile set = %+v, want a usable address", tiles)
	}

	call := worker.call("/v1/tiles/render")
	body := call.body
	// The worker validates one project and one scene; a prefix with a third segment or
	// a dot-dot in it is a path this seam must never build.
	if parts := strings.Split(body["key_prefix"].(string), "/"); len(parts) != 2 {
		t.Errorf("key_prefix %v is not one project and one scene", body["key_prefix"])
	}
	if body["palette_version"] != "ndvi-5band/1" {
		t.Errorf("palette_version = %v, want the version the scene was measured with", body["palette_version"])
	}
	if bbox, _ := body["bbox"].([]any); len(bbox) != 4 {
		t.Errorf("bbox = %v, want the framing box", body["bbox"])
	}
	minZoom, _ := body["zoom_min"].(float64)
	maxZoom, _ := body["zoom_max"].(float64)
	if minZoom < workerZoomMin || maxZoom > workerZoomMax || minZoom > maxZoom {
		t.Errorf("zoom %v-%v outside %d-%d", minZoom, maxZoom, workerZoomMin, workerZoomMax)
	}
	// Never ask the worker for a plan it would truncate into `partial`.
	planned := 0
	for zoom := int(minZoom); zoom <= int(maxZoom); zoom++ {
		planned += tilesForZoom(fixtureAOI().FramingBBox, zoom)
	}
	if planned > workerMaxTilesJob {
		t.Errorf("planned %d tiles, over the %d per-job ceiling", planned, workerMaxTilesJob)
	}
}

func sceneFromFixture(t *testing.T) SceneStats {
	t.Helper()
	body := fixture(t, "scene.json")
	decoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("%v", err)
	}
	var scene SceneStats
	if err := json.Unmarshal(decoded, &scene); err != nil {
		t.Fatalf("%v", err)
	}
	return scene
}

func TestAnIncompleteLayerIsNeverAdvertised(t *testing.T) {
	recorded := fixture(t, "render.json")
	recorded["quality"] = QualityPartial
	recorded["tiles_omitted"] = 12
	worker := newFakeWorker(t, map[string]any{"/v1/tiles/render": recorded})

	tiles, err := worker.provider().TileSet(context.Background(), fixtureAOI(), sceneFromFixture(t))
	if err == nil {
		t.Fatalf("a layer with holes must not be handed out: %+v", tiles)
	}
	if tiles.Valid() {
		t.Errorf("partial render still returned an address: %+v", tiles)
	}
	if !strings.Contains(err.Error(), "partial") || !strings.Contains(err.Error(), "12") {
		t.Errorf("error %q does not say what was left out", err)
	}
}

func TestTileSetRefusesWhatItCannotPaint(t *testing.T) {
	worker := newFakeWorker(t, map[string]any{})
	base := sceneFromFixture(t)
	for _, test := range []struct {
		name   string
		worker *OpenDataStacProvider
		scene  SceneStats
		aoi    AOI
	}{
		{"no worker", NewOpenDataStac("", ""), base, fixtureAOI()},
		{"no granule", worker.provider(), SceneStats{Quality: QualityUnavailable}, fixtureAOI()},
		{"no bands", worker.provider(), SceneStats{SceneID: fixtureSceneID}, fixtureAOI()},
		{"no project", worker.provider(), base, AOI{FramingBBox: fixtureAOI().FramingBBox}},
		{"scene id with a path", worker.provider(),
			SceneStats{SceneID: "../../etc/passwd", Bands: base.Bands, PaletteVersion: base.PaletteVersion}, fixtureAOI()},
		{"project id with a path", worker.provider(), base,
			AOI{ProjectID: "project/one", FramingBBox: fixtureAOI().FramingBBox, Cells: fixtureAOI().Cells}},
	} {
		t.Run(test.name, func(t *testing.T) {
			tiles, err := test.worker.TileSet(context.Background(), test.aoi, test.scene)
			if err == nil {
				t.Fatalf("expected a refusal, got %+v", tiles)
			}
			if tiles.Valid() {
				t.Errorf("refusal still returned an address: %+v", tiles)
			}
		})
	}
	if len(worker.calls) != 0 {
		t.Errorf("refusals made %d request(s)", len(worker.calls))
	}
}

func TestSandboxProviderIsAlwaysLabelledAndNeverPaints(t *testing.T) {
	provider := NewSandboxProvider()
	if provider.Name() != ProviderSandbox {
		t.Errorf("name = %q", provider.Name())
	}
	aoi := fixtureAOI()
	aoi.PlotCount = 3

	baseline, err := provider.BaselineObservation(context.Background(), aoi, 45*24*time.Hour)
	if err != nil {
		t.Fatalf("%v", err)
	}
	latest, err := provider.LatestObservation(context.Background(), aoi, 45*24*time.Hour)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, scene := range []SceneStats{baseline, latest} {
		if scene.AdapterMode != ModeSandbox || scene.ProviderName != ProviderSandbox {
			t.Errorf("sandbox result labelled %s/%s", scene.AdapterMode, scene.ProviderName)
		}
		if scene.Quality != QualitySynthetic {
			t.Errorf("quality = %q, want %q", scene.Quality, QualitySynthetic)
		}
		if scene.SceneID != "" || len(scene.Bands) != 0 {
			t.Errorf("sandbox invented a granule: %q with %d band(s)", scene.SceneID, len(scene.Bands))
		}
		if !scene.AcquiredAt.IsZero() {
			t.Errorf("sandbox claims an acquisition at %s", scene.AcquiredAt)
		}
		if scene.ProcessingVersion != ProcessingVersion {
			t.Errorf("processing version = %q, want the unchanged sandbox constant %q",
				scene.ProcessingVersion, ProcessingVersion)
		}
		if !strings.Contains(strings.Join(scene.Notes, " "), "Not satellite imagery") {
			t.Errorf("notes %q do not say the figure is modelled", scene.Notes)
		}
	}
	step := number(t, latest.MeanNDVI, "latest mean") - number(t, baseline.MeanNDVI, "baseline mean")
	if got := round4(step); got != sandboxGrowthStep {
		t.Errorf("sandbox change = %v, want the modelled %v", got, sandboxGrowthStep)
	}
	if latest.PlotCount != 3 {
		t.Errorf("plot count = %d, want the AOI's 3", latest.PlotCount)
	}

	tiles, err := provider.TileSet(context.Background(), aoi, latest)
	if err != nil {
		t.Fatalf("having no tiles is not a fault: %v", err)
	}
	if tiles.Valid() {
		t.Errorf("sandbox returned a tile address: %+v", tiles)
	}
}

func TestServicePicksTheProviderAndDefaultsToSandbox(t *testing.T) {
	for _, test := range []struct {
		provider, want string
	}{
		{"", ProviderSandbox},
		{"sandbox", ProviderSandbox},
		{"opendata-s2", ProviderOpenData},
		{"planetscope", ProviderSandbox}, // a name this build does not know
	} {
		service := NewService(nil, Options{Mode: "live", Provider: test.provider})
		if got := service.ProviderName(); got != test.want {
			t.Errorf("SATELLITE_PROVIDER %q resolved to %q, want %q", test.provider, got, test.want)
		}
		if got := service.Mode(); got != "live" {
			t.Errorf("mode = %q, want the configured value untouched", got)
		}
	}
}

func TestZoomPlanStaysInsideOneJob(t *testing.T) {
	for _, test := range []struct {
		name string
		bbox []float64
	}{
		{"one 158 m cell", fixtureAOI().FramingBBox},
		{"a 5 km reach", []float64{39.80, -3.66, 39.85, -3.62}},
		{"a scattered 40 plot project", []float64{39.60, -3.75, 40.05, -3.50}},
		{"a whole district", []float64{38.90, -4.30, 40.20, -3.40}},
	} {
		minZoom, maxZoom := zoomPlan(test.bbox)
		if minZoom < workerZoomMin || maxZoom > workerZoomMax || minZoom > maxZoom {
			t.Errorf("%s: plan %d-%d outside %d-%d", test.name, minZoom, maxZoom, workerZoomMin, workerZoomMax)
		}
		if planned := sumTiles(test.bbox, minZoom, maxZoom); planned > workerMaxTilesJob {
			t.Errorf("%s: plan asks for %d tiles, over the %d ceiling", test.name, planned, workerMaxTilesJob)
		}
		if maxZoom != workerZoomMax && tilesForZoom(test.bbox, maxZoom) > workerMaxTilesJob {
			t.Errorf("%s: zoom %d alone needs %d tiles", test.name, maxZoom, tilesForZoom(test.bbox, maxZoom))
		}
	}
	if minZoom, maxZoom := zoomPlan(fixtureAOI().FramingBBox); minZoom != workerZoomMax-zoomSpan+1 ||
		maxZoom != workerZoomMax {
		t.Errorf("a single cell plans %d-%d, want the sharpest %d levels down to %d",
			minZoom, maxZoom, zoomSpan, workerZoomMax-zoomSpan+1)
	}
}

func TestWindowDaysStaysInsideWhatTheWorkerAccepts(t *testing.T) {
	for _, test := range []struct {
		window time.Duration
		want   int
	}{
		{0, 1},
		{24 * time.Hour, 1},
		{45 * 24 * time.Hour, 45},
		{90 * 24 * time.Hour, 90},
		{400 * 24 * time.Hour, workerMaxWindow},
	} {
		if got := windowDays(test.window); got != test.want {
			t.Errorf("window %v gave %d days, want %d", test.window, got, test.want)
		}
	}
}
