package satellite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// OpenDataStacProvider talks to s2-worker over internal HTTP. It is the only
// component that knows the worker's route shapes, so a contract change lands in this
// file plus the recorded fixtures in testdata, and nowhere else.
//
// Two rules from SATELLITE_NDVI_SPEC.md section 8 hold throughout: the worker keeps no
// project state, so every call re-sends the scene reference the last call returned;
// and a failure degrades the result instead of inventing one.
type OpenDataStacProvider struct {
	baseURL string
	token   string
	client  *http.Client
	now     func() time.Time
}

// workerTimeout is the client ceiling. A worker job may queue for 30 s behind the
// concurrency cap and then spend 120 s inside its own wall-clock guard, so a shorter
// client timeout would abandon work that was always going to land.
const workerTimeout = (30 + 120 + 30) * time.Second

// The worker's own limits, mirrored so a request is planned rather than refused.
// Exceeding them still degrades honestly: the worker answers 400 for a bad zoom pair
// and returns quality "partial" for tiles it could not finish.
const (
	workerZoomMin     = 8
	workerZoomMax     = 16
	workerMaxTilesJob = 64
	workerMaxWindow   = 365
)

// baselineLookbackYears is the offset in section 5.2: the baseline is the same calendar
// window one year earlier, so a mangrove stand is compared against its own season.
const baselineLookbackYears = -1

func NewOpenDataStac(baseURL, token string) *OpenDataStacProvider {
	return &OpenDataStacProvider{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client:  &http.Client{Timeout: workerTimeout},
		now:     time.Now,
	}
}

func (p *OpenDataStacProvider) Name() string { return ProviderOpenData }

// configured is false when the deployment asked for this provider but gave it no
// address or no token. The answer is then an honest unavailable observation, not a
// request to a host nobody named.
func (p *OpenDataStacProvider) configured() bool { return p.baseURL != "" && p.token != "" }

func (p *OpenDataStacProvider) LatestObservation(ctx context.Context, aoi AOI, window time.Duration) (SceneStats, error) {
	return p.observe(ctx, aoi, window, p.now().UTC())
}

func (p *OpenDataStacProvider) BaselineObservation(ctx context.Context, aoi AOI, window time.Duration) (SceneStats, error) {
	return p.observe(ctx, aoi, window, p.now().UTC().AddDate(baselineLookbackYears, 0, 0))
}

// observe runs the two calls one figure needs: choose the granule, which includes the
// coverage screen because that screen is a pixel read, then measure the cells on it.
//
// Only a typed `unavailable` from the worker degrades into an unavailable result, and
// that is the honest reading: the catalogs answered and nothing usable was there. Every
// other failure is returned as an error, because an outage is not a period with no
// imagery. Either way no figure is produced, so a caller cannot anchor one.
func (p *OpenDataStacProvider) observe(ctx context.Context, aoi AOI, window time.Duration, end time.Time) (SceneStats, error) {
	if !p.configured() {
		return p.unavailable(aoi, "s2-worker is not configured: set S2_WORKER_URL and S2_WORKER_TOKEN"), nil
	}
	if len(aoi.Cells) == 0 || len(aoi.AnalysisGeom) == 0 {
		return p.unavailable(aoi, "no analysis cells for this AOI, so there is nothing to measure"), nil
	}

	var scene SceneStats
	body := sceneRequest{Geometry: aoi.AnalysisGeom, WindowDays: windowDays(window), EndTime: end.Format(time.RFC3339)}
	if err := p.post(ctx, http.MethodPost, "/v1/scene", body, &scene); err != nil {
		if degraded, ok := p.degrade(err, aoi, "scene search"); ok {
			return degraded, nil
		}
		return SceneStats{}, fmt.Errorf("choose scene: %w", err)
	}

	var stats SceneStats
	if err := p.post(ctx, http.MethodPost, "/v1/stats",
		statsRequest{Cells: aoi.Cells, Scene: scene.reference(), WeightByArea: true}, &stats); err != nil {
		if degraded, ok := p.degrade(err, aoi, "cell measurement"); ok {
			return degraded, nil
		}
		return SceneStats{}, fmt.Errorf("measure cells: %w", err)
	}

	result := stats.adopt(scene)
	result.PlotCount = len(aoi.Cells)
	result.AdapterMode = ModeLive
	result.ProviderName = p.Name()
	result.ProcessingVersion = LiveProcessingVersion
	return result, nil
}

// TileSet asks the worker to paint this scene into our own store and returns the
// address of what it wrote. The prefix is ours, so the device never sees a bucket or
// a credential.
func (p *OpenDataStacProvider) TileSet(ctx context.Context, aoi AOI, scene SceneStats) (TileSet, error) {
	if !p.configured() {
		return TileSet{}, errors.New("satellite tiles need a configured s2-worker")
	}
	if scene.SceneID == "" || len(scene.Bands) == 0 {
		return TileSet{}, fmt.Errorf("scene %q has no granule to paint", scene.SceneID)
	}
	if len(aoi.FramingBBox) != 4 || aoi.ProjectID == "" {
		return TileSet{}, errors.New("tile render needs a framing bbox and a project id")
	}
	// The identifiers come from a catalog and a client, and both end up in an object
	// key. The worker checks the shape too; refusing here keeps a hostile id out of any
	// path at all rather than explaining it in an error the operator has to read.
	for _, part := range []string{aoi.ProjectID, scene.SceneID} {
		if strings.ContainsAny(part, `/\`) || strings.Contains(part, "..") {
			return TileSet{}, fmt.Errorf("%q cannot name a tile prefix", part)
		}
	}
	minZoom, maxZoom := zoomPlan(aoi.FramingBBox)
	body := renderRequest{
		BBox: aoi.FramingBBox, ZoomMin: minZoom, ZoomMax: maxZoom,
		Scene: scene.reference(), PaletteVersion: scene.PaletteVersion,
		KeyPrefix: aoi.ProjectID + "/" + scene.SceneID,
	}
	var result struct {
		TileSet
		Quality       string   `json:"quality"`
		TilesRendered int      `json:"tiles_rendered"`
		TilesEmpty    int      `json:"tiles_empty"`
		TilesOmitted  int      `json:"tiles_omitted"`
		Notes         []string `json:"notes"`
	}
	if err := p.post(ctx, http.MethodPost, "/v1/tiles/render", body, &result); err != nil {
		return TileSet{}, err
	}
	// A layer with holes in it must not be advertised: a missing tile on a map reads
	// as bare substrate, which is the opposite of what happened.
	if result.Quality != QualityComplete {
		return TileSet{}, fmt.Errorf("tile render for %s came back %s (%d painted, %d empty, %d omitted)",
			result.SceneID, result.Quality, result.TilesRendered, result.TilesEmpty, result.TilesOmitted)
	}
	return result.TileSet, nil
}

// sceneRequest, statsRequest and renderRequest mirror the worker's pydantic models.
// Only the fields the worker reads are sent; an extra key there is ignored, a missing
// required one is a 400.
type sceneRequest struct {
	Geometry   json.RawMessage `json:"aoi_geojson"`
	WindowDays int             `json:"window_days"`
	MaxCloud   *float64        `json:"max_cloud_pct,omitempty"`
	EndTime    string          `json:"end_time,omitempty"`
}

type statsRequest struct {
	Cells        []Cell   `json:"cells"`
	Scene        sceneRef `json:"scene"`
	WeightByArea bool     `json:"weight_by_area"`
}

type renderRequest struct {
	BBox           []float64 `json:"bbox"`
	ZoomMin        int       `json:"zoom_min"`
	ZoomMax        int       `json:"zoom_max"`
	Scene          sceneRef  `json:"scene"`
	PaletteVersion string    `json:"palette_version"`
	KeyPrefix      string    `json:"key_prefix"`
}

// sceneRef is what the worker needs to rebuild a granule from a caller's payload: the
// identifiers plus the three COG hrefs. It is deliberately narrow, so a scene reference
// cannot smuggle anything the worker would go and fetch.
type sceneRef struct {
	SceneID           string            `json:"scene_id"`
	AcquiredAt        time.Time         `json:"acquired_at"`
	CloudPct          *float64          `json:"cloud_pct,omitempty"`
	Platform          string            `json:"platform,omitempty"`
	EPSG              int               `json:"epsg,omitempty"`
	RelativeOrbit     string            `json:"relative_orbit,omitempty"`
	Grid              string            `json:"grid,omitempty"`
	ProcessorBaseline string            `json:"processor_baseline,omitempty"`
	DiscoverySource   string            `json:"discovery_source,omitempty"`
	Footprint         json.RawMessage   `json:"footprint_geojson,omitempty"`
	Bands             map[string]string `json:"bands,omitempty"`
}

func (s SceneStats) reference() sceneRef {
	return sceneRef{
		SceneID: s.SceneID, AcquiredAt: s.AcquiredAt, CloudPct: s.CloudPct, Platform: s.Platform,
		EPSG: s.EPSG, RelativeOrbit: s.RelativeOrbit, Grid: s.Grid,
		ProcessorBaseline: s.ProcessorBaseline, DiscoverySource: s.DiscoverySource,
		Footprint: s.Footprint, Bands: s.Bands,
	}
}

// adopt folds the measurement onto the scene it was measured from. The scene keeps the
// walk that chose it, and the quality of the figure is the worse of the two answers:
// /v1/stats only knows whether pixels were read, while /v1/scene knows the granule was
// adopted on the hazy rung of the cloud ladder.
func (s SceneStats) adopt(scene SceneStats) SceneStats {
	s.SceneID = scene.SceneID
	s.AcquiredAt = scene.AcquiredAt
	s.CloudPct = scene.CloudPct
	s.Platform = scene.Platform
	s.Grid = scene.Grid
	s.RelativeOrbit = scene.RelativeOrbit
	s.ProcessorBaseline = scene.ProcessorBaseline
	s.DiscoverySource = scene.DiscoverySource
	s.EPSG = scene.EPSG
	s.Footprint = scene.Footprint
	s.Bands = scene.Bands
	s.Window = scene.Window
	s.Screened = scene.Screened
	s.ClassTable = scene.ClassTable
	s.PaletteVersion = scene.PaletteVersion
	s.Quality = worseQuality(scene.Quality, s.Quality)
	s.Notes = append(append([]string{}, scene.Notes...), s.Notes...)
	return s
}

// qualityRank orders the vocabulary so one field can carry the worst caveat. The
// second reason, when there are two, stays in Notes.
var qualityRank = map[string]int{
	QualityComplete: 0, QualityCloudy: 1, QualityPartial: 2, QualitySynthetic: 3, QualityUnavailable: 4,
}

func worseQuality(a, b string) string {
	if qualityRank[b] > qualityRank[a] {
		return b
	}
	return a
}

// unavailable builds the degraded result: no mean, no scene, and a note saying which
// step refused. It is the only shape this provider may return without a granule.
func (p *OpenDataStacProvider) unavailable(aoi AOI, note string) SceneStats {
	return SceneStats{
		Quality: QualityUnavailable, Notes: []string{note}, PlotCount: len(aoi.Cells),
		AdapterMode: ModeLive, ProviderName: p.Name(), ProcessingVersion: LiveProcessingVersion,
	}
}

// degrade answers whether an error is the one failure that is a legitimate empty
// result rather than a fault: the worker typed `unavailable`, which section 8 reserves
// for "no granule here, or this one is below the class table we know".
func (p *OpenDataStacProvider) degrade(err error, aoi AOI, step string) (SceneStats, bool) {
	failure := new(workerError)
	if !errors.As(err, &failure) || !failure.NoScene() {
		return SceneStats{}, false
	}
	note := fmt.Sprintf("%s: %s", step, failure.Detail)
	if len(failure.Notes) > 0 {
		note = fmt.Sprintf("%s; %s", note, strings.Join(failure.Notes, "; "))
	}
	return p.unavailable(aoi, note), true
}

// workerError is the worker's typed failure envelope. The code list is closed, so the
// API never parses a message to decide what happened.
type workerError struct {
	Status int
	Code   string
	Detail string
	Notes  []string
}

func (e *workerError) Error() string {
	return fmt.Sprintf("s2-worker %d %s: %s", e.Status, e.Code, e.Detail)
}

// workerUnavailable is the code in the worker's closed error list (spec section 8) for
// "no granule, and nothing about a granule was guessed". It is not QualityUnavailable,
// which is the state a client is shown, even though the two strings match.
const workerUnavailable = "unavailable"

// NoScene is the one code that means "the data is not there" rather than "we could not
// get at it", so it is the one that may become an unavailable observation.
func (e *workerError) NoScene() bool { return e.Code == workerUnavailable }

func (p *OpenDataStacProvider) post(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader = http.NoBody
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s request: %w", path, err)
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Authorization", "Bearer "+p.token)
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("s2-worker unreachable at %s: %w", path, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("read %s response: %w", path, err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		var failure struct {
			Error  string   `json:"error"`
			Detail string   `json:"detail"`
			Notes  []string `json:"notes"`
		}
		if json.Unmarshal(raw, &failure) != nil || failure.Error == "" {
			failure.Error = "internal"
			failure.Detail = strings.TrimSpace(string(raw))
		}
		if len(failure.Detail) > 300 {
			failure.Detail = failure.Detail[:300]
		}
		return &workerError{Status: response.StatusCode, Code: failure.Error, Detail: failure.Detail, Notes: failure.Notes}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}

func windowDays(window time.Duration) int {
	days := int(math.Round(window.Hours() / 24))
	switch {
	case days < 1:
		return 1
	case days > workerMaxWindow:
		return workerMaxWindow
	default:
		return days
	}
}

// zoomSpan is how many levels one render job covers. Three is what a manager can zoom
// out by before the plots are too small to see anyway; more would spend the job
// ceiling on levels that cannot show a plot boundary.
const zoomSpan = 3

// zoomPlan picks the levels to render for one framing bbox. It takes the sharpest zoom
// whose tiles fit a single worker job, then adds the coarser levels beneath it while
// both the span and the total still fit, because a layer that disappears the moment a
// manager zooms out reads as a broken overlay rather than as a resolution limit.
//
// Going down costs little: a level below has a quarter of the tiles of the one above.
// The plan never exceeds workerMaxTilesJob, which is the ceiling the worker counts
// across every level in one call.
func zoomPlan(bbox []float64) (minZoom, maxZoom int) {
	sharpest := workerZoomMax
	for sharpest > workerZoomMin && tilesForZoom(bbox, sharpest) > workerMaxTilesJob {
		sharpest--
	}
	minZoom = sharpest
	for level := sharpest - 1; level >= workerZoomMin && sharpest-level < zoomSpan; level-- {
		if sumTiles(bbox, level, sharpest) > workerMaxTilesJob {
			break
		}
		minZoom = level
	}
	return minZoom, sharpest
}

func sumTiles(bbox []float64, from, to int) int {
	total := 0
	for zoom := from; zoom <= to; zoom++ {
		total += tilesForZoom(bbox, zoom)
	}
	return total
}

// tilesForZoom counts the slippy tiles covering a WGS 84 bbox at a zoom. The formulas
// are the Web Mercator ones every tile client uses, including the worker's own
// `render.tiles_covering`.
func tilesForZoom(bbox []float64, zoom int) int {
	minLon, minLat, maxLon, maxLat := bbox[0], bbox[1], bbox[2], bbox[3]
	scale := math.Pow(2, float64(zoom))
	x0 := tileX(minLon, scale)
	x1 := tileX(maxLon, scale)
	y0 := tileY(maxLat, scale)
	y1 := tileY(minLat, scale)
	return int(x1-x0+1) * int(y1-y0+1)
}

func tileX(lon, scale float64) int64 {
	x := int64(math.Floor((lon + 180.0) / 360.0 * scale))
	return clampTile(x, scale)
}

func tileY(lat, scale float64) int64 {
	rad := lat * math.Pi / 180.0
	y := int64(math.Floor((1.0 - math.Log(math.Tan(rad)+1.0/math.Cos(rad))/math.Pi) / 2.0 * scale))
	return clampTile(y, scale)
}

// clampTile keeps a latitude past 85 degrees, which would overflow the projection,
// inside the tile grid. Plot centroids cannot reach it, so it is a guard rather than a
// feature.
func clampTile(index int64, scale float64) int64 {
	last := int64(scale) - 1
	if index < 0 {
		return 0
	}
	if index > last {
		return last
	}
	return index
}
