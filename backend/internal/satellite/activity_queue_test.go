package satellite

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func seedActivityQueueRequest(t *testing.T, s *Service, projectID string, createdAt time.Time) models.ActivityNdviRequest {
	t.Helper()
	var project models.ConservationProject
	var plot models.Plot
	if err := s.db.First(&project, "id = ?", projectID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.First(&plot, "project_id = ?", projectID).Error; err != nil {
		t.Fatal(err)
	}
	activity := models.ConservationActivity{
		ID: uuid.NewString(), ProjectID: projectID, PlotID: plot.ID, ManagerID: project.ManagerID,
		ActivityType: "planting", Status: models.ActivityDraft, MethodologyVersion: "1.0",
		ReportingPeriodStart: createdAt, ReportingPeriodEnd: createdAt, Measurements: datatypes.JSON(`{}`),
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if err := s.db.Create(&activity).Error; err != nil {
		t.Fatal(err)
	}
	request := models.ActivityNdviRequest{
		ActivityID: activity.ID, ProjectID: projectID, PlotID: plot.ID, CenterAt: createdAt,
		AOI: datatypes.JSON(`{}`), GeometryHash: "captured-geometry", Status: models.ActivityNdviWaiting,
		RequestedBy: project.ManagerID, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if err := s.db.Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	return request
}

func readActivityQueueRequest(t *testing.T, s *Service, activityID string) models.ActivityNdviRequest {
	t.Helper()
	var request models.ActivityNdviRequest
	if err := s.db.First(&request, "activity_id = ?", activityID).Error; err != nil {
		t.Fatal(err)
	}
	return request
}

func TestActivityQueueFIFOAndUnrelatedOpenJob(t *testing.T) {
	s, projectID := summaryFixture(t, nil)
	// Insert in reverse age order so admission cannot rely on insertion order.
	newer := seedActivityQueueRequest(t, s, projectID, fixtureNow.Add(-time.Minute))
	older := seedActivityQueueRequest(t, s, projectID, fixtureNow.Add(-2*time.Minute))
	manual, inserted, err := s.QueueRefresh(projectID, "manager", "")
	if err != nil || !inserted {
		t.Fatalf("manual queue: inserted %v, err %v", inserted, err)
	}
	if err := s.admitActivityRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, request := range []models.ActivityNdviRequest{older, newer} {
		got := readActivityQueueRequest(t, s, request.ActivityID)
		if got.Status != models.ActivityNdviWaiting || got.JobID != "" {
			t.Fatalf("request linked to unrelated job: %+v", got)
		}
	}
	if err := s.db.Model(&models.SatelliteRefreshJob{}).Where("id = ?", manual.JobID).Update("status", JobComplete).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.admitActivityRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := readActivityQueueRequest(t, s, older.ActivityID)
	if got.Status != models.ActivityNdviQueued || got.JobID == "" {
		t.Fatalf("oldest request was not admitted: %+v", got)
	}
	if next := readActivityQueueRequest(t, s, newer.ActivityID); next.Status != models.ActivityNdviWaiting || next.JobID != "" {
		t.Fatalf("more than one request admitted per project: %+v", next)
	}
	view, inserted, err := s.QueueRefresh(projectID, "manager", "")
	if err != nil || inserted || view.JobID != got.JobID || view.ActivityID != older.ActivityID {
		t.Fatalf("manual duplicate did not identify baseline: %+v, inserted %v, err %v", view, inserted, err)
	}
	var job models.SatelliteRefreshJob
	if err := s.db.First(&job, "id = ?", got.JobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.RequestedBy != older.RequestedBy || job.Reason != "activity_baseline" {
		t.Fatalf("job lost request provenance: %+v", job)
	}
	health, err := s.HealthJobs()
	if err != nil || health.Waiting != 1 || health.Queued != 1 {
		t.Fatalf("health = %+v, err %v", health, err)
	}
}

func TestActivityQueueSharesQuotaAndSurvivesNewService(t *testing.T) {
	s, projectID := summaryFixture(t, nil)
	request := seedActivityQueueRequest(t, s, projectID, fixtureNow)
	for i := 0; i < refreshRateLimit; i++ {
		job := models.SatelliteRefreshJob{
			ID: uuid.NewString(), ProjectID: projectID, Status: JobFailed,
			CreatedAt: fixtureNow.Add(-time.Duration(i) * time.Minute), UpdatedAt: fixtureNow,
		}
		if err := s.db.Create(&job).Error; err != nil {
			t.Fatal(err)
		}
	}
	finished, err := s.ProcessRefreshJobs(context.Background())
	if err != nil || finished != 0 {
		t.Fatalf("quota-exhausted pass: finished %d, err %v", finished, err)
	}
	got := readActivityQueueRequest(t, s, request.ActivityID)
	if got.Status != models.ActivityNdviWaiting || got.JobID != "" {
		t.Fatalf("quota exhaustion lost waiting request: %+v", got)
	}

	// No in-memory pending list is carried into the replacement service.
	restarted := NewService(s.db, Options{})
	restarted.now = func() time.Time { return fixtureNow.Add(refreshRateWindow + time.Minute) }
	if err := restarted.admitActivityRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	got = readActivityQueueRequest(t, restarted, request.ActivityID)
	if got.Status != models.ActivityNdviQueued || got.JobID == "" {
		t.Fatalf("restart did not admit durable request: %+v", got)
	}
	if err := s.db.Model(&models.SatelliteRefreshJob{}).Where("id = ?", got.JobID).Update("status", JobFailed).Error; err != nil {
		t.Fatal(err)
	}
	for i := 1; i < refreshRateLimit; i++ {
		view, inserted, err := restarted.QueueRefresh(projectID, "manager", "")
		if err != nil || !inserted {
			t.Fatalf("manual slot %d: inserted %v, err %v", i, inserted, err)
		}
		if err := s.db.Model(&models.SatelliteRefreshJob{}).Where("id = ?", view.JobID).Update("status", JobComplete).Error; err != nil {
			t.Fatal(err)
		}
	}
	_, inserted, err := restarted.QueueRefresh(projectID, "manager", "")
	var rejection *domain.Error
	if inserted || !errors.As(err, &rejection) || rejection.Code != "satellite_refresh_rate_limited" {
		t.Fatalf("baseline did not consume one of six jobs: inserted %v, err %v", inserted, err)
	}
}

func TestActivityQueueSkipsArchivedProjectsWithoutBlockingOthers(t *testing.T) {
	s, projectID := summaryFixture(t, nil)
	archived := seedActivityQueueRequest(t, s, projectID, fixtureNow.Add(-time.Minute))
	var project models.ConservationProject
	var plot models.Plot
	if err := s.db.First(&project, "id = ?", projectID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.First(&plot, "project_id = ?", projectID).Error; err != nil {
		t.Fatal(err)
	}
	project.ID = uuid.NewString()
	if err := s.db.Create(&project).Error; err != nil {
		t.Fatal(err)
	}
	site := models.Site{ID: uuid.NewString(), ProjectID: project.ID, Name: "Other site", CreatedAt: fixtureNow, UpdatedAt: fixtureNow}
	if err := s.db.Create(&site).Error; err != nil {
		t.Fatal(err)
	}
	plot.ID, plot.ProjectID, plot.SiteID = uuid.NewString(), project.ID, site.ID
	if err := s.db.Create(&plot).Error; err != nil {
		t.Fatal(err)
	}
	active := seedActivityQueueRequest(t, s, project.ID, fixtureNow)
	if err := s.db.Model(&models.ConservationProject{}).Where("id = ?", projectID).Update("status", "archived").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.admitActivityRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := readActivityQueueRequest(t, s, archived.ActivityID); got.Status != models.ActivityNdviWaiting || got.JobID != "" {
		t.Fatalf("archived project admitted: %+v", got)
	}
	if got := readActivityQueueRequest(t, s, active.ActivityID); got.Status != models.ActivityNdviQueued || got.JobID == "" {
		t.Fatalf("other project blocked: %+v", got)
	}
}

func TestActivityQueueAdmissionRollsBackUnlinkedJob(t *testing.T) {
	s, projectID := summaryFixture(t, nil)
	request := seedActivityQueueRequest(t, s, projectID, fixtureNow)
	if err := s.db.Exec(`CREATE TRIGGER reject_activity_admission BEFORE UPDATE ON activity_ndvi_requests
		BEGIN SELECT RAISE(ABORT, 'test refusal'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.admitActivityRequests(context.Background()); err == nil {
		t.Fatal("request update failure was swallowed")
	}
	var count int64
	if err := s.db.Model(&models.SatelliteRefreshJob{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("unlinked job survived rollback: %d jobs", count)
	}
	if got := readActivityQueueRequest(t, s, request.ActivityID); got.Status != models.ActivityNdviWaiting || got.JobID != "" {
		t.Fatalf("request changed after rollback: %+v", got)
	}
}

func TestActivityQueueRechecksWaitingAndKeepsReapedLink(t *testing.T) {
	s, projectID := summaryFixture(t, nil)
	request := seedActivityQueueRequest(t, s, projectID, fixtureNow)
	if err := s.admitActivityRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	linked := readActivityQueueRequest(t, s, request.ActivityID)
	stuck := fixtureNow.Add(-stuckJobAfter - time.Minute)
	if err := s.db.Model(&models.SatelliteRefreshJob{}).Where("id = ?", linked.JobID).
		Updates(map[string]any{"status": JobRunning, "started_at": stuck}).Error; err != nil {
		t.Fatal(err)
	}
	finished, err := s.ProcessRefreshJobs(context.Background())
	if err != nil || finished != 0 {
		t.Fatalf("reap linked job: finished %d, err %v", finished, err)
	}
	if got := readActivityQueueRequest(t, s, request.ActivityID); got.Status != models.ActivityNdviQueued || got.JobID != linked.JobID {
		t.Fatalf("reaper reset linked request: %+v", got)
	}
	view, err := s.RefreshJob(projectID, linked.JobID)
	if err != nil || view.Status != JobFailed || view.ActivityID != request.ActivityID {
		t.Fatalf("failed baseline not identifiable: %+v, err %v", view, err)
	}
	// Simulate an admission snapshot read before the other poller admitted it.
	err = s.db.Transaction(func(tx *gorm.DB) error {
		view, inserted, err := s.queueRefresh(tx, projectID, "", "activity_baseline", &request)
		if inserted || view.JobID != "" {
			t.Errorf("stale waiting snapshot queued another job: %+v, inserted %v", view, inserted)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestActivityQueueConcurrentAdmissionsShareLastQuotaSlot(t *testing.T) {
	s, projectID := summaryFixture(t, nil)
	seedActivityQueueRequest(t, s, projectID, fixtureNow)
	var before models.ConservationProject
	if err := s.db.First(&before, "id = ?", projectID).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < refreshRateLimit-1; i++ {
		job := models.SatelliteRefreshJob{ID: uuid.NewString(), ProjectID: projectID, Status: JobComplete, CreatedAt: fixtureNow, UpdatedAt: fixtureNow}
		if err := s.db.Create(&job).Error; err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(automatic bool) {
			defer wg.Done()
			<-start
			if automatic {
				if err := s.admitActivityRequests(context.Background()); err != nil {
					t.Error(err)
				}
				return
			}
			view, inserted, err := s.QueueRefresh(projectID, "manager", "")
			if err != nil {
				var rejection *domain.Error
				if !errors.As(err, &rejection) || rejection.Code != "satellite_refresh_rate_limited" {
					t.Error(err)
				}
				return
			}
			if inserted {
				if err := s.db.Model(&models.SatelliteRefreshJob{}).Where("id = ?", view.JobID).Update("status", JobComplete).Error; err != nil {
					t.Error(err)
				}
			}
		}(i%2 == 0)
	}
	close(start)
	wg.Wait()
	var count int64
	if err := s.db.Model(&models.SatelliteRefreshJob{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != refreshRateLimit {
		t.Fatalf("concurrent requests overspent quota: %d jobs", count)
	}
	var after models.ConservationProject
	if err := s.db.First(&after, "id = ?", projectID).Error; err != nil {
		t.Fatal(err)
	}
	if !before.UpdatedAt.Equal(after.UpdatedAt) {
		t.Fatalf("queue lock changed project UpdatedAt: %s -> %s", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestActivityQueueObservationView(t *testing.T) {
	view := refreshViewOf(models.SatelliteRefreshJob{
		ID: "job", ActivityID: "activity", ObservationID: "observation", Status: JobComplete,
	})
	if view.ActivityID != "activity" || view.ObservationID != "observation" || view.SummaryID != "" {
		t.Fatalf("baseline envelope = %+v", view)
	}
}
