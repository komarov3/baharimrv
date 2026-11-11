package satellite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
)

// seedSummary writes one summary row with the fields a proxy read cares about.
// It bypasses ComputeProjectSummary because those tests cover the arithmetic;
// these tests only need the row that arithmetic would have produced, stamped
// with the tile range the worker would have recorded.
func seedSummary(t *testing.T, service *Service, projectID, sceneID, mode string, minZoom, maxZoom *int) models.ProjectNdviSummary {
	t.Helper()
	payload := map[string]any{
		"project_id":      projectID,
		"framing_bbox":    []float64{39.18, -3.71, 39.36, -3.55},
		"current":         map[string]any{"scene_id": sceneID, "discovery_source": "copernicus-stac", "bands": map[string]string{"red": "r.tif", "nir": "n.tif", "scl": "s.tif"}},
		"quality":         QualityComplete,
		"adapter_mode":    mode,
		"provider":        ProviderOpenData,
		"class_table":     "msk-0509/1",
		"palette_version": "ndvi-5band/1",
	}
	_, canonical, err := canon.Hash(payload)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	mean := 0.42
	now := time.Now().UTC()
	summary := models.ProjectNdviSummary{
		ID: uuid.NewString(), ProjectID: projectID,
		PeriodStart: now.Add(-DefaultWindow), PeriodEnd: now,
		CurrentSceneID: sceneID, CurrentAcquiredAt: &now,
		MeanNDVI: &mean, CoveragePct: 88.0, PlotCount: 1,
		Quality: QualityComplete, AdapterMode: mode, Provider: ProviderOpenData,
		MethodologyVersion: "BAHARI-MANGROVE v1", ProcessingVersion: LiveProcessingVersion,
		Result: canonical, ResultHash: "sha256:test",
		TileMinZoom: minZoom, TileMaxZoom: maxZoom,
		ComputedAt: now, CreatedAt: now,
	}
	if err := service.db.Create(&summary).Error; err != nil {
		t.Fatalf("seed summary: %v", err)
	}
	return summary
}

// intPtr is a small helper so the seed call reads left-to-right instead of
// forcing a two-line dance around a local.
func intPtr(v int) *int { return &v }

func TestRemoteTileReaderPreservesOverlaySceneAndCacheContract(t *testing.T) {
	fixture, projectID := summaryFixture(t, nil)
	want := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	var calls atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/tiles/"+projectID+"/S2_REMOTE/14/9012/7234.png" ||
			r.Header.Get("Authorization") != "Bearer worker-test-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(want)
	}))
	defer worker.Close()
	service := NewService(fixture.db, Options{Provider: ProviderOpenData, WorkerURL: worker.URL, WorkerToken: "worker-test-token"})
	seedSummary(t, service, projectID, "S2_REMOTE", ModeLive, intPtr(13), intPtr(15))
	view, err := service.ProjectOverlay(projectID)
	if err != nil || view.TileBaseURL != "/api/v1/projects/"+projectID+"/satellite/tiles" {
		t.Fatalf("remote overlay = %+v, %v", view, err)
	}
	if _, err := service.ReadTile(projectID, "S2_OLD", 14, 9012, 7234); !errors.Is(err, ErrSceneSuperseded) {
		t.Fatalf("stale scene error = %v", err)
	}
	if _, err := service.ReadTile(projectID, "S2_REMOTE", 16, 9012, 7234); !errors.Is(err, ErrSceneSuperseded) {
		t.Fatalf("unrendered zoom error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("refused scene or zoom reached worker")
	}
	for i := 0; i < 2; i++ {
		tile, err := service.ReadTile(projectID, "S2_REMOTE", 14, 9012, 7234)
		if err != nil || string(tile.PNG) != string(want) || tile.ETag != tileETag(want) {
			t.Fatalf("remote tile = %+v, %v", tile, err)
		}
	}
	if calls.Load() != 1 || service.tiles.Stats().Hits != 1 {
		t.Fatal("remote tile cache contract failed")
	}
}

func TestConfiguredRemoteReaderDoesNotAdvertiseUnrenderedTiles(t *testing.T) {
	fixture, projectID := summaryFixture(t, nil)
	service := NewService(fixture.db, Options{Provider: ProviderOpenData, WorkerURL: "http://worker:9000", WorkerToken: "token"})
	seedSummary(t, service, projectID, "S2_NOT_RENDERED", ModeLive, nil, nil)
	view, err := service.ProjectOverlay(projectID)
	if err != nil || view.TileBaseURL != "" || view.ZoomRange != nil {
		t.Fatalf("unrendered overlay advertised tiles: %+v, %v", view, err)
	}
}

func TestOverlaySandboxOmitsTileAddress(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	seedSummary(t, service, projectID, "sandbox-scene", ModeSandbox, nil, nil)

	view, err := service.ProjectOverlay(projectID)
	if err != nil {
		t.Fatalf("overlay: %v", err)
	}
	if view.TileBaseURL != "" {
		t.Errorf("sandbox overlay must not advertise tileBaseUrl, got %q", view.TileBaseURL)
	}
	if view.SceneID != "" {
		t.Errorf("sandbox overlay must not name a scene, got %q", view.SceneID)
	}
	if view.AdapterMode != ModeSandbox {
		t.Errorf("adapter mode = %q, want %q", view.AdapterMode, ModeSandbox)
	}
	if view.Layer != "ndvi" || len(view.Legend) == 0 {
		t.Errorf("overlay shape incomplete: %+v", view)
	}
}

// A live row without a tile range means the worker did not render (or the
// store is absent). The overlay keeps the scene id so the client can still
// label it, but drops tileBaseUrl so no PNG request is ever made.
func TestOverlayLiveWithoutTilesOmitsBaseURL(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	seedSummary(t, service, projectID, "S2_LIVE_SCENE", ModeLive, nil, nil)

	view, err := service.ProjectOverlay(projectID)
	if err != nil {
		t.Fatalf("overlay: %v", err)
	}
	if view.TileBaseURL != "" {
		t.Errorf("live overlay without tiles must omit tileBaseUrl, got %q", view.TileBaseURL)
	}
	if view.SceneID != "S2_LIVE_SCENE" {
		t.Errorf("scene id = %q, want %q", view.SceneID, "S2_LIVE_SCENE")
	}
	if view.ZoomRange != nil {
		t.Errorf("zoom range without tiles = %+v, want nil", view.ZoomRange)
	}
}

// A live row with a tile range advertises the proxy path and the zooms the
// worker painted, so the client can clamp its requests without a round trip.
func TestOverlayLiveWithTilesCarriesAddressAndRange(t *testing.T) {
	root := t.TempDir()
	service, projectID := summaryFixture(t, nil)
	service.tiles = NewTileCache(NewVolumeStore(root), 4)
	seeded := seedSummary(t, service, projectID, "S2_RENDERED", ModeLive, intPtr(13), intPtr(15))

	view, err := service.ProjectOverlay(projectID)
	if err != nil {
		t.Fatalf("overlay: %v", err)
	}
	want := "/api/v1/projects/" + projectID + "/satellite/tiles"
	if view.TileBaseURL != want {
		t.Errorf("tileBaseUrl = %q, want %q", view.TileBaseURL, want)
	}
	if view.SceneID != seeded.CurrentSceneID {
		t.Errorf("scene id = %q, want %q", view.SceneID, seeded.CurrentSceneID)
	}
	if view.ZoomRange == nil || view.ZoomRange.Min != 13 || view.ZoomRange.Max != 15 {
		t.Errorf("zoom range = %+v, want {13,15}", view.ZoomRange)
	}
	if view.BBox == nil || view.BBox.MinLat == 0 {
		t.Errorf("bbox missing: %+v", view.BBox)
	}
}

// A tile read through the cache must answer 410 (the ErrSceneSuperseded
// sentinel) when the requested scene is not the project's active one, so a
// stale deep-link cannot be served pixels from a scene the client was never
// told about.
func TestReadTileRefusesAStaleScene(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	seedSummary(t, service, projectID, "S2_NEW", ModeLive, intPtr(13), intPtr(15))

	_, err := service.ReadTile(projectID, "S2_OLD", 14, 9012, 7234)
	if !errors.Is(err, ErrSceneSuperseded) {
		t.Fatalf("stale scene = %v, want ErrSceneSuperseded", err)
	}
}

// Zoom outside the summary's rendered range is the same refusal: the worker
// never painted that level, so there is no PNG to serve.
func TestReadTileRefusesAnOutOfRangeZoom(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	seeded := seedSummary(t, service, projectID, "S2_ZOOM", ModeLive, intPtr(13), intPtr(15))

	_, err := service.ReadTile(projectID, seeded.CurrentSceneID, 8, 0, 0)
	if !errors.Is(err, ErrSceneSuperseded) {
		t.Fatalf("zoom 8 (below min) = %v, want ErrSceneSuperseded", err)
	}
	_, err = service.ReadTile(projectID, seeded.CurrentSceneID, 16, 0, 0)
	if !errors.Is(err, ErrSceneSuperseded) {
		t.Fatalf("zoom 16 (above max) = %v, want ErrSceneSuperseded", err)
	}
}

// A happy read returns the PNG the worker wrote and an ETag the client can
// use to short-circuit a repeat fetch.
func TestReadTileReturnsThePNGAndAnETag(t *testing.T) {
	root := t.TempDir()
	service, projectID := summaryFixture(t, nil)
	service.tiles = NewTileCache(NewVolumeStore(root), 4)
	seeded := seedSummary(t, service, projectID, "S2_OK", ModeLive, intPtr(13), intPtr(15))

	path := filepath.Join(root, projectID, "S2_OK", "14", "9012", "7234.png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}

	tile, err := service.ReadTile(projectID, seeded.CurrentSceneID, 14, 9012, 7234)
	if err != nil {
		t.Fatalf("read tile: %v", err)
	}
	if len(tile.PNG) != len(png) {
		t.Fatalf("png length = %d, want %d", len(tile.PNG), len(png))
	}
	if tile.ETag == "" {
		t.Fatalf("etag missing")
	}

	// A second read exercises the cache. The inner store's read counter would
	// be the way to assert this; the cache stats are the proxy-visible proof.
	if _, err := service.ReadTile(projectID, seeded.CurrentSceneID, 14, 9012, 7234); err != nil {
		t.Fatalf("repeat read: %v", err)
	}
	stats := service.TileCache().Stats()
	if stats.Hits < 1 {
		t.Errorf("cache did not absorb the repeat read, stats=%+v", stats)
	}
}

// A sandbox row must never serve a tile, even when the caller names the
// sandbox's own scene identifier. The answer is ErrSceneSuperseded so the
// proxy maps it to 410 rather than explaining a sandbox.
func TestReadTileRefusesSandbox(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	seedSummary(t, service, projectID, "sandbox-scene", ModeSandbox, nil, nil)

	_, err := service.ReadTile(projectID, "sandbox-scene", 14, 0, 0)
	if !errors.Is(err, ErrSceneSuperseded) {
		t.Fatalf("sandbox read = %v, want ErrSceneSuperseded", err)
	}
}

// RenderActiveScene is idempotent: a row that already carries a zoom range is
// left alone so a re-run does not burn worker ceiling on tiles that already
// exist.
func TestRenderActiveSceneSkipsWhenAlreadyRendered(t *testing.T) {
	provider := &stubProvider{}
	service, projectID := summaryFixture(t, provider)
	seedSummary(t, service, projectID, "S2_DONE", ModeLive, intPtr(13), intPtr(15))

	if err := service.RenderActiveScene(context.Background(), projectID); err != nil {
		t.Fatalf("render: %v", err)
	}
	if provider.reads != 0 {
		t.Fatalf("render re-ran the worker (%d reads) for a row that already has tiles", provider.reads)
	}
}

// A sandbox row has no scene to paint. Render must not call the worker, and
// the zoom columns must stay nil so the overlay omits tileBaseUrl.
func TestRenderActiveSceneLeavesSandboxAlone(t *testing.T) {
	provider := &stubProvider{}
	service, projectID := summaryFixture(t, provider)
	seedSummary(t, service, projectID, "sandbox-scene", ModeSandbox, nil, nil)

	if err := service.RenderActiveScene(context.Background(), projectID); err != nil {
		t.Fatalf("render: %v", err)
	}
	if provider.reads != 0 {
		t.Fatalf("render called the worker (%d reads) for a sandbox row", provider.reads)
	}
	var reloaded models.ProjectNdviSummary
	if err := service.db.First(&reloaded, "project_id = ?", projectID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.TileMinZoom != nil || reloaded.TileMaxZoom != nil {
		t.Errorf("sandbox row got zoom range %v/%v", reloaded.TileMinZoom, reloaded.TileMaxZoom)
	}
}

// A successful render stamps the zoom range onto the summary row. The payload
// is read back from the hashed result so the worker sees the same bands the
// previous call named.
func TestRenderActiveSceneRecordsZoomRange(t *testing.T) {
	provider := &stubProvider{
		current: SceneStats{
			SceneID: "S2_RENDER", AdapterMode: ModeLive, ProviderName: ProviderOpenData,
			Bands: map[string]string{"red": "r.tif", "nir": "n.tif", "scl": "s.tif"},
		},
	}
	// TileSet on the stub returns the zero value by default. Override it with
	// a wrapper that answers a real zoom range.
	service, projectID := summaryFixture(t, nil)
	service.provider = &renderStub{inner: provider, tileSet: TileSet{
		KeyPrefix: projectID + "/S2_RENDER", ZoomMin: 13, ZoomMax: 15, SceneID: "S2_RENDER",
	}}
	seedSummary(t, service, projectID, "S2_RENDER", ModeLive, nil, nil)

	if err := service.RenderActiveScene(context.Background(), projectID); err != nil {
		t.Fatalf("render: %v", err)
	}
	var reloaded models.ProjectNdviSummary
	if err := service.db.First(&reloaded, "project_id = ?", projectID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.TileMinZoom == nil || reloaded.TileMaxZoom == nil ||
		*reloaded.TileMinZoom != 13 || *reloaded.TileMaxZoom != 15 {
		t.Fatalf("zoom range = %v/%v, want 13/15", reloaded.TileMinZoom, reloaded.TileMaxZoom)
	}
}

// renderStub wraps stubProvider so one test can replace the TileSet answer
// without having to extend the shared stub.
type renderStub struct {
	inner   *stubProvider
	tileSet TileSet
}

func (r *renderStub) Name() string { return r.inner.Name() }

func (r *renderStub) LatestObservation(ctx context.Context, aoi AOI, w time.Duration) (SceneStats, error) {
	return r.inner.LatestObservation(ctx, aoi, w)
}

func (r *renderStub) BaselineObservation(ctx context.Context, aoi AOI, w time.Duration) (SceneStats, error) {
	return r.inner.BaselineObservation(ctx, aoi, w)
}

func (r *renderStub) TileSet(_ context.Context, _ AOI, _ SceneStats) (TileSet, error) {
	return r.tileSet, nil
}
