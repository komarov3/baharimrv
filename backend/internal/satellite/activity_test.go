package satellite

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
)

// Keep the ordinary provider stub, but record the planting-specific seam so a
// correct displayed window cannot hide a different window sent to the worker.
type activityWindowProvider struct {
	*stubProvider
	aoi        AOI
	start, end time.Time
}

func (p *activityWindowProvider) ObservationWindow(ctx context.Context, aoi AOI, start, end time.Time) (SceneStats, error) {
	p.aoi, p.start, p.end = aoi, start, end
	return p.stubProvider.ObservationWindow(ctx, aoi, start, end)
}

func seedActivityNdvi(t *testing.T, service *Service, projectID string, center time.Time) (models.ConservationActivity, models.Plot, models.ActivityNdviRequest) {
	t.Helper()
	var project models.ConservationProject
	if err := service.db.First(&project, "id = ?", projectID).Error; err != nil {
		t.Fatal(err)
	}
	var plot models.Plot
	if err := service.db.Where("project_id = ?", projectID).First(&plot).Error; err != nil {
		t.Fatal(err)
	}
	activity := models.ConservationActivity{
		ID: uuid.NewString(), ProjectID: projectID, PlotID: plot.ID, ManagerID: project.ManagerID,
		ActivityType: "planting", Status: models.ActivityDraft, MethodologyVersion: "1.0", Measurements: []byte(`{}`),
		ReportingPeriodStart: center, ReportingPeriodEnd: fixtureNow,
		CreatedAt: fixtureNow, UpdatedAt: fixtureNow,
	}
	if err := service.db.Create(&activity).Error; err != nil {
		t.Fatalf("seed planting activity: %v", err)
	}
	request, err := NewActivityNdviRequest(activity, plot, center, project.ManagerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.db.Create(&request).Error; err != nil {
		t.Fatalf("persist waiting request: %v", err)
	}
	if request.Status != "waiting" || request.JobID != "" || request.ObservationID != "" || request.RequestedBy != project.ManagerID {
		t.Fatalf("new request is not an attributed waiting capture: %+v", request)
	}
	return activity, plot, request
}

func processActivityNdvi(t *testing.T, service *Service, activityID string) ActivityNdviView {
	t.Helper()
	finished, err := service.ProcessRefreshJobs(context.Background())
	if err != nil || finished != 1 {
		t.Fatalf("process activity request: finished %d, error %v", finished, err)
	}
	view, err := service.ActivityNdvi(activityID)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func activityFloat(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s is nil, want %v", label, want)
	} else if math.IsNaN(*got) || math.Abs(*got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", label, *got, want)
	}
}

func TestActivityNdviCaptureOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, quality, status string
		coverage              float64
		missing, sandbox      bool
	}{
		{name: "complete", quality: QualityComplete, status: "complete", coverage: 100},
		{name: "partial", quality: QualityPartial, status: "partial", coverage: 41},
		{name: "coverage_floor", quality: QualityComplete, status: "partial", coverage: 41},
		{name: "cloudy", quality: QualityCloudy, status: "cloudy", coverage: 10},
		{name: "no_observation", quality: QualityUnavailable, status: "no_observation", missing: true},
		{name: "missing_values", quality: QualityPartial, status: "partial", missing: true},
		{name: "sandbox", quality: QualitySynthetic, status: "synthetic", sandbox: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scene := measuredScene("planting-scene", .5, tc.coverage, tc.quality)
			if tc.missing {
				scene.MeanNDVI, scene.CloudPct = nil, nil
				scene.SceneID, scene.AcquiredAt = "", time.Time{}
				scene.ValidPixels, scene.ExpectedPixels = 0, 0
			}
			stub := &stubProvider{baseline: scene}
			var provider Provider = stub
			if tc.sandbox {
				provider = NewSandboxProvider()
			}
			service, projectID := summaryFixture(t, provider)
			activity, plot, request := seedActivityNdvi(t, service, projectID, fixtureNow)
			waiting, err := service.ActivityNdvi(activity.ID)
			if err != nil || waiting.Status != "waiting" || waiting.Observation != nil || waiting.Baseline != nil {
				t.Fatalf("waiting view = %+v, error %v", waiting, err)
			}
			view := processActivityNdvi(t, service, activity.ID)
			if view.Status != tc.status || view.Error != "" || view.Observation == nil || view.JobID == "" {
				t.Fatalf("capture result = %+v", view)
			}
			var row models.SatelliteObservation
			if err := service.db.First(&row, "id = ?", view.Observation.ID).Error; err != nil {
				t.Fatal(err)
			}
			if row.Purpose != "planting_baseline" || row.ActivityID != activity.ID || row.PlotID != plot.ID || row.GeometryHash != request.GeometryHash || row.Status != tc.status {
				t.Errorf("stored baseline attribution = %+v", row)
			}
			if row.ResultHash == "" || row.ResultHash != canon.HashBytes(row.Result) || view.Observation.ResultHash != row.ResultHash {
				t.Error("the returned commitment does not match the stored payload")
			}
			var stored activityBaselinePayload
			if err := json.Unmarshal(row.Result, &stored); err != nil {
				t.Fatal(err)
			}
			job, err := service.RefreshJob(projectID, view.JobID)
			if err != nil || job.Status != JobComplete || job.ActivityID != activity.ID || job.ObservationID != row.ID || job.SummaryID != "" {
				t.Errorf("job did not complete with its activity observation: %+v, error %v", job, err)
			}
			if tc.missing || tc.quality == QualityCloudy {
				if view.Observation.MeanNDVI != nil || stored.Scene.MeanNDVI != nil || view.Baseline != nil {
					t.Errorf("an unusable observation invented a baseline: %+v", view)
				}
			} else {
				want := .5
				if tc.sandbox {
					want = .64
				}
				activityFloat(t, "mean", view.Observation.MeanNDVI, want)
				if view.Baseline == nil || view.Baseline.ID != row.ID {
					t.Errorf("usable capture is not its own first baseline: %+v", view)
				}
			}
			if tc.missing || tc.sandbox {
				if view.Observation.AcquiredAt != nil || view.Observation.SceneID != "" || view.Observation.CloudPct != nil || view.Observation.CoveragePct != nil {
					t.Errorf("missing imagery metadata was invented: %+v", view.Observation)
				}
			} else {
				if view.Observation.AcquiredAt == nil || !view.Observation.AcquiredAt.Equal(scene.AcquiredAt) || view.Observation.SceneID != scene.SceneID {
					t.Errorf("scene provenance lost: %+v", view.Observation)
				}
				activityFloat(t, "cloud", view.Observation.CloudPct, *scene.CloudPct)
				activityFloat(t, "coverage", view.Observation.CoveragePct, tc.coverage)
			}
			mode, source, version := ModeLive, ProviderOpenData, LiveProcessingVersion
			if tc.sandbox {
				mode, source, version = ModeSandbox, ProviderSandbox, ProcessingVersion
			}
			if view.Observation.AdapterMode != mode || view.Observation.Provider != source || view.Observation.ProcessingVersion != version || view.Observation.Quality != stored.Scene.Quality {
				t.Errorf("capture labels = %+v", view.Observation)
			}
			if view.LatestPass != nil || view.ChangeAbs != nil || view.ChangePct != nil {
				t.Errorf("a single capture fabricated a later comparison: %+v", view)
			}
			if !tc.sandbox && stub.reads != 1 {
				t.Errorf("provider reads = %d, want one planting window", stub.reads)
			}
			if rows := storedSummaries(t, service, projectID); len(rows) != 0 {
				t.Error("planting capture also wrote a project summary")
			}
		})
	}
}

func TestActivityNdviUsesExactWindowAndGeometrySnapshot(t *testing.T) {
	for _, boundary := range []bool{false, true} {
		name := "centroid_buffer"
		if boundary {
			name = "surveyed_boundary"
		}
		t.Run(name, func(t *testing.T) {
			provider := &activityWindowProvider{stubProvider: &stubProvider{baseline: measuredScene("snapshot-scene", .5, 100, QualityComplete)}}
			service, projectID := summaryFixture(t, provider)
			if boundary {
				outline := []byte(`{"type":"Polygon","coordinates":[[[39.835,-3.641],[39.837,-3.641],[39.836,-3.639],[39.835,-3.641]]]}`)
				if err := service.db.Model(&models.Plot{}).Where("project_id = ?", projectID).Update("boundary", outline).Error; err != nil {
					t.Fatal(err)
				}
			}
			center := time.Date(2026, 11, 4, 17, 32, 19, 123000000, time.FixedZone("EAT", 3*60*60))
			activity, plot, request := seedActivityNdvi(t, service, projectID, center)
			var snapshot AOI
			if err := json.Unmarshal(request.AOI, &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot.ProjectID != projectID || snapshot.PlotCount != 1 || len(snapshot.Cells) != 1 || snapshot.Cells[0].PlotID != plot.ID || snapshot.CentroidLat != plot.Latitude || snapshot.CentroidLon != plot.Longitude {
				t.Fatalf("not a single-plot snapshot: %+v", snapshot)
			}
			var geometry polygonGeometry
			if err := json.Unmarshal(snapshot.Cells[0].Geometry, &geometry); err != nil {
				t.Fatal(err)
			}
			if boundary {
				var expected polygonGeometry
				if err := json.Unmarshal(plot.Boundary, &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(geometry, expected) {
					t.Errorf("surveyed boundary replaced by a buffer: %+v", geometry)
				}
			} else if geometry.Type != "Polygon" || len(geometry.Coordinates) != 1 || len(geometry.Coordinates[0]) != 5 {
				t.Errorf("centroid buffer is not a closed square: %+v", geometry)
			} else {
				ring := geometry.Coordinates[0]
				if math.Abs((ring[0][0]+ring[2][0])/2-plot.Longitude) > 1e-9 || math.Abs((ring[0][1]+ring[2][1])/2-plot.Latitude) > 1e-9 || ring[0] != ring[4] {
					t.Errorf("buffer not centred on the recorded plot: %v", ring)
				}
			}
			if err := service.db.Model(&models.Plot{}).Where("id = ?", plot.ID).Updates(map[string]any{
				"latitude": -4.0, "longitude": 39.0, "area_hectares": "1.000000", "boundary": nil,
			}).Error; err != nil {
				t.Fatal(err)
			}
			view := processActivityNdvi(t, service, activity.ID)
			wantStart, wantEnd := center.Add(-15*24*time.Hour), center.Add(15*24*time.Hour)
			if !provider.start.Equal(wantStart) || !provider.end.Equal(wantEnd) || !reflect.DeepEqual(provider.aoi, snapshot) {
				t.Errorf("provider received a changed date or geometry: %s .. %s, %+v", provider.start, provider.end, provider.aoi)
			}
			if view.CenterAt == nil || !view.CenterAt.Equal(center) || view.WindowStart == nil || !view.WindowStart.Equal(wantStart) || view.WindowEnd == nil || !view.WindowEnd.Equal(wantEnd) {
				t.Errorf("displayed window differs from requested window: %+v", view)
			}
			if request.CenterAt.Location() != time.UTC {
				t.Error("request center was not normalized to UTC")
			}
			hash, _, err := canon.Hash(snapshot.Cells)
			if err != nil || hash != request.GeometryHash {
				t.Errorf("snapshot hash = %q, error %v", hash, err)
			}
		})
	}
}

func TestActivityNdviProviderFaultRetriesWithoutObservation(t *testing.T) {
	provider := &stubProvider{baselineErr: errors.New("https://private-worker:443 failed; token=secret-credential")}
	service, projectID := summaryFixture(t, provider)
	service.retryDelay = 0
	activity, _, _ := seedActivityNdvi(t, service, projectID, fixtureNow)
	view := processActivityNdvi(t, service, activity.ID)
	if provider.reads != 2 {
		t.Errorf("attempts = %d, want initial attempt and one retry", provider.reads)
	}
	const safeError = "The imagery provider did not answer. Request a refresh to try again."
	if view.Status != JobFailed || view.Error != safeError || view.Observation != nil || view.Baseline != nil || view.ChangeAbs != nil || view.ChangePct != nil {
		t.Errorf("provider failure was not safely reported: %+v", view)
	}
	job, err := service.RefreshJob(projectID, view.JobID)
	if err != nil || job.Status != JobFailed || job.Error != safeError || job.ObservationID != "" || job.FinishedAt == nil {
		t.Errorf("failed job = %+v, error %v", job, err)
	}
	var count int64
	if err := service.db.Model(&models.SatelliteObservation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("provider outage persisted %d observations", count)
	}
}

func TestActivityNdviRejectsInvalidProviderReadings(t *testing.T) {
	for _, name := range []string{"future", "before_window", "after_window", "missing_date", "missing_scene", "nan", "infinite", "out_of_range"} {
		t.Run(name, func(t *testing.T) {
			scene := measuredScene("invalid-scene", .5, 100, QualityComplete)
			switch name {
			case "future":
				scene.AcquiredAt = fixtureNow.Add(time.Hour)
			case "before_window":
				scene.AcquiredAt = fixtureNow.Add(-plantingHalfWindow - time.Second)
			case "after_window":
				scene.AcquiredAt = fixtureNow.Add(plantingHalfWindow + time.Second)
			case "missing_date":
				scene.AcquiredAt = time.Time{}
			case "missing_scene":
				scene.SceneID = ""
			case "nan":
				value := math.NaN()
				scene.MeanNDVI = &value
			case "infinite":
				value := math.Inf(1)
				scene.MeanNDVI = &value
			case "out_of_range":
				value := 1.01
				scene.MeanNDVI = &value
			}
			provider := &stubProvider{baseline: scene}
			service, projectID := summaryFixture(t, provider)
			service.retryDelay = 0
			activity, _, _ := seedActivityNdvi(t, service, projectID, fixtureNow)
			view := processActivityNdvi(t, service, activity.ID)
			if view.Status != JobFailed || view.Observation != nil || provider.reads != 2 {
				t.Fatalf("invalid provider reading accepted: %+v, reads %d", view, provider.reads)
			}
			var count int64
			if err := service.db.Model(&models.SatelliteObservation{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Errorf("invalid provider reading persisted %d observations", count)
			}
		})
	}
}

func TestActivityNdviObservationWriteRollsBackWithRequest(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	activity, _, _ := seedActivityNdvi(t, service, projectID, fixtureNow)
	if err := service.admitActivityRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	job, ok, err := service.claimJob(context.Background())
	if err != nil || !ok {
		t.Fatalf("claim baseline: %v, %v", ok, err)
	}
	if err := service.db.Exec(`CREATE TRIGGER reject_baseline_completion BEFORE UPDATE ON activity_ndvi_requests
		BEGIN SELECT RAISE(ABORT, 'test refusal'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.ComputeActivityBaseline(context.Background(), job); err == nil {
		t.Fatal("request update failure was swallowed")
	}
	var count int64
	if err := service.db.Model(&models.SatelliteObservation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("unlinked observation survived rollback: %d", count)
	}
	view, err := service.ActivityNdvi(activity.ID)
	if err != nil || view.Observation != nil || view.Status != JobRunning {
		t.Errorf("rollback changed the capture view: %+v, %v", view, err)
	}
}

func TestActivityNdviBaselineReuseRequiresCompatibleCapture(t *testing.T) {
	for _, name := range []string{"same_geometry", "partial", "changed_centroid", "changed_area", "changed_boundary", "changed_provider", "changed_processing_version", "acquisition_outside_window", "cloudy_not_reused"} {
		t.Run(name, func(t *testing.T) {
			center := fixtureNow.Add(-20 * 24 * time.Hour)
			scene := measuredScene("original", .5, 100, QualityComplete)
			scene.AcquiredAt = center
			switch name {
			case "changed_provider":
				scene.ProviderName = "previous-provider"
			case "changed_processing_version":
				scene.ProcessingVersion = "ndvi/s2-l2a-opendata/0"
			case "partial":
				scene.Quality, scene.CoveragePct = QualityPartial, 41
			case "cloudy_not_reused":
				scene.Quality = QualityCloudy
			}
			provider := &stubProvider{baseline: scene}
			service, projectID := summaryFixture(t, provider)
			first, plot, _ := seedActivityNdvi(t, service, projectID, center)
			original := processActivityNdvi(t, service, first.ID)
			if original.Observation == nil {
				t.Fatalf("first capture missing: %+v", original)
			}
			changes := map[string]any{}
			switch name {
			case "changed_centroid":
				changes["latitude"] = plot.Latitude + .01
			case "changed_area":
				changes["area_hectares"] = "3.000000"
			case "changed_boundary":
				changes["boundary"] = []byte(`{"type":"Polygon","coordinates":[[[39.835,-3.641],[39.837,-3.641],[39.836,-3.639],[39.835,-3.641]]]}`)
			}
			if len(changes) > 0 {
				if err := service.db.Model(&models.Plot{}).Where("id = ?", plot.ID).Updates(changes).Error; err != nil {
					t.Fatal(err)
				}
			}
			nextCenter := center.Add(24 * time.Hour)
			if name == "acquisition_outside_window" {
				nextCenter = center.Add(16 * 24 * time.Hour)
			}
			provider.baseline = measuredScene("new-capture", .6, 100, QualityComplete)
			provider.baseline.AcquiredAt = nextCenter
			second, _, _ := seedActivityNdvi(t, service, projectID, nextCenter)
			view := processActivityNdvi(t, service, second.ID)
			if view.Observation == nil {
				t.Fatalf("second capture missing: %+v", view)
			}
			wantReuse := name == "same_geometry" || name == "partial"
			if wantReuse {
				if provider.reads != 1 || view.Observation.ID != original.Observation.ID || view.Observation.SourceActivityID != first.ID || view.Observation.ResultHash != original.Observation.ResultHash {
					t.Errorf("compatible baseline was not reused: reads %d, view %+v", provider.reads, view)
				}
				activityFloat(t, "reused mean", view.Observation.MeanNDVI, .5)
			} else {
				if provider.reads != 2 || view.Observation.ID == original.Observation.ID || view.Observation.SourceActivityID != second.ID {
					t.Errorf("incompatible baseline reused: reads %d, view %+v", provider.reads, view)
				}
				activityFloat(t, "new mean", view.Observation.MeanNDVI, .6)
			}
		})
	}
}

func TestActivityNdviBaselineDoesNotEnterCorroborationReads(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	activity, _, _ := seedActivityNdvi(t, service, projectID, fixtureNow)
	processActivityNdvi(t, service, activity.ID)
	if _, ok := service.Latest(activity.ID); ok {
		t.Error("planting baseline appeared as proof corroboration")
	}
	if _, ok := service.LatestMeanNDVI(activity.ID); ok {
		t.Error("planting baseline appeared as a corroborated plot mean")
	}
	rows, err := service.Summaries(activity.ID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("baseline-only corroboration list = %+v, error %v", rows, err)
	}
	// The filter must not hide genuine corroboration, even when a baseline is newer.
	service.now = func() time.Time { return fixtureNow.Add(-time.Hour) }
	corroboration, err := service.AttachSandbox(activity, .4)
	if err != nil {
		t.Fatal(err)
	}
	latest, ok := service.Latest(activity.ID)
	if !ok || latest.ID != corroboration.ID {
		t.Errorf("latest corroboration = %+v, found %v", latest, ok)
	}
	rows, err = service.Summaries(activity.ID)
	if err != nil || len(rows) != 1 || rows[0].ID != corroboration.ID {
		t.Errorf("corroboration list = %+v, error %v", rows, err)
	}
}

func TestActivityNdviPercentageUsesUnroundedDifference(t *testing.T) {
	baseline := measuredScene("planting-baseline", .10004, 100, QualityComplete)
	baselineMean := .10004
	baseline.MeanNDVI = &baselineMean
	baseline.AcquiredAt = fixtureNow.Add(-30 * 24 * time.Hour)
	provider := &stubProvider{baseline: baseline}
	service, projectID := summaryFixture(t, provider)
	activity, plot, _ := seedActivityNdvi(t, service, projectID, baseline.AcquiredAt)
	processActivityNdvi(t, service, activity.ID)
	provider.current = measuredScene("later-pass", .20005, 100, QualityComplete)
	mean := .20005
	provider.current.PerCell = []CellStats{{PlotID: plot.ID, AreaHectares: 2.5, MeanNDVI: &mean, ValidPixels: 100, ExpectedPixels: 100, CoveragePct: 100}}
	provider.baseline = measuredScene("annual-baseline", .1, 100, QualityComplete)
	provider.baseline.AcquiredAt = fixtureNow.AddDate(-1, 0, 0)
	if _, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow); err != nil {
		t.Fatal(err)
	}
	view, err := service.ActivityNdvi(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	activityFloat(t, "rounded absolute difference", view.ChangeAbs, .1)
	activityFloat(t, "percentage before rounding", view.ChangePct, 99.97)
}

func TestActivityNdviReportUsesComparablePerPlotPass(t *testing.T) {
	for _, name := range []string{"later_pass", "partial_pass", "changed_geometry", "same_scene_and_date", "same_date", "older_pass", "future_pass", "changed_provider", "changed_processing_version", "sandbox_pass", "sandbox_baseline", "missing_baseline", "zero_baseline", "missing_per_cell_mean", "no_per_cell_pixels"} {
		t.Run(name, func(t *testing.T) {
			baseline := measuredScene("planting-baseline", .5, 100, QualityComplete)
			baseline.AcquiredAt = fixtureNow.Add(-30 * 24 * time.Hour)
			switch name {
			case "missing_baseline":
				baseline = SceneStats{Quality: QualityUnavailable, AdapterMode: ModeLive, ProviderName: ProviderOpenData, ProcessingVersion: LiveProcessingVersion}
			case "zero_baseline":
				value := 0.0
				baseline.MeanNDVI = &value
			case "sandbox_baseline":
				baseline.AdapterMode, baseline.Quality = ModeSandbox, QualitySynthetic
				baseline.ProviderName, baseline.ProcessingVersion = ProviderSandbox, ProcessingVersion
				baseline.SceneID, baseline.AcquiredAt = "", time.Time{}
			}
			provider := &stubProvider{baseline: baseline}
			service, projectID := summaryFixture(t, provider)
			activity, plot, _ := seedActivityNdvi(t, service, projectID, fixtureNow.Add(-30*24*time.Hour))
			captured := processActivityNdvi(t, service, activity.ID)
			if captured.Observation == nil {
				t.Fatalf("baseline capture missing: %+v", captured)
			}
			if name == "changed_geometry" {
				if err := service.db.Model(&models.Plot{}).Where("id = ?", plot.ID).Update("latitude", plot.Latitude+.01).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Deliberately distinct from the plot's .7: neither the project mean nor
			// its annual-baseline delta belongs in a planting comparison.
			current := measuredScene("later-pass", .9, 100, QualityComplete)
			mean := .7
			current.PerCell = []CellStats{{PlotID: plot.ID, AreaHectares: 2.5, MeanNDVI: &mean, ValidPixels: 100, ExpectedPixels: 100, CoveragePct: 100}}
			switch name {
			case "same_scene_and_date":
				current.SceneID, current.AcquiredAt = baseline.SceneID, baseline.AcquiredAt
			case "same_date":
				current.AcquiredAt = baseline.AcquiredAt
			case "older_pass":
				current.AcquiredAt = baseline.AcquiredAt.Add(-time.Hour)
			case "future_pass":
				current.AcquiredAt = fixtureNow.Add(time.Hour)
			case "changed_provider":
				current.ProviderName = "different-provider"
			case "changed_processing_version":
				current.ProcessingVersion = "ndvi/s2-l2a-opendata/2"
			case "sandbox_pass":
				current.AdapterMode, current.Quality = ModeSandbox, QualitySynthetic
				current.ProviderName, current.ProcessingVersion = ProviderSandbox, ProcessingVersion
				current.SceneID, current.AcquiredAt = "", time.Time{}
			case "missing_per_cell_mean":
				current.PerCell[0].MeanNDVI = nil
			case "no_per_cell_pixels":
				current.PerCell[0].ValidPixels = 0
			case "partial_pass":
				current.PerCell[0].CoveragePct, current.PerCell[0].ValidPixels = 41, 41
			}
			provider.current = current
			provider.baseline = measuredScene("annual-baseline", .1, 100, QualityComplete)
			provider.baseline.AcquiredAt = fixtureNow.AddDate(-1, 0, 0)
			summary, err := service.ComputeProjectSummary(context.Background(), projectID, DefaultWindow)
			if err != nil {
				t.Fatal(err)
			}
			var payload summaryPayload
			if err := json.Unmarshal(summary.Result, &payload); err != nil {
				t.Fatal(err)
			}
			aoi, err := service.ProjectAOI(projectID)
			if err != nil {
				t.Fatal(err)
			}
			wantCellsHash, _, err := canon.Hash(aoi.Cells)
			if err != nil {
				t.Fatal(err)
			}
			gotCellsHash, _, err := canon.Hash(payload.Cells)
			if err != nil || gotCellsHash != wantCellsHash || !reflect.DeepEqual(payload.Current.PerCell, current.PerCell) {
				t.Fatalf("project summary did not retain cells and per-cell statistics: %+v, error %v", payload, err)
			}
			if summary.ResultHash != canon.HashBytes(summary.Result) {
				t.Error("project summary geometry and statistics are not committed")
			}
			view, err := service.ActivityNdvi(activity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if name != "later_pass" && name != "partial_pass" && name != "zero_baseline" {
				if view.LatestPass != nil || view.ChangeAbs != nil || view.ChangePct != nil {
					t.Errorf("incompatible or missing pass published a comparison: %+v", view)
				}
				return
			}
			if view.Baseline == nil || view.Baseline.ID != captured.Observation.ID || view.LatestPass == nil || view.LatestPass.ID != summary.ID || view.LatestPass.ResultHash != summary.ResultHash {
				t.Fatalf("report lost baseline/pass provenance: %+v", view)
			}
			activityFloat(t, "plot pass mean", view.LatestPass.MeanNDVI, .7)
			if name == "zero_baseline" {
				activityFloat(t, "zero baseline delta", view.ChangeAbs, .7)
				if view.ChangePct != nil {
					t.Error("zero baseline must not produce a percentage")
				}
			} else {
				activityFloat(t, "planting delta", view.ChangeAbs, .2)
				activityFloat(t, "planting percent", view.ChangePct, 40)
			}
			wantQuality, wantCoverage := QualityComplete, 100.0
			if name == "partial_pass" {
				wantQuality, wantCoverage = QualityPartial, 41
			}
			if view.LatestPass.Quality != wantQuality {
				t.Errorf("pass quality = %q, want %q", view.LatestPass.Quality, wantQuality)
			}
			activityFloat(t, "per-plot coverage", view.LatestPass.CoveragePct, wantCoverage)
		})
	}
}
