package satellite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// stubProvider stands in for the seam so the arithmetic and the labelling can be
// checked without a worker. What it returns is what a provider is allowed to return:
// a measured scene, an honest unavailable, or a fault.
type stubProvider struct {
	current     SceneStats
	baseline    SceneStats
	currentErr  error
	baselineErr error
	reads       int
}

func (p *stubProvider) Name() string { return ProviderOpenData }

func (p *stubProvider) LatestObservation(_ context.Context, _ AOI, _ time.Duration) (SceneStats, error) {
	p.reads++
	return p.current, p.currentErr
}

func (p *stubProvider) BaselineObservation(_ context.Context, _ AOI, _ time.Duration) (SceneStats, error) {
	p.reads++
	return p.baseline, p.baselineErr
}

func (p *stubProvider) TileSet(_ context.Context, _ AOI, _ SceneStats) (TileSet, error) {
	return TileSet{}, nil
}

func measuredScene(id string, mean, coverage float64, quality string) SceneStats {
	value := round4(mean)
	cloud := 24.9
	return SceneStats{
		SceneID: id, AcquiredAt: time.Date(2026, 11, 5, 7, 51, 0, 0, time.UTC), CloudPct: &cloud,
		Platform: "Sentinel-2C", Grid: "37MER", RelativeOrbit: "049", ProcessorBaseline: "05.09",
		DiscoverySource: "copernicus-stac", EPSG: 32637, Quality: quality,
		ClassTable: "msk-0509/1", PaletteVersion: "ndvi-5band/1",
		MeanNDVI: &value, CoveragePct: coverage, ValidPixels: 289, ExpectedPixels: 289,
		AdapterMode: ModeLive, ProviderName: ProviderOpenData, ProcessingVersion: LiveProcessingVersion,
	}
}

// summaryFixture seeds one 2.5 ha plot and freezes the clock, so a period is a checked
// value rather than a race with the wall time.
func summaryFixture(t *testing.T, provider Provider) (*Service, string) {
	t.Helper()
	db, projectID, siteID := openAOIDatabase(t)
	now := time.Now().UTC()
	plot := models.Plot{ID: uuid.NewString(), ProjectID: projectID, SiteID: siteID, Code: "P-1",
		Latitude: -3.64, Longitude: 39.836, AreaHectares: "2.500000", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&plot).Error; err != nil {
		t.Fatalf("create plot: %v", err)
	}
	service := NewService(db, Options{})
	service.now = func() time.Time { return fixtureNow }
	if provider != nil {
		service.provider = provider
	}
	return service, projectID
}

func storedSummaries(t *testing.T, service *Service, projectID string) []models.ProjectNdviSummary {
	t.Helper()
	var rows []models.ProjectNdviSummary
	if err := service.db.Where("project_id = ?", projectID).Order("created_at").Find(&rows).Error; err != nil {
		t.Fatalf("read summaries: %v", err)
	}
	return rows
}

func TestSandboxSummaryIsLabelledHashedAndStored(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	summary, err := service.ComputeProjectSummary(context.Background(), projectID, 0)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if summary.Quality != QualitySynthetic || summary.AdapterMode != ModeSandbox {
		t.Errorf("sandbox row is not labelled: quality %q adapter %q", summary.Quality, summary.AdapterMode)
	}
	if summary.Provider != ProviderSandbox || summary.ProcessingVersion != ProcessingVersion {
		t.Errorf("provenance = %q / %q", summary.Provider, summary.ProcessingVersion)
	}
	if summary.MeanNDVI == nil || *summary.MeanNDVI != 0.68 {
		t.Errorf("mean = %v, want the sandbox stand plus one growth step", summary.MeanNDVI)
	}
	if summary.BaselineMeanNDVI == nil || *summary.BaselineMeanNDVI != 0.64 {
		t.Errorf("baseline mean = %v", summary.BaselineMeanNDVI)
	}
	if summary.ChangeAbs == nil || *summary.ChangeAbs != 0.04 {
		t.Errorf("change abs = %v", summary.ChangeAbs)
	}
	if summary.ChangePct == nil || *summary.ChangePct != 6.25 {
		t.Errorf("change pct = %v, want 6.25", summary.ChangePct)
	}
	// Nothing was acquired and no granule exists, so neither may be invented.
	if summary.CurrentAcquiredAt != nil || summary.CurrentSceneID != "" || summary.BaselineSceneID != "" {
		t.Errorf("sandbox row carries imagery fields: %+v", summary)
	}
	if summary.PlotCount != 1 || summary.MethodologyVersion != "AOI-TEST v1.0" {
		t.Errorf("plot count %d, methodology %q", summary.PlotCount, summary.MethodologyVersion)
	}
	if summary.PeriodEnd != fixtureNow || summary.PeriodStart != fixtureNow.Add(-DefaultWindow) {
		t.Errorf("period = %s .. %s", summary.PeriodStart, summary.PeriodEnd)
	}
	if summary.ResultHash != canon.HashBytes(summary.Result) {
		t.Errorf("stored hash does not match the stored payload")
	}
	rows := storedSummaries(t, service, projectID)
	if len(rows) != 1 || rows[0].ID != summary.ID {
		t.Fatalf("stored rows = %d", len(rows))
	}

	var payload map[string]any
	if err := json.Unmarshal(summary.Result, &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	for _, key := range []string{"project_id", "window_days", "plot_count", "analysis_geometry",
		"framing_bbox", "weight_by_area", "current", "baseline", "mean_ndvi", "coverage_pct",
		"quality", "adapter_mode", "provider", "methodology_version", "processing_version",
		"class_table", "palette_version", "notes"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("commitment is missing %s", key)
		}
	}
	for _, key := range []string{"computed_at", "period_start", "period_end"} {
		if _, ok := payload[key]; ok {
			t.Errorf("commitment carries the clock field %s", key)
		}
	}
	if payload["weight_by_area"] != true {
		t.Errorf("weight_by_area = %v", payload["weight_by_area"])
	}
	geometry, ok := payload["analysis_geometry"].(map[string]any)
	if !ok || geometry["type"] != "MultiPolygon" {
		t.Errorf("analysis geometry = %v", payload["analysis_geometry"])
	}
}

func TestTheSameMeasurementKeepsTheSameCommitment(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	first, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
	if err != nil {
		t.Fatalf("first compute: %v", err)
	}
	service.now = func() time.Time { return fixtureNow.Add(48 * time.Hour) }
	second, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
	if err != nil {
		t.Fatalf("second compute: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("a recomputation reused the row id")
	}
	if first.ResultHash != second.ResultHash || string(first.Result) != string(second.Result) {
		t.Errorf("the clock moved the commitment:\n%s\n%s", first.Result, second.Result)
	}
	if second.PeriodEnd == first.PeriodEnd {
		t.Errorf("the period did not move with the clock")
	}
	// Corrections are additive, so the first figure and its hash stay queryable.
	if rows := storedSummaries(t, service, projectID); len(rows) != 2 {
		t.Fatalf("stored rows = %d, want both computations", len(rows))
	}
	latest, ok, err := service.LatestSummary(projectID)
	if err != nil || !ok {
		t.Fatalf("latest summary: ok %v err %v", ok, err)
	}
	if latest.ID != second.ID {
		t.Errorf("latest = %s, want the newer period %s", latest.ID, second.ID)
	}
}

func TestCoverageBelowTheFloorIsPartial(t *testing.T) {
	provider := &stubProvider{
		current:  measuredScene("S2C_37MER_20261105_1_L2A", 0.837, 41, QualityComplete),
		baseline: measuredScene("S2C_37MER_20251110_0_L2A", 0.7979, 100, QualityComplete),
	}
	service, projectID := summaryFixture(t, provider)
	summary, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if summary.Quality != QualityPartial {
		t.Errorf("quality = %q, want partial below the 60 percent floor", summary.Quality)
	}
	if summary.CoveragePct != 41 {
		t.Errorf("coverage = %v", summary.CoveragePct)
	}
	if summary.ChangeAbs == nil || *summary.ChangeAbs != 0.0391 {
		t.Errorf("change abs = %v, want the recorded 0.0391", summary.ChangeAbs)
	}
	if summary.ChangePct == nil || *summary.ChangePct != 4.9 {
		t.Errorf("change pct = %v, want the recorded 4.90", summary.ChangePct)
	}
	if summary.CurrentSceneID != provider.current.SceneID || summary.CurrentAcquiredAt == nil {
		t.Errorf("live row lost its granule: %+v", summary)
	}
	if summary.AdapterMode != ModeLive || summary.ProcessingVersion != LiveProcessingVersion {
		t.Errorf("live row is not labelled live: %q / %q", summary.AdapterMode, summary.ProcessingVersion)
	}
	var payload struct {
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal(summary.Result, &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if len(payload.Notes) == 0 {
		t.Errorf("a partial figure carries no caveat")
	}
}

func TestAMissingBaselineDoesNotHideTheCurrentMean(t *testing.T) {
	provider := &stubProvider{
		current:  measuredScene("S2C_37MER_20261105_1_L2A", 0.837, 100, QualityComplete),
		baseline: SceneStats{Quality: QualityUnavailable, AdapterMode: ModeLive, ProviderName: ProviderOpenData},
	}
	service, projectID := summaryFixture(t, provider)
	summary, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if summary.Quality != QualityComplete {
		t.Errorf("quality = %q, the measured current mean must stand on its own", summary.Quality)
	}
	if summary.MeanNDVI == nil || *summary.MeanNDVI != 0.837 {
		t.Errorf("mean = %v", summary.MeanNDVI)
	}
	if summary.ChangeAbs != nil || summary.ChangePct != nil || summary.BaselineMeanNDVI != nil {
		t.Errorf("a change was published without a baseline: %+v", summary)
	}
	if summary.BaselineSceneID != "" || summary.BaselineAcquiredAt != nil {
		t.Errorf("an absent baseline acquired imagery fields: %+v", summary)
	}
}

func TestAZerosBaselineHasNoPercentage(t *testing.T) {
	provider := &stubProvider{
		current:  measuredScene("current", 0.2, 100, QualityComplete),
		baseline: measuredScene("baseline", 0, 100, QualityComplete),
	}
	service, projectID := summaryFixture(t, provider)
	summary, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if summary.ChangeAbs == nil || *summary.ChangeAbs != 0.2 {
		t.Errorf("change abs = %v", summary.ChangeAbs)
	}
	if summary.ChangePct != nil {
		t.Errorf("change pct = %v, a zero baseline has no percentage", *summary.ChangePct)
	}
}

func TestAProviderFaultWritesNoPeriod(t *testing.T) {
	provider := &stubProvider{
		current:     measuredScene("current", 0.8, 100, QualityComplete),
		baselineErr: errors.New("dial tcp 10.0.0.1:443: connect: connection refused"),
	}
	service, projectID := summaryFixture(t, provider)
	_, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("err = %v, want the provider fault", err)
	}
	if rows := storedSummaries(t, service, projectID); len(rows) != 0 {
		t.Errorf("an outage was written as a period: %d rows", len(rows))
	}
	// The internal address stays in the log, never in the error a client or a job sees.
	if err.Error() != "baseline observation: "+ErrProviderFailed.Error() {
		t.Errorf("fault text = %q", err.Error())
	}
}

func TestNoPlotsIsAnUnavailableRowAndNotAnError(t *testing.T) {
	db, projectID, _ := openAOIDatabase(t)
	service := NewService(db, Options{})
	service.now = func() time.Time { return fixtureNow }
	summary, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if summary.Quality != QualityUnavailable || summary.MeanNDVI != nil || summary.PlotCount != 0 {
		t.Errorf("empty project row = %+v", summary)
	}
	if summary.CoveragePct != 0 || summary.CurrentSceneID != "" || summary.CurrentAcquiredAt != nil {
		t.Errorf("empty project invented a reading: %+v", summary)
	}
	if summary.AdapterMode != ModeSandbox {
		t.Errorf("adapter mode = %q", summary.AdapterMode)
	}
	view, err := service.ProjectChange(projectID)
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if view.Quality != QualityUnavailable || view.MeanNDVI != nil || view.SummaryID != summary.ID {
		t.Errorf("change view = %+v", view)
	}
}

func TestChangeBeforeAnyComputationIsRefused(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	if _, err := service.ProjectChange(projectID); !errors.Is(err, ErrSummaryMissing) {
		t.Fatalf("err = %v, want the missing-summary refusal", err)
	}
	if _, ok, err := service.LatestSummary(projectID); ok || err != nil {
		t.Fatalf("latest = ok %v err %v, want a clean absence", ok, err)
	}
}

func TestAnUnknownProjectIsRefusedRatherThanOrphaned(t *testing.T) {
	service, _ := summaryFixture(t, nil)
	missing := uuid.NewString()
	_, err := service.ComputeProjectSummary(context.Background(), missing, DefaultWindow)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("err = %v, want the missing project", err)
	}
	if rows := storedSummaries(t, service, missing); len(rows) != 0 {
		t.Errorf("an orphan summary was written: %d rows", len(rows))
	}
}

func TestLatestSummaryBreaksTiesOnTheNewestRun(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	if _, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow); err != nil {
		t.Fatalf("compute: %v", err)
	}
	// Same period_end and same computed_at, which is what two runs inside one second
	// look like. The id tie-break has to pick one deterministically.
	var rows []models.ProjectNdviSummary
	if err := service.db.Where("project_id = ?", projectID).Find(&rows).Error; err != nil {
		t.Fatalf("read rows: %v", err)
	}
	clone := rows[0]
	clone.ID = uuid.NewString()
	if err := service.db.Create(&clone).Error; err != nil {
		t.Fatalf("clone row: %v", err)
	}
	first, ok, err := service.LatestSummary(projectID)
	if err != nil || !ok {
		t.Fatalf("latest: ok %v err %v", ok, err)
	}
	second, _, err := service.LatestSummary(projectID)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("the same query answered twice: %s then %s", first.ID, second.ID)
	}
	want := clone.ID
	if rows[0].ID > want {
		want = rows[0].ID
	}
	if first.ID != want {
		t.Errorf("latest = %s, want the newest id %s", first.ID, want)
	}
}
