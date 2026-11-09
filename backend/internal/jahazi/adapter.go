package jahazi

import (
	"github.com/baharilink/baharimrv/internal/models"
)

// MatcherAdapter wraps the Jahazi Service to implement the mrv.RestorationMatcher
// interface. This avoids a circular dependency between the mrv and jahazi packages.
type MatcherAdapter struct {
	service *Service
}

// NewMatcherAdapter returns an adapter that the mrv service can use for
// activity matching without importing the jahazi package directly.
func NewMatcherAdapter(service *Service) *MatcherAdapter {
	return &MatcherAdapter{service: service}
}

// MatchActivity implements mrv.RestorationMatcher. It delegates to the Jahazi
// service's matching engine. When the integration is disabled or no open claim
// matches, it returns nil (no error) — the activity is treated as normal.
func (a *MatcherAdapter) MatchActivity(activity models.ConservationActivity, measurements map[string]any) error {
	if a.service == nil || !a.service.Enabled() {
		return nil
	}
	_, err := a.service.MatchActivity(activity, measurements)
	return err
}
