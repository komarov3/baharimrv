package satellite

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The job lifecycle. A job is queued by a request, claimed by a poller, and finished
// exactly once; the database holds the state so a restart cannot strand a project.
const (
	JobQueued   = "queued"
	JobRunning  = "running"
	JobComplete = "complete"
	JobFailed   = "failed"
)

const (
	// refreshRateLimit is spec section 7.3: six queued computations per project per
	// hour. A tap that finds an open job is answered from the database and never
	// reaches the worker, so it is not what this limit is protecting.
	refreshRateLimit  = 6
	refreshRateWindow = time.Hour
	refreshPoll       = 15 * time.Second

	// jobAttempts and jobRetryDelay implement "retried by the job" from spec section
	// 7.4. Only a provider fault is retried: a missing project or a refused write will
	// be refused again, and retrying it only delays the answer.
	jobAttempts   = 2
	jobRetryDelay = 10 * time.Second

	// jobTimeout bounds one job. A compute makes at most four worker calls and the
	// seam allows 180 s each, so two attempts fit inside this with room to spare.
	jobTimeout = 30 * time.Minute
	// stuckJobAfter is when a running job is presumed dead rather than slow. It is
	// longer than jobTimeout so a live computation is never reaped, and it exists
	// because a process that died mid-run would otherwise hold the single-flight lock
	// forever and lock the project out of refresh.
	stuckJobAfter = 40 * time.Minute
)

// openJobPredicate is the arbiter for the partial unique index. It is written as
// literals because PostgreSQL infers a partial index by matching this predicate
// against the index's own, and the two have to be the same expression. The values are
// this package's constants, not input.
var openJobPredicate = clause.Expr{SQL: fmt.Sprintf("status IN ('%s','%s')", JobQueued, JobRunning)}

// RefreshView is the job envelope from spec section 7.3. The error is a sentence this
// package wrote for a client, never the worker's or a catalog's own text, which can
// carry internal addresses.
type RefreshView struct {
	JobID      string     `json:"jobId"`
	Status     string     `json:"status"`
	SummaryID  string     `json:"summaryId,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

func refreshViewOf(job models.SatelliteRefreshJob) RefreshView {
	return RefreshView{
		JobID: job.ID, Status: job.Status, SummaryID: job.SummaryID, Error: job.Error,
		StartedAt: job.StartedAt, FinishedAt: job.FinishedAt,
	}
}

// QueueRefresh records one computation request. The second return value reports
// whether this request became a job: false means one was already open, and the view is
// that job, which is what the caller shows instead of a second spinner.
func (s *Service) QueueRefresh(projectID, requestedBy, reason string) (RefreshView, bool, error) {
	var recent int64
	if err := s.db.Model(&models.SatelliteRefreshJob{}).
		Where("project_id = ? AND created_at >= ?", projectID, s.now().UTC().Add(-refreshRateWindow)).
		Count(&recent).Error; err != nil {
		return RefreshView{}, false, fmt.Errorf("count recent refreshes: %w", err)
	}
	if recent >= refreshRateLimit {
		return RefreshView{}, false, &domain.Error{
			Kind: domain.Kind{Status: http.StatusTooManyRequests},
			Code: "satellite_refresh_rate_limited",
			Message: fmt.Sprintf("This project has already requested %d imagery refreshes in the last hour. Try again later.",
				refreshRateLimit),
		}
	}

	now := s.now().UTC()
	job := models.SatelliteRefreshJob{
		ID: uuid.NewString(), ProjectID: projectID, Status: JobQueued,
		RequestedBy: requestedBy, Reason: reason, CreatedAt: now, UpdatedAt: now,
	}
	result := s.db.Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "project_id"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{openJobPredicate}},
		DoNothing:   true,
	}).Create(&job)
	if result.Error != nil {
		return RefreshView{}, false, fmt.Errorf("queue refresh: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return refreshViewOf(job), true, nil
	}

	open, ok, err := s.OpenRefreshJob(projectID)
	if err != nil {
		return RefreshView{}, false, err
	}
	if !ok {
		// The job that blocked this one finished between the insert and the read, so
		// there is nothing to point at. Saying so beats returning an empty job id.
		return RefreshView{}, false, domain.Reject("the previous refresh finished as this one was requested; try again")
	}
	return open, false, nil
}

// OpenRefreshJob is the job holding a project's single-flight lock, if any.
func (s *Service) OpenRefreshJob(projectID string) (RefreshView, bool, error) {
	var job models.SatelliteRefreshJob
	err := s.db.Where("project_id = ? AND status IN (?,?)", projectID, JobQueued, JobRunning).
		Order("created_at").Order("id").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RefreshView{}, false, nil
	}
	if err != nil {
		return RefreshView{}, false, err
	}
	return refreshViewOf(job), true, nil
}

// RefreshJob reads one job, scoped to the project so a member of one project cannot
// watch another's queue by guessing an id.
func (s *Service) RefreshJob(projectID, jobID string) (RefreshView, error) {
	var job models.SatelliteRefreshJob
	if err := s.db.Where("id = ? AND project_id = ?", jobID, projectID).First(&job).Error; err != nil {
		return RefreshView{}, fmt.Errorf("refresh job %s: %w", jobID, err)
	}
	return refreshViewOf(job), nil
}

// RunRefreshLoop drains the queue until ctx is cancelled. It runs one pass immediately
// so jobs queued before a restart are not left waiting for the first tick.
func (s *Service) RunRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(refreshPoll)
	defer ticker.Stop()
	for {
		if _, err := s.ProcessRefreshJobs(ctx); err != nil && ctx.Err() == nil {
			slog.Error("satellite refresh pass failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessRefreshJobs runs one pass: reap jobs whose process died, then claim and
// compute whatever is queued. It returns how many jobs it finished, so a caller can
// drive a single pass without waiting on a ticker.
func (s *Service) ProcessRefreshJobs(ctx context.Context) (int, error) {
	if err := s.reapStuckJobs(ctx); err != nil {
		return 0, err
	}
	finished := 0
	for ctx.Err() == nil {
		job, ok, err := s.claimJob(ctx)
		if err != nil {
			return finished, err
		}
		if !ok {
			return finished, nil
		}
		s.runJob(ctx, job)
		finished++
	}
	return finished, ctx.Err()
}

// claimJob takes the oldest queued job, but only if it is still queued when the write
// lands. Two pollers can select the same row; the conditional update is what decides
// which one runs it, and the loser ends its pass rather than spinning on a row another
// process owns.
func (s *Service) claimJob(ctx context.Context) (models.SatelliteRefreshJob, bool, error) {
	var job models.SatelliteRefreshJob
	err := s.db.WithContext(ctx).Where("status = ?", JobQueued).
		Order("created_at").Order("id").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.SatelliteRefreshJob{}, false, nil
	}
	if err != nil {
		return models.SatelliteRefreshJob{}, false, err
	}
	now := s.now().UTC()
	result := s.db.WithContext(ctx).Model(&models.SatelliteRefreshJob{}).
		Where("id = ? AND status = ?", job.ID, JobQueued).
		Updates(map[string]any{"status": JobRunning, "started_at": now, "updated_at": now})
	if result.Error != nil {
		return models.SatelliteRefreshJob{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return models.SatelliteRefreshJob{}, false, nil
	}
	job.Status = JobRunning
	job.StartedAt = &now
	job.UpdatedAt = now
	return job, true, nil
}

func (s *Service) runJob(ctx context.Context, job models.SatelliteRefreshJob) {
	runCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	var summary models.ProjectNdviSummary
	var err error
	for attempt := 1; attempt <= jobAttempts; attempt++ {
		summary, err = s.ComputeProjectSummary(runCtx, job.ProjectID, DefaultWindow)
		if err == nil || !errors.Is(err, ErrProviderFailed) {
			break
		}
		if attempt == jobAttempts || !waitOut(runCtx, s.retryDelay*time.Duration(attempt)) {
			break
		}
	}

	now := s.now().UTC()
	updates := map[string]any{"finished_at": now, "updated_at": now}
	if err != nil {
		updates["status"] = JobFailed
		updates["error"] = jobFaultText(err)
		slog.Error("satellite refresh failed", "job", job.ID, "project", job.ProjectID, "error", err)
	} else {
		updates["status"] = JobComplete
		updates["summary_id"] = summary.ID
		// Best-effort tile render. A summary without tiles still stands on its
		// number; the overlay omits tileBaseUrl and the proxy answers 410 until
		// the next refresh paints pixels for the new scene.
		if renderErr := s.RenderActiveScene(runCtx, job.ProjectID); renderErr != nil {
			slog.Warn("satellite refresh did not paint tiles", "job", job.ID, "project", job.ProjectID, "error", renderErr)
		}
	}
	if err := s.db.Model(&models.SatelliteRefreshJob{}).Where("id = ?", job.ID).Updates(updates).Error; err != nil {
		// The computation happened but its outcome is not recorded. The reaper frees
		// the lock later, and the summary row is still readable on its own.
		slog.Error("store satellite refresh outcome", "job", job.ID, "error", err)
	}
}

// waitOut reports whether the delay elapsed, and false when the caller was cancelled.
func waitOut(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// jobFaultText is the whole of what a client is told about a failure. The detail goes
// to the log, because a worker's error can name an internal host and a catalog's can
// name a query, and neither belongs on a device.
func jobFaultText(err error) string {
	switch {
	case errors.Is(err, ErrProviderFailed):
		return "The imagery provider did not answer. Request a refresh to try again."
	case errors.Is(err, gorm.ErrRecordNotFound):
		return "The project or its methodology is no longer registered."
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "The computation ran out of time. Request a refresh to try again."
	default:
		return "The computation could not be completed."
	}
}

const errInterrupted = "The computation was interrupted before it finished. Request a refresh to try again."

// reapStuckJobs frees the lock a dead process left behind. Without it a restart mid-run
// would leave the project permanently unable to refresh, because the partial unique
// index still sees a running job.
func (s *Service) reapStuckJobs(ctx context.Context) error {
	now := s.now().UTC()
	return s.db.WithContext(ctx).Model(&models.SatelliteRefreshJob{}).
		Where("status = ? AND started_at < ?", JobRunning, now.Add(-stuckJobAfter)).
		Updates(map[string]any{
			"status": JobFailed, "error": errInterrupted,
			"finished_at": now, "updated_at": now,
		}).Error
}
