package satellite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DefaultWindow is the current-observation window from spec section 5.2. It stays a
// documented convention until methodologies carry seasonality.
const DefaultWindow = 45 * 24 * time.Hour

// minSummaryCoveragePct is spec section 5.1 rule 2. It is enforced here rather than in
// the worker because /v1/stats reports the quality of the read it performed, and only
// the caller knows whether the figure that read produced is fit to publish.
const minSummaryCoveragePct = 60.0

// ErrSummaryMissing is what a project answers before its first computation. It is a
// refusal rather than an empty success so the client offers a refresh instead of
// drawing a state nobody measured.
var ErrSummaryMissing = errors.New("no NDVI summary has been computed for this project yet")

// ErrProviderFailed is a fault in the imagery path, which is not the same thing as an
// absence of imagery. An outage must never be stored as a period with no growth, so a
// computation that hits one writes nothing and the refresh job retries it.
var ErrProviderFailed = errors.New("the imagery provider could not complete this computation")

// summaryPayload is the canonical commitment. Both observations are carried verbatim,
// so a verifier can re-derive the mean from the same granules, cells and mask and then
// check the project arithmetic on top.
//
// The clock is deliberately absent: no computed_at, and no period_start or period_end,
// because the scenes already carry the search window that produced them. The hash is a
// function of what was measured, so re-reading the same two scenes tomorrow cannot mint
// a second commitment for the same number.
type summaryPayload struct {
	ProjectID        string          `json:"project_id"`
	WindowDays       int             `json:"window_days"`
	PlotCount        int             `json:"plot_count"`
	AnalysisGeometry json.RawMessage `json:"analysis_geometry"`
	FramingBBox      []float64       `json:"framing_bbox"`
	WeightByArea     bool            `json:"weight_by_area"`

	Current  SceneStats  `json:"current"`
	Baseline *SceneStats `json:"baseline"`

	MeanNDVI           *float64 `json:"mean_ndvi"`
	BaselineMeanNDVI   *float64 `json:"baseline_mean_ndvi"`
	ChangeAbs          *float64 `json:"change_abs"`
	ChangePct          *float64 `json:"change_pct"`
	CoveragePct        float64  `json:"coverage_pct"`
	Quality            string   `json:"quality"`
	AdapterMode        string   `json:"adapter_mode"`
	Provider           string   `json:"provider"`
	MethodologyVersion string   `json:"methodology_version"`
	ProcessingVersion  string   `json:"processing_version"`
	ClassTable         string   `json:"class_table"`
	PaletteVersion     string   `json:"palette_version"`
	Notes              []string `json:"notes"`
}

// ComputeProjectSummary reads one project observation and the same window a year
// earlier, then appends the result. Rows are never updated: a figure that was anchored
// keeps the value it was anchored with, and a correction is a new row with a new hash.
//
// Every label on the row comes from the provider that produced the figure, so no
// configuration can present a modelled number as imagery.
func (s *Service) ComputeProjectSummary(ctx context.Context, projectID string, window time.Duration) (models.ProjectNdviSummary, error) {
	if window <= 0 {
		window = DefaultWindow
	}
	methodology, err := s.projectMethodology(projectID)
	if err != nil {
		return models.ProjectNdviSummary{}, err
	}
	aoi, err := s.ProjectAOI(projectID)
	if err != nil {
		return models.ProjectNdviSummary{}, err
	}

	current, err := s.provider.LatestObservation(ctx, aoi, window)
	if err != nil {
		return models.ProjectNdviSummary{}, s.providerFault(projectID, "current observation", err)
	}
	// A baseline is only worth a catalog round trip when there is a figure to compare
	// it against. Without one the current observation's own note already says why the
	// period is empty.
	var baseline *SceneStats
	if current.Measured() {
		observation, err := s.provider.BaselineObservation(ctx, aoi, window)
		if err != nil {
			return models.ProjectNdviSummary{}, s.providerFault(projectID, "baseline observation", err)
		}
		baseline = &observation
	}

	payload := summaryPayload{
		ProjectID:          projectID,
		WindowDays:         windowDays(window),
		PlotCount:          aoi.PlotCount,
		AnalysisGeometry:   aoi.AnalysisGeom,
		FramingBBox:        aoi.FramingBBox,
		WeightByArea:       true,
		Current:            current,
		Baseline:           baseline,
		MeanNDVI:           current.MeanNDVI,
		CoveragePct:        round2(current.CoveragePct),
		Quality:            current.Quality,
		AdapterMode:        current.AdapterMode,
		Provider:           current.ProviderName,
		MethodologyVersion: methodology,
		ProcessingVersion:  current.ProcessingVersion,
		ClassTable:         current.ClassTable,
		PaletteVersion:     current.PaletteVersion,
		Notes:              append([]string{}, current.Notes...),
	}
	if baseline != nil {
		payload.BaselineMeanNDVI = baseline.MeanNDVI
		payload.Notes = append(payload.Notes, baseline.Notes...)
		switch {
		case current.MeanNDVI == nil || baseline.MeanNDVI == nil:
			// The current mean stands on its own. Worsening the row to the baseline's
			// quality here would hide a figure that was genuinely measured.
			payload.Notes = append(payload.Notes, "No baseline figure for the earlier window, so no change is published.")
		default:
			payload.Quality = worseQuality(payload.Quality, baseline.Quality)
			absolute := round4(*current.MeanNDVI - *baseline.MeanNDVI)
			payload.ChangeAbs = &absolute
			// A zero baseline has no percentage to speak of, and dividing by it would
			// publish an infinity as a growth rate.
			if *baseline.MeanNDVI != 0 {
				percent := round2((*current.MeanNDVI - *baseline.MeanNDVI) / math.Abs(*baseline.MeanNDVI) * 100)
				payload.ChangePct = &percent
			}
		}
	}
	if current.Measured() && current.CoveragePct < minSummaryCoveragePct {
		payload.Quality = worseQuality(payload.Quality, QualityPartial)
		payload.Notes = append(payload.Notes, fmt.Sprintf(
			"Coverage %.1f%% of expected pixels is below the %.0f%% floor, so this figure is partial.",
			current.CoveragePct, minSummaryCoveragePct))
	}

	hash, canonical, err := canon.Hash(payload)
	if err != nil {
		return models.ProjectNdviSummary{}, fmt.Errorf("hash ndvi summary: %w", err)
	}
	now := s.now().UTC()
	summary := models.ProjectNdviSummary{
		ID: uuid.NewString(), ProjectID: projectID,
		PeriodStart: now.Add(-window), PeriodEnd: now,
		CurrentSceneID: current.SceneID, CurrentAcquiredAt: acquiredAt(current),
		MeanNDVI: payload.MeanNDVI, BaselineMeanNDVI: payload.BaselineMeanNDVI,
		ChangeAbs: payload.ChangeAbs, ChangePct: payload.ChangePct,
		CoveragePct: payload.CoveragePct, CurrentCloudPct: current.CloudPct,
		PlotCount: payload.PlotCount, Quality: payload.Quality, AdapterMode: payload.AdapterMode,
		Provider: payload.Provider, MethodologyVersion: methodology,
		Result: canonical, ResultHash: hash, ProcessingVersion: payload.ProcessingVersion,
		ComputedAt: now, CreatedAt: now,
	}
	if baseline != nil {
		summary.BaselineSceneID = baseline.SceneID
		summary.BaselineAcquiredAt = acquiredAt(*baseline)
	}
	if err := s.db.Create(&summary).Error; err != nil {
		return models.ProjectNdviSummary{}, fmt.Errorf("store ndvi summary: %w", err)
	}
	return summary, nil
}

// providerFault logs the detail and returns the stable client-facing fault. The worker
// and the catalogs say things in their own words, including addresses, and none of that
// belongs in a response body or in a job's error column.
func (s *Service) providerFault(projectID, step string, err error) error {
	slog.Error("satellite computation failed", "project", projectID, "provider", s.provider.Name(),
		"step", step, "error", err)
	return fmt.Errorf("%s: %w", step, ErrProviderFailed)
}

// projectMethodology names the methodology the figure was computed under. A missing
// project is refused here rather than left to the foreign key, because the SQLite test
// driver does not enforce one and an orphan summary would be unattributable.
func (s *Service) projectMethodology(projectID string) (string, error) {
	var project models.ConservationProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return "", fmt.Errorf("project %s: %w", projectID, err)
	}
	var methodology models.Methodology
	if err := s.db.Where("id = ?", project.MethodologyID).First(&methodology).Error; err != nil {
		return "", fmt.Errorf("methodology for project %s: %w", projectID, err)
	}
	return fmt.Sprintf("%s v%s", methodology.Code, methodology.Version), nil
}

// acquiredAt converts the granule's own timestamp, leaving it nil when nothing was
// acquired. A zero time is a date nobody measured, so it must not reach a column.
func acquiredAt(scene SceneStats) *time.Time {
	if scene.AcquiredAt.IsZero() {
		return nil
	}
	value := scene.AcquiredAt.UTC()
	return &value
}

// LatestSummary is the project's active computation: the newest period, and the newest
// run within it. The tie-break on id keeps the answer stable when two runs share a
// timestamp.
func (s *Service) LatestSummary(projectID string) (models.ProjectNdviSummary, bool, error) {
	var summary models.ProjectNdviSummary
	err := s.db.Where("project_id = ?", projectID).
		Order("period_end desc").Order("computed_at desc").Order("id desc").
		First(&summary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ProjectNdviSummary{}, false, nil
	}
	if err != nil {
		return models.ProjectNdviSummary{}, false, err
	}
	return summary, true, nil
}

// ProjectChange is the read side of the same figure, for the change endpoint and the
// dashboard headline.
func (s *Service) ProjectChange(projectID string) (ChangeView, error) {
	summary, ok, err := s.LatestSummary(projectID)
	if err != nil {
		return ChangeView{}, err
	}
	if !ok {
		return ChangeView{}, ErrSummaryMissing
	}
	return ChangeViewOf(summary), nil
}

// RenderActiveScene asks the worker to paint the active summary's scene into the
// shared tile store, then stamps the zoom range onto the summary row. It is
// idempotent: a row that already carries a zoom range is left alone, because a
// second render would spend worker ceiling on tiles that already exist.
//
// A render failure does not poison the summary. The row keeps meaning what it
// said, and the overlay simply omits tileBaseUrl until the next refresh paints
// it. That split is deliberate: a number the worker measured is still a number
// worth presenting, even before pixels were rendered for it.
func (s *Service) RenderActiveScene(ctx context.Context, projectID string) error {
	summary, ok, err := s.LatestSummary(projectID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrSummaryMissing
	}
	if summary.TileMinZoom != nil && summary.TileMaxZoom != nil {
		return nil
	}
	if summary.CurrentSceneID == "" || summary.AdapterMode != ModeLive {
		// Sandbox and unavailable rows have nothing to paint. The zoom range
		// stays nil so the proxy answers 410 and the overlay omits tileBaseUrl.
		return nil
	}
	aoi, err := s.ProjectAOI(projectID)
	if err != nil {
		return err
	}
	if aoi.Status != "available" {
		return nil
	}
	// The provider needs the bands it recorded on the result payload to ask the
	// worker to paint the same granule again. They are in the hashed payload,
	// not on the summary row, so they come back through a decode.
	var payload struct {
		Current SceneStats `json:"current"`
	}
	if len(summary.Result) > 0 {
		_ = json.Unmarshal(summary.Result, &payload)
	}
	if payload.Current.SceneID == "" || len(payload.Current.Bands) == 0 {
		return nil
	}
	tileSet, err := s.provider.TileSet(ctx, aoi, payload.Current)
	if err != nil {
		slog.Warn("satellite tile render refused", "project", projectID, "error", err)
		return nil
	}
	if !tileSet.Valid() {
		return nil
	}
	updates := map[string]any{"tile_min_zoom": tileSet.ZoomMin, "tile_max_zoom": tileSet.ZoomMax}
	if err := s.db.WithContext(ctx).Model(&models.ProjectNdviSummary{}).Where("id = ?", summary.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("record tile range: %w", err)
	}
	return nil
}

// ChangeView is the client shape from spec section 7.3. Absent figures stay null rather
// than becoming zero, because NDVI 0.0 is a real reading for bare substrate. There is no
// anchor field until phase 5 writes one: a client must never be told a commitment is
// anchored when nothing was submitted.
type ChangeView struct {
	SummaryID         string     `json:"summaryId"`
	MeanNDVI          *float64   `json:"meanNdvi"`
	BaselineMeanNDVI  *float64   `json:"baselineMeanNdvi"`
	ChangeAbs         *float64   `json:"changeAbs"`
	ChangePct         *float64   `json:"changePct"`
	PeriodStart       time.Time  `json:"periodStart"`
	PeriodEnd         time.Time  `json:"periodEnd"`
	CurrentSceneID    string     `json:"currentSceneId"`
	BaselineSceneID   string     `json:"baselineSceneId"`
	CurrentAcquiredAt *time.Time `json:"currentAcquiredAt"`
	CoveragePct       float64    `json:"coveragePct"`
	PlotCount         int        `json:"plotCount"`
	Quality           string     `json:"quality"`
	AdapterMode       string     `json:"adapterMode"`
	Provider          string     `json:"provider"`
	ProcessingVersion string     `json:"processingVersion"`
	ResultHash        string     `json:"resultHash"`
	ComputedAt        time.Time  `json:"computedAt"`
}

func ChangeViewOf(summary models.ProjectNdviSummary) ChangeView {
	return ChangeView{
		SummaryID: summary.ID, MeanNDVI: summary.MeanNDVI, BaselineMeanNDVI: summary.BaselineMeanNDVI,
		ChangeAbs: summary.ChangeAbs, ChangePct: summary.ChangePct,
		PeriodStart: summary.PeriodStart, PeriodEnd: summary.PeriodEnd,
		CurrentSceneID: summary.CurrentSceneID, BaselineSceneID: summary.BaselineSceneID,
		CurrentAcquiredAt: summary.CurrentAcquiredAt,
		CoveragePct:       summary.CoveragePct, PlotCount: summary.PlotCount,
		Quality: summary.Quality, AdapterMode: summary.AdapterMode, Provider: summary.Provider,
		ProcessingVersion: summary.ProcessingVersion, ResultHash: summary.ResultHash,
		ComputedAt: summary.ComputedAt,
	}
}

// OverlayView is what the map needs to request tiles and to label them. The
// envelope matches spec section 7.3 so sandbox and live responses share one
// shape: a deployment without tiles omits tileBaseUrl rather than sending a
// second structure the client has to branch on.
//
// cacheExpiresAt is our own re-check deadline, not a credential or a vendor
// handle. Before it a cached envelope is still correct; after it the client
// may ask again to see whether a newer scene has been ingested.
type OverlayView struct {
	TileBaseURL       string        `json:"tileBaseUrl,omitempty"`
	Layer             string        `json:"layer"`
	SceneID           string        `json:"sceneId,omitempty"`
	Mission           string        `json:"mission"`
	AcquiredAt        *time.Time    `json:"acquiredAt,omitempty"`
	CloudCoverPct     *float64      `json:"cloudCoverPct,omitempty"`
	Quality           string        `json:"quality"`
	AdapterMode       string        `json:"adapterMode"`
	Provider          string        `json:"provider"`
	DiscoverySource   string        `json:"discoverySource,omitempty"`
	ProcessingVersion string        `json:"processingVersion"`
	BBox              *OverlayBBox  `json:"bbox,omitempty"`
	ZoomRange         *ZoomRange    `json:"zoomRange,omitempty"`
	Legend            []LegendStop  `json:"legend"`
	CacheExpiresAt    time.Time     `json:"cacheExpiresAt"`
	SummaryID         string        `json:"summaryId"`
}

// OverlayBBox is the framing bbox the worker was asked to render. The map
// client uses it to fit the viewport to the overlay before any tile arrives.
type OverlayBBox struct {
	MinLat float64 `json:"minLat"`
	MinLon float64 `json:"minLon"`
	MaxLat float64 `json:"maxLat"`
	MaxLon float64 `json:"maxLon"`
}

// ZoomRange is the pair of slippy-map zooms the worker painted. A request
// outside this range is answered 410 so the client re-requests overlay rather
// than painting a gap.
type ZoomRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// LegendStop is one colour band from spec section 5.4. The labelKey is an
// Android string resource id; the client translates rather than displaying
// the English label.
type LegendStop struct {
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Color    string  `json:"color"`
	LabelKey string  `json:"labelKey"`
}

// ndviLegend is the palette from spec section 5.4. It lives here rather than
// in the Android string table so the API and the device cannot disagree on
// the stop boundaries. The labelKey points at the translation.
var ndviLegend = []LegendStop{
	{Min: -1.0, Max: 0.10, Color: "#B7A57A", LabelKey: "satellite.legend.bare"},
	{Min: 0.10, Max: 0.30, Color: "#E8E28A", LabelKey: "satellite.legend.verylow"},
	{Min: 0.30, Max: 0.45, Color: "#B8DB7A", LabelKey: "satellite.legend.low"},
	{Min: 0.45, Max: 0.60, Color: "#6FBE44", LabelKey: "satellite.legend.moderate"},
	{Min: 0.60, Max: 1.0, Color: "#2A7D1E", LabelKey: "satellite.legend.dense"},
}

// overlayCacheTTL is how long a cached overlay envelope stays correct. It is
// the same order as the refresh rate limit: a client that re-asks sooner than
// this would only see a 409 from the refresh endpoint anyway.
const overlayCacheTTL = 15 * time.Minute

// ProjectOverlay is the read side of the overlay endpoint. A project without a
// summary answers ErrSummaryMissing, which the handler maps to 404. A sandbox
// or unavailable summary answers the same envelope with no tileBaseUrl and no
// scene id, so the client draws its no-imagery state rather than branching on
// a different shape.
func (s *Service) ProjectOverlay(projectID string) (OverlayView, error) {
	summary, ok, err := s.LatestSummary(projectID)
	if err != nil {
		return OverlayView{}, err
	}
	if !ok {
		return OverlayView{}, ErrSummaryMissing
	}
	view := OverlayView{
		Layer:             "ndvi",
		Mission:           "S2",
		AcquiredAt:        summary.CurrentAcquiredAt,
		Quality:           summary.Quality,
		AdapterMode:       summary.AdapterMode,
		Provider:          summary.Provider,
		ProcessingVersion: summary.ProcessingVersion,
		Legend:            ndviLegend,
		CacheExpiresAt:    s.now().UTC().Add(overlayCacheTTL),
		SummaryID:         summary.ID,
	}
	if summary.CurrentCloudPct != nil {
		cloud := *summary.CurrentCloudPct
		view.CloudCoverPct = &cloud
	}
	if len(summary.Result) > 0 {
		var payload struct {
			Current struct {
				DiscoverySource string    `json:"discovery_source"`
				FramingBBox     []float64 `json:"-"`
			} `json:"current"`
			FramingBBox []float64 `json:"framing_bbox"`
		}
		if json.Unmarshal(summary.Result, &payload) == nil {
			view.DiscoverySource = payload.Current.DiscoverySource
			if len(payload.FramingBBox) == 4 {
				view.BBox = &OverlayBBox{
					MinLon: payload.FramingBBox[0], MinLat: payload.FramingBBox[1],
					MaxLon: payload.FramingBBox[2], MaxLat: payload.FramingBBox[3],
				}
			}
		}
	}
	if summary.AdapterMode == ModeLive && summary.CurrentSceneID != "" {
		view.SceneID = summary.CurrentSceneID
		if s.tiles.Available() {
			view.TileBaseURL = fmt.Sprintf("/api/v1/projects/%s/satellite/tiles", summary.ProjectID)
		}
		if summary.TileMinZoom != nil && summary.TileMaxZoom != nil {
			view.ZoomRange = &ZoomRange{Min: *summary.TileMinZoom, Max: *summary.TileMaxZoom}
		}
	}
	return view, nil
}

// Tile is one rendered PNG. The proxy sets Cache-Control and ETag from the
// bytes, so the bytes are returned alongside the digest.
type Tile struct {
	PNG  []byte
	ETag string
}

// ReadTile returns the PNG for one address. A missing tile is ErrTileNotFound,
// which the proxy maps to 410; a scene mismatch or a zoom outside the summary's
// range is ErrSceneSuperseded, which the proxy also maps to 410 but with a
// code the client can distinguish.
func (s *Service) ReadTile(projectID, scene string, z, x, y int) (Tile, error) {
	summary, ok, err := s.LatestSummary(projectID)
	if err != nil {
		return Tile{}, err
	}
	if !ok {
		return Tile{}, ErrSummaryMissing
	}
	if summary.AdapterMode != ModeLive || summary.CurrentSceneID == "" {
		// Sandbox and unavailable rows never painted anything; a client asking
		// for a tile they cannot have is asking about a scene this project no
		// longer shows.
		return Tile{}, ErrSceneSuperseded
	}
	if scene != summary.CurrentSceneID {
		return Tile{}, ErrSceneSuperseded
	}
	if summary.TileMinZoom == nil || summary.TileMaxZoom == nil ||
		z < *summary.TileMinZoom || z > *summary.TileMaxZoom {
		return Tile{}, ErrSceneSuperseded
	}
	keyPrefix := projectID + "/" + scene
	data, err := s.tiles.Get(keyPrefix, z, x, y)
	if err != nil {
		return Tile{}, err
	}
	return Tile{PNG: data, ETag: tileETag(data)}, nil
}

// ErrSceneSuperseded is the typed answer to a tile request that names a scene
// the project has moved past, or a zoom the active render never painted. The
// client re-requests the overlay to discover what is currently on offer.
var ErrSceneSuperseded = errors.New("satellite scene has been superseded")

// tileETag is a short digest of the PNG bytes. It lets a downstream cache or
// the device itself short-circuit a repeat fetch without re-transferring the
// image.
func tileETag(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}
