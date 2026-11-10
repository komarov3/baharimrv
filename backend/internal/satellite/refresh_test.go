package satellite

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestRefreshIsSingleFlightPerProject(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	first, queued, err := service.QueueRefresh(projectID, "manager-1", "first look")
	if err != nil || !queued {
		t.Fatalf("queue: queued %v err %v", queued, err)
	}
	if first.Status != JobQueued || first.JobID == "" || first.StartedAt != nil {
		t.Errorf("queued job = %+v", first)
	}

	second, queued, err := service.QueueRefresh(projectID, "manager-1", "impatient tap")
	if err != nil {
		t.Fatalf("second queue: %v", err)
	}
	if queued {
		t.Errorf("a second job was stacked on an open one")
	}
	if second.JobID != first.JobID {
		t.Errorf("the conflict answered with %s instead of the open job %s", second.JobID, first.JobID)
	}

	finished, err := service.ProcessRefreshJobs(context.Background())
	if err != nil || finished != 1 {
		t.Fatalf("process: finished %d err %v", finished, err)
	}
	third, queued, err := service.QueueRefresh(projectID, "manager-1", "")
	if err != nil || !queued {
		t.Fatalf("queue after completion: queued %v err %v", queued, err)
	}
	if third.JobID == first.JobID {
		t.Errorf("the lock was never freed")
	}
}

func TestRefreshRunsTheComputationAndRecordsIt(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	job, _, err := service.QueueRefresh(projectID, "manager-1", "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if _, err := service.ProcessRefreshJobs(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	view, err := service.RefreshJob(projectID, job.JobID)
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	if view.Status != JobComplete || view.SummaryID == "" || view.Error != "" {
		t.Fatalf("finished job = %+v", view)
	}
	if view.StartedAt == nil || view.FinishedAt == nil || view.FinishedAt.Before(*view.StartedAt) {
		t.Errorf("job timings = %+v", view)
	}
	summary, ok, err := service.LatestSummary(projectID)
	if err != nil || !ok || summary.ID != view.SummaryID {
		t.Fatalf("the job's summary is not the project's: ok %v err %v", ok, err)
	}
	if summary.Quality != QualitySynthetic || summary.AdapterMode != ModeSandbox {
		t.Errorf("the job wrote an unlabelled figure: %+v", summary)
	}
}

func TestRefreshRetriesAProviderFaultThenFailsHonestly(t *testing.T) {
	provider := &stubProvider{currentErr: errors.New("Get \"http://10.0.0.7:8000/v1/scene\": dial tcp: connection refused")}
	service, projectID := summaryFixture(t, provider)
	service.retryDelay = time.Millisecond
	job, _, err := service.QueueRefresh(projectID, "manager-1", "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if _, err := service.ProcessRefreshJobs(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	if provider.reads != jobAttempts {
		t.Errorf("provider calls = %d, want %d attempts", provider.reads, jobAttempts)
	}
	view, err := service.RefreshJob(projectID, job.JobID)
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	if view.Status != JobFailed || view.SummaryID != "" {
		t.Errorf("failed job = %+v", view)
	}
	if view.Error != jobFaultText(ErrProviderFailed) {
		t.Errorf("job error = %q", view.Error)
	}
	if strings.Contains(view.Error, "10.0.0.7") || strings.Contains(view.Error, "dial") {
		t.Errorf("the worker's own text reached the client: %q", view.Error)
	}
	if rows := storedSummaries(t, service, projectID); len(rows) != 0 {
		t.Errorf("an outage was written as a period: %d rows", len(rows))
	}
	// The lock is free again, so the manager can ask once more.
	if _, queued, err := service.QueueRefresh(projectID, "manager-1", ""); err != nil || !queued {
		t.Fatalf("requeue after failure: queued %v err %v", queued, err)
	}
}

func TestRefreshDoesNotRetryARefusedProject(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	service.retryDelay = time.Millisecond
	job := models.SatelliteRefreshJob{
		ID: uuid.NewString(), ProjectID: uuid.NewString(), Status: JobQueued,
		CreatedAt: fixtureNow, UpdatedAt: fixtureNow,
	}
	if err := service.db.Create(&job).Error; err != nil {
		t.Fatalf("seed orphan job: %v", err)
	}
	if _, err := service.ProcessRefreshJobs(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	view, err := service.RefreshJob(job.ProjectID, job.ID)
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	if view.Status != JobFailed || view.Error != jobFaultText(gorm.ErrRecordNotFound) {
		t.Errorf("orphan job = %+v", view)
	}
	if _, ok, _ := service.LatestSummary(projectID); ok {
		t.Errorf("a summary appeared for the wrong project")
	}
}

func TestRefreshRateLimitIsSixAnHour(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	for i := 0; i < refreshRateLimit; i++ {
		row := models.SatelliteRefreshJob{
			ID: uuid.NewString(), ProjectID: projectID, Status: JobComplete,
			CreatedAt: fixtureNow.Add(-time.Duration(i) * time.Minute), UpdatedAt: fixtureNow,
		}
		if err := service.db.Create(&row).Error; err != nil {
			t.Fatalf("seed job %d: %v", i, err)
		}
	}
	_, queued, err := service.QueueRefresh(projectID, "manager-1", "")
	if queued {
		t.Errorf("a seventh refresh was queued inside the hour")
	}
	var rejection *domain.Error
	if !errors.As(err, &rejection) {
		t.Fatalf("err = %v, want a client-facing rejection", err)
	}
	if rejection.Kind.Status != http.StatusTooManyRequests || rejection.Code != "satellite_refresh_rate_limited" {
		t.Errorf("rejection = %+v", rejection)
	}

	// An hour later the same project may ask again, and another project was never
	// limited by this one's traffic.
	service.now = func() time.Time { return fixtureNow.Add(refreshRateWindow + time.Minute) }
	if _, queued, err := service.QueueRefresh(projectID, "manager-1", ""); err != nil || !queued {
		t.Fatalf("queue after the window: queued %v err %v", queued, err)
	}
}

func TestAJobLeftRunningByADeadProcessIsReaped(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	stuck := fixtureNow.Add(-stuckJobAfter - time.Minute)
	dead := models.SatelliteRefreshJob{
		ID: uuid.NewString(), ProjectID: projectID, Status: JobRunning,
		StartedAt: &stuck, CreatedAt: stuck, UpdatedAt: stuck,
	}
	if err := service.db.Create(&dead).Error; err != nil {
		t.Fatalf("seed stuck job: %v", err)
	}
	if _, queued, err := service.QueueRefresh(projectID, "manager-1", ""); err != nil || queued {
		t.Fatalf("the dead job still holds the lock: queued %v err %v", queued, err)
	}
	finished, err := service.ProcessRefreshJobs(context.Background())
	if err != nil || finished != 0 {
		t.Fatalf("process: finished %d err %v", finished, err)
	}
	view, err := service.RefreshJob(projectID, dead.ID)
	if err != nil {
		t.Fatalf("read reaped job: %v", err)
	}
	if view.Status != JobFailed || view.Error != errInterrupted || view.FinishedAt == nil {
		t.Errorf("reaped job = %+v", view)
	}
	if _, queued, err := service.QueueRefresh(projectID, "manager-1", ""); err != nil || !queued {
		t.Fatalf("queue after reaping: queued %v err %v", queued, err)
	}
}

func TestASlowJobIsNotReaped(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	started := fixtureNow.Add(-jobTimeout)
	live := models.SatelliteRefreshJob{
		ID: uuid.NewString(), ProjectID: projectID, Status: JobRunning,
		StartedAt: &started, CreatedAt: started, UpdatedAt: started,
	}
	if err := service.db.Create(&live).Error; err != nil {
		t.Fatalf("seed running job: %v", err)
	}
	if _, err := service.ProcessRefreshJobs(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	view, err := service.RefreshJob(projectID, live.ID)
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	if view.Status != JobRunning {
		t.Errorf("a job inside its own timeout was reaped: %+v", view)
	}
}

func TestAJobCannotBeReadFromAnotherProject(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	job, _, err := service.QueueRefresh(projectID, "manager-1", "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if _, err := service.RefreshJob(uuid.NewString(), job.JobID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("err = %v, want the job to be invisible outside its project", err)
	}
}

func TestAnEmptyQueueIsACleanPass(t *testing.T) {
	service, _ := summaryFixture(t, nil)
	finished, err := service.ProcessRefreshJobs(context.Background())
	if err != nil || finished != 0 {
		t.Fatalf("process: finished %d err %v", finished, err)
	}
}

func TestTheLoopStopsWhenItsContextIsCancelled(t *testing.T) {
	service, projectID := summaryFixture(t, nil)
	job, _, err := service.QueueRefresh(projectID, "manager-1", "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		service.RunRefreshLoop(ctx)
		close(done)
	}()
	// The first pass runs before the first tick, so the job is computed without waiting
	// out the poll interval.
	deadline := time.Now().Add(10 * time.Second)
	for {
		view, err := service.RefreshJob(projectID, job.JobID)
		if err != nil {
			t.Fatalf("read job: %v", err)
		}
		if view.Status == JobComplete {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the first pass never ran: %+v", view)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("the loop ignored its cancellation")
	}
}
