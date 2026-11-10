// Package satellite provides NDVI corroboration summaries. Only two modes exist:
// a sandbox model that is always labelled as synthetic, and an unconfigured live
// adapter that reports unavailable. A live result requires real scene metadata.
package satellite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const ProcessingVersion = "sandbox-ndvi/1"

type Summary struct {
	ID                string    `json:"id"`
	AdapterMode       string    `json:"adapter_mode"`
	Status            string    `json:"status"`
	SceneID           string    `json:"scene_id,omitempty"`
	AcquiredAt        time.Time `json:"acquired_at"`
	ProcessingVersion string    `json:"processing_version"`
	MeanNDVI          float64   `json:"mean_ndvi"`
	ChangeVsBaseline  float64   `json:"change_vs_baseline"`
	Source            string    `json:"source"`
	ResultHash        string    `json:"result_hash"`
	Live              bool      `json:"live"`
}

type Service struct {
	db       *gorm.DB
	mode     string
	provider Provider
	now      func() time.Time
	// retryDelay is the pause between attempts at a failed provider call. It is a
	// field so a test can drive the retry path without waiting out the real one.
	retryDelay time.Duration
}

// Options is what the environment says about imagery. Mode still decides how
// per-activity corroboration behaves; Provider decides where a project figure comes
// from, and an unset or unrecognised value means sandbox.
type Options struct {
	Mode        string
	Provider    string
	WorkerURL   string
	WorkerToken string
}

func NewService(db *gorm.DB, opts Options) *Service {
	service := &Service{db: db, mode: opts.Mode, now: time.Now, retryDelay: jobRetryDelay}
	switch opts.Provider {
	case ProviderOpenData:
		service.provider = NewOpenDataStac(opts.WorkerURL, opts.WorkerToken)
	default:
		service.provider = NewSandboxProvider()
	}
	return service
}

// Provider names the installed source so health and a summary can say which
// machinery produced a figure. It is never inferred from SATELLITE_MODE.
func (s *Service) Provider() Provider { return s.provider }

func (s *Service) ProviderName() string { return s.provider.Name() }

// Mode reports the configured adapter. Anything other than "live" is sandbox.
func (s *Service) Mode() string { return s.mode }

// Latest returns the most recent corroboration for an activity, if any.
func (s *Service) Latest(activityID string) (Summary, bool) {
	var observation models.SatelliteObservation
	if err := s.db.Where("activity_id = ?", activityID).Order("created_at desc").First(&observation).Error; err != nil {
		return Summary{}, false
	}
	return toSummary(observation), true
}

// LatestMeanNDVI exposes the dashboard overlay value for a plot's activity.
func (s *Service) LatestMeanNDVI(activityID string) (float64, bool) {
	summary, ok := s.Latest(activityID)
	if !ok || summary.Status != "complete" {
		return 0, false
	}
	return summary.MeanNDVI, true
}

// AttachSandbox records a synthetic NDVI observation for a demo activity. It is
// stored with adapter_mode "sandbox" and can never be read back as live imagery.
func (s *Service) AttachSandbox(activity models.ConservationActivity, baselineNDVI float64) (Summary, error) {
	if s.mode == "live" {
		return Summary{}, fmt.Errorf("live satellite corroboration requires scene metadata from the configured provider")
	}
	mean := clamp(baselineNDVI + 0.04)
	result := map[string]any{
		"mean_ndvi":          round4(mean),
		"baseline_ndvi":      round4(baselineNDVI),
		"change_vs_baseline": round4(mean - baselineNDVI),
		"tiles":              []string{"sandbox-tile-01"},
		"note":               "Sandbox model output. Not satellite imagery.",
	}
	hash, canonical, err := canon.Hash(result)
	if err != nil {
		return Summary{}, err
	}
	now := s.now().UTC()
	observation := models.SatelliteObservation{
		ID: uuid.NewString(), ActivityID: activity.ID, AdapterMode: "sandbox", Status: "complete",
		SceneID: "sandbox-scene-" + shortHash(activity.ID), AcquiredAt: &now,
		ProcessingVersion: ProcessingVersion, Result: canonical, ResultHash: hash, CreatedAt: now,
	}
	if err := s.db.Create(&observation).Error; err != nil {
		return Summary{}, fmt.Errorf("store sandbox observation: %w", err)
	}
	return toSummary(observation), nil
}

// Summaries lists every corroboration attached to an activity.
func (s *Service) Summaries(activityID string) ([]Summary, error) {
	var observations []models.SatelliteObservation
	if err := s.db.Where("activity_id = ?", activityID).Order("created_at asc").Find(&observations).Error; err != nil {
		return nil, err
	}
	summaries := make([]Summary, 0, len(observations))
	for _, observation := range observations {
		summaries = append(summaries, toSummary(observation))
	}
	return summaries, nil
}

func toSummary(observation models.SatelliteObservation) Summary {
	var result struct {
		MeanNDVI         float64 `json:"mean_ndvi"`
		ChangeVsBaseline float64 `json:"change_vs_baseline"`
	}
	if len(observation.Result) > 0 {
		_ = json.Unmarshal(observation.Result, &result)
	}
	acquired := time.Time{}
	if observation.AcquiredAt != nil {
		acquired = *observation.AcquiredAt
	}
	source := "Sandbox NDVI model (not live imagery)"
	if observation.AdapterMode == "live" {
		source = observation.SceneID
	}
	return Summary{
		ID: observation.ID, AdapterMode: observation.AdapterMode, Status: observation.Status,
		SceneID: observation.SceneID, AcquiredAt: acquired,
		ProcessingVersion: observation.ProcessingVersion, MeanNDVI: result.MeanNDVI,
		ChangeVsBaseline: result.ChangeVsBaseline, Source: source, ResultHash: observation.ResultHash,
		Live: observation.AdapterMode == "live",
	}
}

func clamp(value float64) float64 {
	return math.Max(-1, math.Min(1, value))
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func shortHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

type AOI struct {
	ProjectID    string          `json:"projectId,omitempty"`
	CentroidLat  float64         `json:"centroidLatitude"`
	CentroidLon  float64         `json:"centroidLongitude"`
	MinLat       float64         `json:"minLatitude"`
	MaxLat       float64         `json:"maxLatitude"`
	MinLon       float64         `json:"minLongitude"`
	MaxLon       float64         `json:"maxLongitude"`
	PlotCount    int             `json:"plotCount"`
	Status       string          `json:"status"`
	Cells        []Cell          `json:"cells,omitempty"`
	AnalysisGeom json.RawMessage `json:"analysisGeometry,omitempty"`
	FramingBBox  []float64       `json:"framingBBox,omitempty"`
}

type Pass struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"projectId"`
	Mission       string     `json:"mission"`
	PassType      string     `json:"passType"`
	WindowStart   time.Time  `json:"windowStart"`
	WindowEnd     time.Time  `json:"windowEnd"`
	AcquiredAt    *time.Time `json:"acquiredAt,omitempty"`
	SceneID       string     `json:"sceneId,omitempty"`
	CloudCoverPct *float64   `json:"cloudCoverPct,omitempty"`
	CoveragePct   float64    `json:"coveragePct"`
	StatusDetail  string     `json:"statusDetail"`
	AdapterMode   string     `json:"adapterMode"`
}

// ProjectAOI is the project's analysis area: one buffered cell per registered plot, the
// geometry they cover, and a bbox for framing the map. The cell order is fixed by code
// then id, because the geometry is hashed into the summary and a database that returned
// rows in a different order would otherwise change the commitment for the same ground.
func (s *Service) ProjectAOI(projectID string) (AOI, error) {
	var plots []models.Plot
	if err := s.db.Where("project_id = ?", projectID).Order("code").Order("id").Find(&plots).Error; err != nil {
		return AOI{}, err
	}
	if len(plots) == 0 {
		return AOI{ProjectID: projectID, Status: "unavailable"}, nil
	}
	aoi := aoiFromPlots(plots)
	aoi.ProjectID = projectID
	aoi.Status = "available"
	return aoi, nil
}

func (s *Service) missionRepeat(mission string) time.Duration {
	if mission == "S1" {
		return 6 * 24 * time.Hour
	}
	return 5 * 24 * time.Hour
}

func (s *Service) PredictPasses(projectID, mission string, from, to time.Time) ([]Pass, error) {
	mission = strings.ToUpper(strings.TrimSpace(mission))
	if mission != "S1" && mission != "S2" {
		mission = "S2"
	}
	aoi, err := s.ProjectAOI(projectID)
	if err != nil {
		return nil, err
	}
	if aoi.Status != "available" {
		return []Pass{{ID: "unavailable", ProjectID: projectID, Mission: mission, PassType: "unavailable", StatusDetail: "No project plots/AOI are available for satellite correlation.", AdapterMode: s.mode}}, nil
	}
	if s.mode != "live" {
		// Sandbox: deterministic synthetic passes. Never labelled live.
		passes := make([]Pass, 0, 4)
		lastAcquired := from.Add(-24 * time.Hour)
		cloud := 18.0
		if mission == "S2" {
			cloud = 42.0
		}
		passes = append(passes, Pass{
			ID: "sandbox-last-" + mission, ProjectID: projectID, Mission: mission,
			PassType: "processed", WindowStart: lastAcquired, WindowEnd: lastAcquired.Add(15 * time.Minute),
			AcquiredAt: &lastAcquired, SceneID: "sandbox-scene-" + mission + "-" + shortHash(projectID),
			CloudCoverPct: &cloud, CoveragePct: 88.0,
			StatusDetail: "Sandbox synthetic scene. Not live satellite imagery.", AdapterMode: "sandbox",
		})
		repeat := s.missionRepeat(mission)
		cursor := from.UTC().Truncate(time.Hour)
		for i := 0; i < 3; i++ {
			start := cursor.Add(time.Duration(i)*repeat + 2*time.Hour)
			if start.After(to) {
				break
			}
			end := start.Add(15 * time.Minute)
			passes = append(passes, Pass{
				ID: fmt.Sprintf("sandbox-predicted-%s-%d", mission, i), ProjectID: projectID, Mission: mission,
				PassType: "predicted", WindowStart: start, WindowEnd: end,
				CoveragePct:  90.0,
				StatusDetail: "Predicted acquisition window based on AOI and orbit cycle. Not a confirmed acquisition.",
				AdapterMode:  "sandbox",
			})
		}
		return passes, nil
	}
	// Live mode: placeholder for real provider integration.
	return []Pass{{ID: "live-unavailable", ProjectID: projectID, Mission: mission, PassType: "unavailable", StatusDetail: "Live satellite provider is not configured.", AdapterMode: "unavailable"}}, nil
}

func (s *Service) LatestPass(projectID, mission string) (Pass, bool, error) {
	passes, err := s.PredictPasses(projectID, mission, time.Now().UTC().Add(-30*24*time.Hour), time.Now().UTC().Add(30*24*time.Hour))
	if err != nil {
		return Pass{}, false, err
	}
	for _, pass := range passes {
		if pass.PassType == "processed" {
			return pass, true, nil
		}
	}
	if len(passes) > 0 {
		return passes[0], true, nil
	}
	return Pass{}, false, nil
}
