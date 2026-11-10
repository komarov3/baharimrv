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
