package satellite

import (
	"context"
	"errors"
	"fmt"

	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"gorm.io/gorm"
)

// admitActivityRequests tries the oldest waiting request for each project. An
// occupied project or exhausted quota leaves that request (and its successors)
// waiting for a later pass, including after a process restart.
func (s *Service) admitActivityRequests(ctx context.Context) error {
	var waiting []models.ActivityNdviRequest
	if err := s.db.WithContext(ctx).Select("activity_id", "project_id").
		Where("status = ?", models.ActivityNdviWaiting).
		Order("created_at").Order("activity_id").Find(&waiting).Error; err != nil {
		return fmt.Errorf("list waiting activity baselines: %w", err)
	}
	seen := make(map[string]bool)
	for _, request := range waiting {
		if seen[request.ProjectID] {
			continue
		}
		seen[request.ProjectID] = true
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			view, inserted, err := s.queueRefresh(tx, request.ProjectID, "", "activity_baseline", &request)
			if err != nil {
				return err
			}
			if !inserted {
				// In particular, never link this request to an unrelated open job.
				return nil
			}
			result := tx.Model(&models.ActivityNdviRequest{}).
				Where("activity_id = ? AND status = ?", request.ActivityID, models.ActivityNdviWaiting).
				Updates(map[string]any{
					"job_id": view.JobID, "status": models.ActivityNdviQueued, "updated_at": s.now().UTC(),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("activity baseline request changed during admission")
			}
			return nil
		})
		if err != nil {
			var rejection *domain.Error
			if errors.As(err, &rejection) && rejection.Code == "satellite_refresh_rate_limited" {
				continue
			}
			return fmt.Errorf("admit activity baseline %s: %w", request.ActivityID, err)
		}
	}
	return nil
}
