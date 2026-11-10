// Provider seam: the one place that knows how a project's NDVI figure is produced.
//
// Two implementations exist. SandboxProvider is a labelled model output, so a demo
// deployment can be driven end to end without imagery. OpenDataStacProvider reads
// real pixels through s2-worker. Nothing else in this package, and nothing in the
// API layer, may branch on which is installed: the quality and adapter mode come
// back on the result, and the result is what a verifier later hashes.
//
// The rule that makes the seam safe is that a provider can only degrade. It returns
// an honest `unavailable`, never a number it did not measure, so an outage reads to
// a manager as "no imagery for this period" rather than as a fallow mangrove.
package satellite

import (
	"context"
	"encoding/json"
	"time"
)

// The accepted SATELLITE_PROVIDER values. An unrecognised name falls back to
// sandbox at construction, because sandbox is the only provider that cannot be
// misconfigured into a wrong figure.
const (
	ProviderSandbox  = "sandbox"
	ProviderOpenData = "opendata-s2"
)

// LiveProcessingVersion replaces the sandbox constant for rows computed from real
// granules. The sandbox value stays "sandbox-ndvi/1" so rows already anchored keep
// meaning what they said when they were written.
const LiveProcessingVersion = "ndvi/s2-l2a-opendata/1"

// Quality vocabulary shared by the worker, these providers and the API. Only
// `complete` may be presented as a project figure without a coverage caveat.
const (
	QualityComplete    = "complete"
	QualityPartial     = "partial"
	QualityCloudy      = "cloudy"
	QualitySynthetic   = "synthetic"
	QualityUnavailable = "unavailable"
)

// ModeLive and ModeSandbox are the adapter modes stamped on a result. They come
// from the provider that produced it, never from configuration alone. A sandbox
// result uses this package's existing sandbox label and processing version, so rows
// written before the seam keep meaning what they said when they were anchored.
const (
	ModeLive    = "live"
	ModeSandbox = "sandbox"
)

// Cell is one plot's buffered analysis geometry, as a GeoJSON Polygon. The API owns
// the cell arithmetic; the provider only reads pixels inside what it is handed.
type Cell struct {
	PlotID       string          `json:"plot_id"`
	Geometry     json.RawMessage `json:"geometry"`
	AreaHectares float64         `json:"area_hectares"`
}

// CellStats is the worker's per-plot figure. It is carried on the result so a
// reviewer can see how much of a project mean one large plot contributed.
type CellStats struct {
	PlotID         string   `json:"plot_id"`
	AreaHectares   float64  `json:"area_hectares"`
	MeanNDVI       *float64 `json:"mean_ndvi"`
	MedianNDVI     *float64 `json:"median_ndvi"`
	ValidPixels    int      `json:"valid_pixels"`
	ExpectedPixels int      `json:"expected_pixels"`
	CoveragePct    float64  `json:"coverage_pct"`
}

// ScreenRow is one candidate the worker's coverage screen looked at.
type ScreenRow struct {
	SceneID    string    `json:"scene_id"`
	UsablePct  float64   `json:"usable_pct"`
	CloudPct   *float64  `json:"cloud_pct"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// SearchWindow is the period a scene was chosen from, kept with the result so a
// summary can state which calendar window produced its number.
type SearchWindow struct {
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	Days          int       `json:"days"`
	CloudCeilings []float64 `json:"cloud_ceilings"`
}

// SceneStats is one project observation: the granule, the figures measured on it,
// and the provenance a verifier needs to re-derive them. The JSON tags match the
// worker's response bodies, so a reply decodes straight in and the band references
// can be sent back unchanged.
type SceneStats struct {
	SceneID           string            `json:"scene_id"`
	AcquiredAt        time.Time         `json:"acquired_at"`
	CloudPct          *float64          `json:"cloud_pct"`
	Platform          string            `json:"platform"`
	Grid              string            `json:"grid"`
	RelativeOrbit     string            `json:"relative_orbit"`
	ProcessorBaseline string            `json:"processor_baseline"`
	DiscoverySource   string            `json:"discovery_source"`
	EPSG              int               `json:"epsg"`
	Footprint         json.RawMessage   `json:"footprint_geojson"`
	Quality           string            `json:"quality"`
	ClassTable        string            `json:"class_table"`
	PaletteVersion    string            `json:"palette_version"`
	Notes             []string          `json:"notes"`
	Bands             map[string]string `json:"bands"`

	MeanNDVI       *float64    `json:"mean_ndvi"`
	MedianNDVI     *float64    `json:"median_ndvi"`
	ValidPixels    int         `json:"valid_pixels"`
	ExpectedPixels int         `json:"expected_pixels"`
	CoveragePct    float64     `json:"coverage_pct"`
	PerCell        []CellStats `json:"per_cell"`
	PixelsRead     int         `json:"pixels_read"`

	Window   SearchWindow `json:"window"`
	Screened []ScreenRow  `json:"screened"`

	// Filled by the provider, not by the worker: how this figure must be labelled.
	PlotCount         int    `json:"-"`
	AdapterMode       string `json:"adapter_mode"`
	ProviderName      string `json:"provider"`
	ProcessingVersion string `json:"processing_version"`
}

// TileSet is an address inside our own object store, which is why it has no
// credential and no expiry: a client that holds one can only ever be pointed at the
// scene it was told about.
type TileSet struct {
	KeyPrefix      string `json:"key_prefix"`
	PaletteVersion string `json:"palette_version"`
	ZoomMin        int    `json:"zoom_min"`
	ZoomMax        int    `json:"zoom_max"`
	SceneID        string `json:"scene_id"`
}

// Valid reports whether there is anything to draw. The zero TileSet is the honest
// "no overlay" answer, not a failure, and it is what a sandbox figure always gives.
func (t TileSet) Valid() bool { return t.KeyPrefix != "" && t.SceneID != "" }

// Measured reports whether a project mean exists at all. An unavailable or empty
// scene carries no mean, and the API must not render one.
func (s SceneStats) Measured() bool { return s.MeanNDVI != nil }

type Provider interface {
	Name() string
	LatestObservation(ctx context.Context, aoi AOI, window time.Duration) (SceneStats, error)
	BaselineObservation(ctx context.Context, aoi AOI, window time.Duration) (SceneStats, error)
	TileSet(ctx context.Context, aoi AOI, scene SceneStats) (TileSet, error)
}

// Sandbox figures. There is no imagery behind them, so the only defensible source
// for the number is the same step the per-activity sandbox model uses: a healthy
// stand, plus 0.04. The wording of the note follows SATELLITE_NDVI_SPEC.md
// section 11 rule 2.
const (
	sandboxStandNDVI  = 0.64
	sandboxGrowthStep = 0.04
)

// SandboxProvider answers with model output, always labelled. It never returns a
// tile set, so no screen can paint a fake layer over a real map.
type SandboxProvider struct{}

func NewSandboxProvider() *SandboxProvider { return &SandboxProvider{} }

func (p *SandboxProvider) Name() string { return ProviderSandbox }

func (p *SandboxProvider) LatestObservation(_ context.Context, aoi AOI, _ time.Duration) (SceneStats, error) {
	return p.scene(aoi, sandboxStandNDVI+sandboxGrowthStep), nil
}

func (p *SandboxProvider) BaselineObservation(_ context.Context, aoi AOI, _ time.Duration) (SceneStats, error) {
	return p.scene(aoi, sandboxStandNDVI), nil
}

func (p *SandboxProvider) TileSet(_ context.Context, _ AOI, _ SceneStats) (TileSet, error) {
	return TileSet{}, nil
}

// scene builds one synthetic result. Only the figure the demo arithmetic needs is
// invented, and the two fields that would make it look like a raster read are left
// empty: AcquiredAt, because nothing was acquired, and the class and palette versions,
// because no pixels were masked or painted. A plausible timestamp is the one sandbox
// field a reader could mistake for imagery, so it is the one this function refuses to
// supply.
//
// A project with no cells gets no figure at all. There is nothing to model, and a
// synthetic mean here would describe a restoration that was never registered.
func (p *SandboxProvider) scene(aoi AOI, mean float64) SceneStats {
	if len(aoi.Cells) == 0 {
		return SceneStats{
			Quality: QualityUnavailable, PlotCount: aoi.PlotCount, AdapterMode: ModeSandbox,
			ProviderName: ProviderSandbox, ProcessingVersion: ProcessingVersion,
			Notes: []string{"No plots are registered for this project, so there is no area to model."},
		}
	}
	value := round4(clamp(mean))
	return SceneStats{
		Platform:          "sandbox-model",
		Quality:           QualitySynthetic,
		MeanNDVI:          &value,
		PlotCount:         aoi.PlotCount,
		AdapterMode:       ModeSandbox,
		ProviderName:      ProviderSandbox,
		ProcessingVersion: ProcessingVersion,
		Notes:             []string{"Sandbox model output. Not satellite imagery."},
	}
}
