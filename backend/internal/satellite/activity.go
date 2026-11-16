package satellite

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const plantingBaseline = "planting_baseline"
const plantingHalfWindow = 15 * 24 * time.Hour

type activityBaselinePayload struct {
	CenterAt time.Time  `json:"center_at"`
	AOI      AOI        `json:"aoi"`
	Scene    SceneStats `json:"scene"`
}

type ActivityNdviReading struct {
	ID                string     `json:"id"`
	SourceActivityID  string     `json:"sourceActivityId,omitempty"`
	MeanNDVI          *float64   `json:"meanNdvi"`
	AcquiredAt        *time.Time `json:"acquiredAt"`
	SceneID           string     `json:"sceneId,omitempty"`
	CloudPct          *float64   `json:"cloudPct"`
	CoveragePct       *float64   `json:"coveragePct"`
	Quality           string     `json:"quality"`
	AdapterMode       string     `json:"adapterMode"`
	Provider          string     `json:"provider"`
	ProcessingVersion string     `json:"processingVersion"`
	ResultHash        string     `json:"resultHash"`
}

type ActivityNdviView struct {
	Status      string               `json:"status"`
	JobID       string               `json:"jobId,omitempty"`
	Error       string               `json:"error,omitempty"`
	CenterAt    *time.Time           `json:"centerAt"`
	WindowStart *time.Time           `json:"windowStart"`
	WindowEnd   *time.Time           `json:"windowEnd"`
	Observation *ActivityNdviReading `json:"observation"`
	Baseline    *ActivityNdviReading `json:"baseline"`
	LatestPass  *ActivityNdviReading `json:"latestPass"`
	ChangeAbs   *float64             `json:"changeAbs"`
	ChangePct   *float64             `json:"changePct"`
}

func NewActivityNdviRequest(activity models.ConservationActivity, plot models.Plot, center time.Time, requestedBy string) (models.ActivityNdviRequest, error) {
	aoi := aoiFromPlots([]models.Plot{plot})
	aoi.ProjectID, aoi.Status = activity.ProjectID, "available"
	geometryHash, _, err := canon.Hash(aoi.Cells)
	if err != nil {
		return models.ActivityNdviRequest{}, err
	}
	body, err := json.Marshal(aoi)
	if err != nil {
		return models.ActivityNdviRequest{}, err
	}
	return models.ActivityNdviRequest{
		ActivityID: activity.ID, ProjectID: activity.ProjectID, PlotID: plot.ID,
		CenterAt: center.UTC(), AOI: body, GeometryHash: geometryHash,
		Status: "waiting", RequestedBy: requestedBy,
		CreatedAt: activity.CreatedAt, UpdatedAt: activity.CreatedAt,
	}, nil
}

func (s *Service) ComputeActivityBaseline(ctx context.Context, job models.SatelliteRefreshJob) (models.SatelliteObservation, error) {
	var request models.ActivityNdviRequest
	if err := s.db.WithContext(ctx).Where("activity_id = ? AND job_id = ?", job.ActivityID, job.ID).First(&request).Error; err != nil {
		return models.SatelliteObservation{}, err
	}
	var project models.ConservationProject
	if err := s.db.WithContext(ctx).Where("id = ?", request.ProjectID).First(&project).Error; err != nil {
		return models.SatelliteObservation{}, err
	}
	if project.Status == "archived" {
		return models.SatelliteObservation{}, domain.Reject("the project is archived")
	}
	var aoi AOI
	if err := json.Unmarshal(request.AOI, &aoi); err != nil {
		return models.SatelliteObservation{}, err
	}
	observation, reused, err := s.reusableActivityBaseline(request)
	if err != nil {
		return observation, err
	}
	if !reused {
		start, end := request.CenterAt.Add(-plantingHalfWindow), request.CenterAt.Add(plantingHalfWindow)
		scene, err := s.provider.ObservationWindow(ctx, aoi, start, end)
		if err != nil {
			return observation, s.providerFault(job.ProjectID, "planting baseline", err)
		}
		if scene.MeanNDVI != nil && (math.IsNaN(*scene.MeanNDVI) || math.IsInf(*scene.MeanNDVI, 0) || *scene.MeanNDVI < -1 || *scene.MeanNDVI > 1) {
			return observation, s.providerFault(job.ProjectID, "planting baseline", errors.New("invalid NDVI"))
		}
		if scene.AdapterMode == ModeLive && scene.MeanNDVI != nil && (scene.SceneID == "" || scene.AcquiredAt.IsZero() || scene.AcquiredAt.Before(start) || scene.AcquiredAt.After(end) || scene.AcquiredAt.After(s.now().UTC())) {
			return observation, s.providerFault(job.ProjectID, "planting baseline", errors.New("scene outside requested window"))
		}
		if scene.AdapterMode == ModeLive && scene.Quality == QualityComplete && scene.CoveragePct < minSummaryCoveragePct {
			scene.Quality = QualityPartial
		}
		status := scene.Quality
		if scene.Quality == QualityUnavailable {
			scene.MeanNDVI = nil
			status = "no_observation"
		}
		if scene.Quality == QualityCloudy {
			scene.MeanNDVI = nil
		}
		payload := activityBaselinePayload{CenterAt: request.CenterAt, AOI: aoi, Scene: scene}
		hash, body, err := canon.Hash(payload)
		if err != nil {
			return observation, err
		}
		observation = models.SatelliteObservation{
			ID: uuid.NewString(), ActivityID: request.ActivityID, PlotID: request.PlotID,
			Purpose: plantingBaseline, GeometryHash: request.GeometryHash,
			AdapterMode: scene.AdapterMode, Status: status, SceneID: scene.SceneID,
			AcquiredAt: acquiredAt(scene), ProcessingVersion: scene.ProcessingVersion,
			Result: body, ResultHash: hash, CreatedAt: s.now().UTC(),
		}
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !reused {
			if err := tx.Create(&observation).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&models.ActivityNdviRequest{}).Where("activity_id = ? AND job_id = ?", request.ActivityID, job.ID).
			Updates(map[string]any{"observation_id": observation.ID, "status": "complete", "updated_at": s.now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("baseline request changed during computation")
		}
		return nil
	})
	return observation, err
}

func usableBaseline(scene SceneStats) bool {
	return scene.MeanNDVI != nil && (scene.Quality == QualityComplete || scene.Quality == QualityPartial || (scene.Quality == QualitySynthetic && scene.AdapterMode == ModeSandbox))
}

func (s *Service) baselineRows(plotID, geometryHash string) ([]models.SatelliteObservation, error) {
	var rows []models.SatelliteObservation
	err := s.db.Where("purpose = ? AND plot_id = ? AND geometry_hash = ?", plantingBaseline, plotID, geometryHash).
		Order("created_at").Order("id").Find(&rows).Error
	return rows, err
}

func (s *Service) reusableActivityBaseline(request models.ActivityNdviRequest) (models.SatelliteObservation, bool, error) {
	rows, err := s.baselineRows(request.PlotID, request.GeometryHash)
	if err != nil {
		return models.SatelliteObservation{}, false, err
	}
	start, end := request.CenterAt.Add(-plantingHalfWindow), request.CenterAt.Add(plantingHalfWindow)
	for _, row := range rows {
		var payload activityBaselinePayload
		if err := json.Unmarshal(row.Result, &payload); err != nil {
			return row, false, err
		}
		scene := payload.Scene
		if scene.ProviderName != s.provider.Name() || !usableBaseline(scene) {
			continue
		}
		if scene.AdapterMode == ModeLive && scene.ProcessingVersion != LiveProcessingVersion {
			continue
		}
		if scene.AdapterMode == ModeSandbox && scene.ProcessingVersion != ProcessingVersion {
			continue
		}
		if scene.AdapterMode == ModeLive {
			if scene.AcquiredAt.IsZero() || scene.AcquiredAt.Before(start) || scene.AcquiredAt.After(end) {
				continue
			}
		} else if payload.CenterAt.Add(plantingHalfWindow).Before(start) || payload.CenterAt.Add(-plantingHalfWindow).After(end) {
			continue
		}
		return row, true, nil
	}
	return models.SatelliteObservation{}, false, nil
}

func (s *Service) RecaptureActivityBaseline(activity models.ConservationActivity, requestedBy string) error {
	if activity.ActivityType != "planting" {
		return domain.Invalid("NDVI baseline capture is available for planting activities")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ConservationProject{}).Where("id = ?", activity.ProjectID).UpdateColumn("id", gorm.Expr("id")).Error; err != nil {
			return err
		}
		var project models.ConservationProject
		if err := tx.Where("id = ?", activity.ProjectID).First(&project).Error; err != nil {
			return err
		}
		if project.Status == "archived" {
			return domain.Reject("the project is archived; unarchive it before making changes")
		}
		var previous models.ActivityNdviRequest
		err := tx.Where("activity_id = ?", activity.ID).First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && previous.Status == "waiting" {
			return nil
		}
		if previous.JobID != "" {
			var job models.SatelliteRefreshJob
			if err := tx.Where("id = ?", previous.JobID).First(&job).Error; err != nil {
				return err
			}
			if job.Status == JobQueued || job.Status == JobRunning {
				return nil
			}
		}
		var plot models.Plot
		if err := tx.Where("id = ? AND project_id = ?", activity.PlotID, activity.ProjectID).First(&plot).Error; err != nil {
			return err
		}
		center := activity.ReportingPeriodStart
		if center.IsZero() {
			center = activity.CreatedAt
		}
		if !previous.CenterAt.IsZero() {
			center = previous.CenterAt
		}
		request, err := NewActivityNdviRequest(activity, plot, center, requestedBy)
		if err != nil {
			return err
		}
		request.CreatedAt, request.UpdatedAt = s.now().UTC(), s.now().UTC()
		return tx.Save(&request).Error
	})
}

func readingOf(row models.SatelliteObservation) (ActivityNdviReading, SceneStats, error) {
	var payload activityBaselinePayload
	if err := json.Unmarshal(row.Result, &payload); err != nil {
		return ActivityNdviReading{}, SceneStats{}, err
	}
	scene := payload.Scene
	view := ActivityNdviReading{
		ID: row.ID, SourceActivityID: row.ActivityID, MeanNDVI: scene.MeanNDVI,
		AcquiredAt: row.AcquiredAt, SceneID: row.SceneID, CloudPct: scene.CloudPct,
		Quality: scene.Quality, AdapterMode: row.AdapterMode, Provider: scene.ProviderName,
		ProcessingVersion: row.ProcessingVersion, ResultHash: row.ResultHash,
	}
	if scene.AdapterMode == ModeLive && scene.ExpectedPixels > 0 {
		coverage := scene.CoveragePct
		view.CoveragePct = &coverage
	}
	return view, scene, nil
}

func (s *Service) ActivityNdvi(activityID string) (ActivityNdviView, error) {
	view := ActivityNdviView{Status: "not_requested"}
	var request models.ActivityNdviRequest
	err := s.db.Where("activity_id = ?", activityID).First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return view, nil
	}
	if err != nil {
		return view, err
	}
	start, end := request.CenterAt.Add(-plantingHalfWindow), request.CenterAt.Add(plantingHalfWindow)
	view.Status, view.CenterAt, view.WindowStart, view.WindowEnd = request.Status, &request.CenterAt, &start, &end
	if request.JobID != "" {
		var job models.SatelliteRefreshJob
		if err := s.db.Where("id = ?", request.JobID).First(&job).Error; err != nil {
			return view, err
		}
		view.JobID, view.Status, view.Error = job.ID, job.Status, job.Error
	}
	if request.ObservationID == "" {
		return view, nil
	}
	var observation models.SatelliteObservation
	if err := s.db.Where("id = ? AND purpose = ?", request.ObservationID, plantingBaseline).First(&observation).Error; err != nil {
		return view, err
	}
	reading, scene, err := readingOf(observation)
	if err != nil {
		return view, err
	}
	view.Observation = &reading
	if view.Status == JobComplete {
		view.Status = observation.Status
	}
	rows, err := s.baselineRows(request.PlotID, request.GeometryHash)
	if err != nil {
		return view, err
	}
	var baselineScene SceneStats
	for _, row := range rows {
		candidate, candidateScene, err := readingOf(row)
		if err != nil {
			return view, err
		}
		if !usableBaseline(candidateScene) || candidate.AdapterMode != reading.AdapterMode || candidate.Provider != reading.Provider || candidate.ProcessingVersion != reading.ProcessingVersion {
			continue
		}
		if candidate.AcquiredAt != nil && candidate.AcquiredAt.After(end) {
			continue
		}
		if view.Baseline == nil || (candidate.AcquiredAt != nil && view.Baseline.AcquiredAt != nil && candidate.AcquiredAt.Before(*view.Baseline.AcquiredAt)) {
			view.Baseline, baselineScene = &candidate, candidateScene
		}
	}
	if view.Baseline == nil || baselineScene.AdapterMode != ModeLive || view.Baseline.AcquiredAt == nil {
		return view, nil
	}
	view.LatestPass, err = s.latestActivityPass(request, *view.Baseline)
	if err != nil {
		return view, err
	}
	// A later planting capture can itself be the newest comparable observation.
	if usableBaseline(scene) && reading.AcquiredAt != nil && reading.AcquiredAt.After(*view.Baseline.AcquiredAt) && (view.LatestPass == nil || reading.AcquiredAt.After(*view.LatestPass.AcquiredAt)) {
		view.LatestPass = &reading
	}
	if view.LatestPass != nil && view.LatestPass.MeanNDVI != nil && view.Baseline.MeanNDVI != nil {
		difference := *view.LatestPass.MeanNDVI - *view.Baseline.MeanNDVI
		delta := round4(difference)
		view.ChangeAbs = &delta
		if *view.Baseline.MeanNDVI != 0 {
			pct := round2(difference / math.Abs(*view.Baseline.MeanNDVI) * 100)
			view.ChangePct = &pct
		}
	}
	return view, nil
}

func (s *Service) latestActivityPass(request models.ActivityNdviRequest, baseline ActivityNdviReading) (*ActivityNdviReading, error) {
	var rows []models.ProjectNdviSummary
	if err := s.db.Where("project_id = ? AND current_acquired_at > ? AND current_acquired_at <= ? AND adapter_mode = ? AND provider = ? AND processing_version = ?",
		request.ProjectID, baseline.AcquiredAt, s.now().UTC(), baseline.AdapterMode, baseline.Provider, baseline.ProcessingVersion).
		Order("current_acquired_at desc").Order("computed_at desc").Limit(100).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		var payload summaryPayload
		if err := json.Unmarshal(row.Result, &payload); err != nil {
			return nil, err
		}
		if !usableBaseline(payload.Current) {
			continue
		}
		for _, cell := range payload.Cells {
			if cell.PlotID != request.PlotID {
				continue
			}
			hash, _, err := canon.Hash([]Cell{cell})
			if err != nil {
				return nil, err
			}
			if hash != request.GeometryHash {
				continue
			}
			for _, stat := range payload.Current.PerCell {
				if stat.PlotID != request.PlotID || stat.MeanNDVI == nil || stat.ValidPixels <= 0 || stat.ExpectedPixels <= 0 {
					continue
				}
				quality := payload.Current.Quality
				if stat.CoveragePct < minSummaryCoveragePct {
					quality = QualityPartial
				}
				coverage := stat.CoveragePct
				return &ActivityNdviReading{
					ID: row.ID, MeanNDVI: stat.MeanNDVI, AcquiredAt: row.CurrentAcquiredAt,
					SceneID: row.CurrentSceneID, CloudPct: row.CurrentCloudPct, CoveragePct: &coverage,
					Quality: quality, AdapterMode: row.AdapterMode, Provider: row.Provider,
					ProcessingVersion: row.ProcessingVersion, ResultHash: row.ResultHash,
				}, nil
			}
		}
	}
	return nil, nil
}
