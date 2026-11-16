package mrv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/geo"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/money"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/baharilink/baharimrv/internal/satellite"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Methodology holds the estimation parameters a project's CO2e figure depends on.
// Without them the dashboard reports the metric as unavailable instead of guessing.
type EstimationParams struct {
	TonnesCO2PerTree  string `json:"tonnes_co2_per_tree"`
	Source            string `json:"source"`
	SurvivalThreshold int    `json:"survival_threshold_pct"`
	PeriodYears       int    `json:"period_years"`
}

type Metric struct {
	Key               string `json:"key"`
	Label             string `json:"label"`
	Value             string `json:"value"`
	Unit              string `json:"unit,omitempty"`
	Status            string `json:"status"`
	MethodologyPeriod string `json:"methodologyPeriod,omitempty"`
}

// PlotView is one measured parcel on the dashboard map and site table. Area and
// activity type are included because the wireframe's site manager lists them per
// row; both are read straight from the registered plot and activity.
type PlotView struct {
	ID           string          `json:"id"`
	Label        string          `json:"label"`
	Latitude     float64         `json:"latitude"`
	Longitude    float64         `json:"longitude"`
	AreaHectares string          `json:"areaHectares"`
	ActivityType string          `json:"activityType,omitempty"`
	Status       string          `json:"status"`
	ChainState   string          `json:"chainState"`
	NDVI         *float64        `json:"ndvi,omitempty"`
	Boundary     json.RawMessage `json:"boundary,omitempty"`
	GpsAccuracyM *float64        `json:"gpsAccuracyM,omitempty"`
}

type Alert struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	ActionLabel string `json:"actionLabel,omitempty"`
	ActionHref  string `json:"actionHref,omitempty"`
}

type DashboardProject struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	CommunityGroup    string          `json:"communityGroup,omitempty"`
	Species           string          `json:"species,omitempty"`
	ConservationType  string          `json:"conservationType,omitempty"`
	SiteCount         int             `json:"siteCount"`
	Methodology       string          `json:"methodology"`
	MethodologyID     string          `json:"methodologyId,omitempty"`
	MethodologyPeriod string          `json:"methodologyPeriod,omitempty"`
	Status            string          `json:"status"`
	CommunityShareBPS int             `json:"communityShareBasisPoints"`
	Boundary          json.RawMessage `json:"boundary,omitempty"`
}

type NDVIState struct {
	Status     string     `json:"status"`
	Source     string     `json:"source,omitempty"`
	AcquiredAt *time.Time `json:"acquiredAt,omitempty"`
	// Fields below are only present when Status is "available" or "pending". They
	// describe the project-level figure from project_ndvi_summaries rather than the
	// per-activity cherry-pick the old headline used. When no summary has been
	// computed yet (status unavailable), they stay nil.
	MeanNDVI    *float64 `json:"meanNdvi,omitempty"`
	ChangePct   *float64 `json:"changePct,omitempty"`
	Quality     string   `json:"quality,omitempty"`
	CoveragePct float64  `json:"coveragePct,omitempty"`
	AdapterMode string   `json:"adapterMode,omitempty"`
	SceneID     string   `json:"sceneId,omitempty"`
	Provider    string   `json:"provider,omitempty"`
}

// ndviStatusFromQuality turns the satellite quality vocabulary into a dashboard
// status. "complete" and "cloudy" are still a number a manager can act on; "partial"
// and "synthetic" both read as pending until the figure improves; "unavailable" is
// the honest "nothing is here". The mapping is closed to new vocabulary: an unknown
// quality becomes unavailable rather than available.
func ndviStatusFromQuality(quality string) string {
	switch quality {
	case "complete", "cloudy":
		return "available"
	case "partial", "synthetic":
		return "pending"
	case "unavailable":
		return "unavailable"
	default:
		return "unavailable"
	}
}

// singlePlotMarkerNDVI decides whether the project figure may be shown on a plot
// marker. It may only when the project is exactly one plot, the summary measured
// that one plot live, and the figure is available. A single-plot summary is that
// plot's own measurement, so attributing it to the marker is honest. A multi-plot
// aggregate or a sandbox synthetic is never one marker's measurement, so it stays
// on the headline only and the marker keeps its per-activity answer.
func singlePlotMarkerNDVI(change satellite.ChangeView, plotCount int, err error) *float64 {
	if err != nil || plotCount != 1 || change.PlotCount != 1 ||
		change.MeanNDVI == nil || change.AdapterMode != satellite.ModeLive ||
		ndviStatusFromQuality(change.Quality) != "available" {
		return nil
	}
	value := *change.MeanNDVI
	return &value
}

type Dashboard struct {
	Project DashboardProject `json:"project"`
	Plots   []PlotView       `json:"plots"`
	NDVI    NDVIState        `json:"ndvi"`
	// ChainMode qualifies every chainState below: "sandbox" means no public
	// ledger holds the commitment, however confirmed the local readback is.
	ChainMode string   `json:"chainMode"`
	Metrics   []Metric `json:"metrics"`
	Alerts    []Alert  `json:"alerts"`
}

// AssertProjectAccess enforces the role rules from SPEC section 4 server-side.
// Hiding a control is never the authorisation.
func (s *Service) AssertProjectAccess(userID, projectID string) error {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return ErrForbidden
	}
	if user.Role == models.RoleAdmin {
		return nil
	}
	var managed int64
	if err := s.db.Model(&models.ConservationProject{}).Where("id = ? AND manager_id = ?", projectID, userID).Count(&managed).Error; err != nil {
		return ErrForbidden
	}
	if managed > 0 {
		return nil
	}
	switch user.Role {
	case models.RoleMonitor:
		var assigned int64
		if err := s.db.Model(&models.Plot{}).Where("project_id = ? AND assigned_monitor_id = ?", projectID, userID).Count(&assigned).Error; err != nil {
			return ErrForbidden
		}
		if assigned > 0 {
			return nil
		}
	case models.RoleVerifier:
		var requested int64
		if err := s.db.Model(&models.VerificationRequest{}).
			Joins("JOIN conservation_activities ON conservation_activities.id = verification_requests.activity_id").
			Where("conservation_activities.project_id = ? AND verification_requests.verifier_id = ?", projectID, userID).
			Count(&requested).Error; err != nil {
			return ErrForbidden
		}
		if requested > 0 {
			return nil
		}
	}
	return ErrForbidden
}

// AssertProjectManager refuses anyone who is not an admin or the project's manager. It
// is narrower than AssertProjectAccess, which also allows monitors and verifiers, and
// it is what a write that changes project state (a refresh, a new plot, a rename) has
// to check before it acts.
func (s *Service) AssertProjectManager(userID, projectID string) error {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return ErrForbidden
	}
	if user.Role == models.RoleAdmin {
		return nil
	}
	if user.Role != models.RoleManager {
		return ErrForbidden
	}
	var managed int64
	if err := s.db.Model(&models.ConservationProject{}).
		Where("id = ? AND manager_id = ?", projectID, userID).Count(&managed).Error; err != nil {
		return ErrForbidden
	}
	if managed == 0 {
		return ErrForbidden
	}
	return nil
}

// ProjectsForUser lists what the signed-in account may open on the dashboard.
func (s *Service) ProjectsForUser(userID string) ([]models.ConservationProject, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, ErrForbidden
	}
	var projects []models.ConservationProject
	query := s.db.Order("name asc")
	switch user.Role {
	case models.RoleAdmin:
		err := query.Find(&projects).Error
		return projects, err
	case models.RoleManager:
		err := query.Where("manager_id = ?", userID).Find(&projects).Error
		return projects, err
	case models.RoleMonitor:
		var projectIDs []string
		if err := s.db.Model(&models.Plot{}).Where("assigned_monitor_id = ?", userID).Distinct().Pluck("project_id", &projectIDs).Error; err != nil {
			return nil, err
		}
		if len(projectIDs) == 0 {
			return nil, nil
		}
		err := query.Where("id IN ?", projectIDs).Find(&projects).Error
		return projects, err
	case models.RoleVerifier:
		var projectIDs []string
		err := s.db.Model(&models.ConservationActivity{}).
			Joins("JOIN verification_requests ON verification_requests.activity_id = conservation_activities.id").
			Where("verification_requests.verifier_id = ?", userID).Distinct().
			Pluck("conservation_activities.project_id", &projectIDs).Error
		if err != nil {
			return nil, err
		}
		if len(projectIDs) == 0 {
			return nil, nil
		}
		err = query.Where("id IN ?", projectIDs).Find(&projects).Error
		return projects, err
	default:
		return nil, ErrForbidden
	}
}

// ProjectRow is one line of the conservation project table. Every figure is summed
// from stored monitoring records; nothing here promotes a measured count to a
// verified one, and verification state is reported separately from activity status.
type ProjectRow struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	CommunityGroup    string `json:"communityGroup"`
	Site              string `json:"site"`
	Species           string `json:"species"`
	ConservationType  string `json:"conservationType,omitempty"`
	ActivityType      string `json:"activityType,omitempty"`
	Status            string `json:"status"`
	AreaHectares      string `json:"areaHectares"`
	SeedlingsPlanted  int    `json:"seedlingsPlanted"`
	SeedlingsSurvived int    `json:"seedlingsSurvived"`
	SurvivalRate      string `json:"survivalRate"`
	LatestMrvAt       string `json:"latestMrvAt,omitempty"`
	VerificationState string `json:"verificationState"`
	ProofAvailable    bool   `json:"proofAvailable"`
	SandboxDemo       bool   `json:"sandboxDemo"`
	ManagerName       string `json:"managerName,omitempty"`
	IntegrationNote   string `json:"integrationNote,omitempty"`
}

// ProjectTable is the C1 project-admin read model: the rows plus the counts the
// KPI strip shows.
type ProjectTable struct {
	Rows    []ProjectRow `json:"rows"`
	Summary struct {
		Total               int `json:"total"`
		Active              int `json:"active"`
		PendingVerification int `json:"pendingVerification"`
		AchievedUnverified  int `json:"achievedUnverified"`
	} `json:"summary"`
}

// ProjectsTable rolls every accessible project up for the table view.
func (s *Service) ProjectsTable(userID string) (ProjectTable, error) {
	projects, err := s.ProjectsForUser(userID)
	if err != nil {
		return ProjectTable{}, err
	}
	table := ProjectTable{Rows: make([]ProjectRow, 0, len(projects))}
	for _, project := range projects {
		row, err := s.projectRow(project)
		if err != nil {
			return ProjectTable{}, err
		}
		table.Rows = append(table.Rows, row)
		table.Summary.Total++
		if project.Status == "active" {
			table.Summary.Active++
		}
		switch row.VerificationState {
		case "awaiting_verification":
			table.Summary.PendingVerification++
		case "achieved_unverified":
			table.Summary.AchievedUnverified++
		}
	}
	return table, nil
}

func (s *Service) projectRow(project models.ConservationProject) (ProjectRow, error) {
	var sites []models.Site
	if err := s.db.Where("project_id = ?", project.ID).Find(&sites).Error; err != nil {
		return ProjectRow{}, err
	}
	var plots []models.Plot
	if err := s.db.Where("project_id = ?", project.ID).Find(&plots).Error; err != nil {
		return ProjectRow{}, err
	}
	var activities []models.ConservationActivity
	if err := s.db.Where("project_id = ?", project.ID).Find(&activities).Error; err != nil {
		return ProjectRow{}, err
	}
	activityIDs := make([]string, 0, len(activities))
	for _, activity := range activities {
		activityIDs = append(activityIDs, activity.ID)
	}

	row := ProjectRow{
		ID: project.ID, Name: project.Name, CommunityGroup: project.CommunityGroup,
		Species: project.Species, ConservationType: project.ConservationType, Status: project.Status, SandboxDemo: project.SandboxDemo,
		VerificationState: "not_started", AreaHectares: "0.000000", SurvivalRate: "0.00",
	}
	if len(sites) > 0 {
		names := make([]string, 0, len(sites))
		for _, site := range sites {
			names = append(names, site.Name)
		}
		row.Site = strings.Join(names, ", ")
	}
	totalArea := decimal.Zero
	for _, plot := range plots {
		area, err := money.ParseQuantity(plot.AreaHectares)
		if err == nil {
			totalArea = totalArea.Add(area)
		}
	}
	row.AreaHectares = money.Quantity(totalArea)

	if len(activityIDs) > 0 {
		var records []models.MonitoringRecord
		if err := s.db.Where("activity_id IN ?", activityIDs).Find(&records).Error; err != nil {
			return row, err
		}
		var latest time.Time
		for _, record := range records {
			row.SeedlingsPlanted += record.PlantedCount
			row.SeedlingsSurvived += record.AliveCount
			if record.DeviceTimestamp.After(latest) {
				latest = record.DeviceTimestamp
			}
		}
		if !latest.IsZero() {
			row.LatestMrvAt = latest.UTC().Format(time.RFC3339)
		}
		if row.SeedlingsPlanted > 0 {
			rate := decimal.NewFromInt(int64(row.SeedlingsSurvived)).
				Div(decimal.NewFromInt(int64(row.SeedlingsPlanted))).
				Mul(decimal.NewFromInt(100)).RoundBank(2)
			row.SurvivalRate = rate.StringFixed(2)
		}
		// A project is only as advanced as its furthest activity. Verified requires an
		// independent decision, so a completion claim reports as achieved-unverified.
		for _, activity := range activities {
			if activity.ActivityType != "" {
				row.ActivityType = activity.ActivityType
			}
			switch activity.Status {
			case models.ActivityVerified:
				row.VerificationState = "verified"
				row.ProofAvailable = true
			case models.ActivityDisputed:
				if row.VerificationState != "verified" {
					row.VerificationState = "disputed"
				}
			case models.ActivityRejected:
				if row.VerificationState == "not_started" || row.VerificationState == "in_progress" {
					row.VerificationState = "rejected"
				}
			case models.ActivityAwaitingVerification, models.ActivityCompletionClaimed:
				if row.VerificationState == "not_started" || row.VerificationState == "in_progress" {
					row.VerificationState = "achieved_unverified"
				}
			case models.ActivityMonitoring:
				if row.VerificationState == "not_started" {
					row.VerificationState = "in_progress"
				}
			}
		}
		var proofCount int64
		if err := s.db.Model(&models.ProofDocument{}).Where("activity_id IN ? AND status IN ?",
			activityIDs, []string{models.ProofAnchored, models.ProofPending}).Count(&proofCount).Error; err != nil {
			return row, err
		}
		if proofCount > 0 {
			row.ProofAvailable = true
		}
	}
	var manager models.User
	if err := s.db.Where("id = ?", project.ManagerID).First(&manager).Error; err == nil {
		row.ManagerName = manager.DisplayName
	}
	if s.provenance.Mode() != "iota" {
		row.IntegrationNote = "Sandbox anchoring: no public chain commitment exists for this project yet."
	}
	return row, nil
}

func (s *Service) project(projectID string) (models.ConservationProject, models.Methodology, error) {
	var project models.ConservationProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return project, models.Methodology{}, ErrNotFound
	}
	var methodology models.Methodology
	if err := s.db.Where("id = ?", project.MethodologyID).First(&methodology).Error; err != nil {
		return project, methodology, ErrNotFound
	}
	return project, methodology, nil
}

// Dashboard builds C1. Every metric carries its own evidence status and measured
// values are never promoted to verified.
func (s *Service) Dashboard(projectID, userID string) (Dashboard, error) {
	if err := s.AssertProjectAccess(userID, projectID); err != nil {
		return Dashboard{}, err
	}
	project, methodology, err := s.project(projectID)
	if err != nil {
		return Dashboard{}, err
	}
	params, err := estimationParams(methodology)
	if err != nil {
		return Dashboard{}, err
	}
	var sites []models.Site
	if err := s.db.Where("project_id = ?", projectID).Find(&sites).Error; err != nil {
		return Dashboard{}, err
	}
	var plots []models.Plot
	if err := s.db.Where("project_id = ?", projectID).Order("code asc").Find(&plots).Error; err != nil {
		return Dashboard{}, err
	}
	var activities []models.ConservationActivity
	if err := s.db.Where("project_id = ?", projectID).Find(&activities).Error; err != nil {
		return Dashboard{}, err
	}
	activityByPlot := map[string]models.ConservationActivity{}
	for _, activity := range activities {
		activityByPlot[activity.PlotID] = activity
	}
	var batches []models.CreditBatch
	if err := s.db.Where("project_id = ?", projectID).Find(&batches).Error; err != nil {
		return Dashboard{}, err
	}

	dashboard := Dashboard{
		Project: DashboardProject{
			ID: project.ID, Name: project.Name, SiteCount: len(sites),
			CommunityGroup: project.CommunityGroup, Species: project.Species, ConservationType: project.ConservationType,
			Methodology:   fmt.Sprintf("%s v%s", methodology.Code, methodology.Version),
			MethodologyID: project.MethodologyID, Status: project.Status,
			CommunityShareBPS: project.CommunityShareBPS, Boundary: json.RawMessage(project.Boundary),
		},
		Plots:     make([]PlotView, 0, len(plots)),
		ChainMode: s.provenance.Mode(),
		Metrics:   []Metric{},
		Alerts:    []Alert{},
	}
	period := reportingPeriod(activities)
	dashboard.Project.MethodologyPeriod = period

	// The project figure is read once and reused: it drives the headline below and,
	// for a single-plot project, the marker's own NDVI.
	change, changeErr := s.satellite.ProjectChange(projectID)
	projectPlotNDVI := singlePlotMarkerNDVI(change, len(plots), changeErr)

	var latestNDVI *float64
	var ndviSource string
	var ndviAt *time.Time
	for _, plot := range plots {
		activity := activityByPlot[plot.ID]
		view := PlotView{ID: plot.ID, Label: plot.Code, Latitude: plot.Latitude, Longitude: plot.Longitude,
			AreaHectares: plot.AreaHectares, Status: "pending", ChainState: "not_recorded",
			Boundary: json.RawMessage(plot.Boundary), GpsAccuracyM: plot.GpsAccuracyM}
		if activity.ID != "" {
			view.ActivityType = activity.ActivityType
			view.Status = plotEvidenceState(activity)
			view.ChainState = s.chainState(activity.ID)
			// Per-plot NDVI stays derived from the activity's own corroboration: a
			// manager looking at one marker still sees what the latest read for that
			// marker said. The headline below is now the project figure, not the
			// maximum of these.
			if summary, ok := s.satellite.Latest(activity.ID); ok && summary.Status == "complete" {
				value := summary.MeanNDVI
				view.NDVI = &value
				if latestNDVI == nil || summary.MeanNDVI > *latestNDVI {
					acquired := summary.AcquiredAt
					latestNDVI = &value
					ndviAt = &acquired
					ndviSource = summary.Source
				}
			}
		}
		// A single-plot live figure is this marker's measurement even before a
		// per-activity corroboration exists, so the marker shows it rather than
		// "Unavailable". Multi-plot and sandbox figures never reach a marker here.
		if view.NDVI == nil && projectPlotNDVI != nil {
			value := *projectPlotNDVI
			view.NDVI = &value
		}
		dashboard.Plots = append(dashboard.Plots, view)
	}
	// Drive the headline from the project figure rather than the loudest plot. The
	// cherry-pick here was the one piece of arithmetic that could be honest about
	// each plot and wrong about the project.
	if changeErr == nil {
		source := change.Provider
		if source == "" {
			source = "satellite"
		}
		dashboard.NDVI = NDVIState{
			Status:      ndviStatusFromQuality(change.Quality),
			Source:      source,
			AcquiredAt:  change.CurrentAcquiredAt,
			MeanNDVI:    change.MeanNDVI,
			ChangePct:   change.ChangePct,
			Quality:     change.Quality,
			CoveragePct: change.CoveragePct,
			AdapterMode: change.AdapterMode,
			SceneID:     change.CurrentSceneID,
			Provider:    change.Provider,
		}
	} else if errors.Is(changeErr, satellite.ErrSummaryMissing) {
		// No computation yet: fall back to the per-activity answer, which at least
		// knows the sandbox has been attached, and keep the live/sandbox distinction
		// honest.
		if latestNDVI != nil {
			dashboard.NDVI = NDVIState{Status: "available", Source: ndviSource, AcquiredAt: ndviAt}
		} else if s.satellite.Mode() == "live" {
			dashboard.NDVI = NDVIState{Status: "pending"}
		} else {
			dashboard.NDVI = NDVIState{Status: "unavailable", Source: "No corroboration attached"}
		}
	} else {
		return Dashboard{}, changeErr
	}

	restored, restoredState, err := s.areaRestored(plots, activityByPlot)
	if err != nil {
		return Dashboard{}, err
	}
	dashboard.Metrics = append(dashboard.Metrics, Metric{
		Key: "area_restored", Label: "Area restored", Value: money.Quantity(restored), Unit: "ha",
		Status: restoredState, MethodologyPeriod: period,
	})

	survival, survivalState, survivalRecords := s.survivalRate(activities)
	survivalMetric := Metric{
		Key: "survival", Label: "Seedling survival", Value: survival, Unit: "%",
		Status: survivalState, MethodologyPeriod: period,
	}
	dashboard.Metrics = append(dashboard.Metrics, survivalMetric)

	co2e, co2eState := s.estimatedCO2e(activities, params)
	dashboard.Metrics = append(dashboard.Metrics, Metric{
		Key: "co2e", Label: "Estimated CO2e", Value: co2e, Unit: "tCO2e",
		Status: co2eState, MethodologyPeriod: period,
	})

	verifiedActivities := make([]models.ConservationActivity, 0, len(activities))
	for _, activity := range activities {
		if activity.Status == models.ActivityVerified {
			verifiedActivities = append(verifiedActivities, activity)
		}
	}
	verifiedCO2e, verifiedState := s.estimatedCO2e(verifiedActivities, params)
	if len(verifiedActivities) == 0 {
		verifiedCO2e, verifiedState = "0.00", "pending"
	}
	dashboard.Metrics = append(dashboard.Metrics, Metric{
		Key: "verified_co2e", Label: "Verified CO2e", Value: verifiedCO2e, Unit: "tCO2e",
		Status: verifiedState, MethodologyPeriod: period,
	})

	credits, creditsState, creditsMode := creditsIssued(batches)
	creditsMetric := Metric{
		Key: "credits", Label: "Credits issued", Value: credits, Unit: "tCO2e",
		Status: creditsState, MethodologyPeriod: period,
	}
	if creditsMode != "" {
		creditsMetric.Label = "Credits issued (" + creditsMode + ")"
	}
	dashboard.Metrics = append(dashboard.Metrics, creditsMetric)

	dashboard.Alerts = append(dashboard.Alerts, s.alerts(project, plots, activities, activityByPlot, params, survivalRecords, batches)...)
	sort.SliceStable(dashboard.Alerts, func(i, j int) bool { return dashboard.Alerts[i].Severity < dashboard.Alerts[j].Severity })
	return dashboard, nil
}

func (s *Service) areaRestored(plots []models.Plot, byPlot map[string]models.ConservationActivity) (decimal.Decimal, string, error) {
	var total decimal.Decimal
	state := "pending"
	for _, plot := range plots {
		activity := byPlot[plot.ID]
		if activity.ID == "" || activity.Status == models.ActivityDraft {
			continue
		}
		area, err := money.ParseQuantity(plot.AreaHectares)
		if err != nil {
			return total, state, fmt.Errorf("plot %s has an invalid area", plot.Code)
		}
		total = total.Add(area)
		switch activity.Status {
		case models.ActivityVerified:
			state = "verified"
		case models.ActivityCompletionClaimed, models.ActivityAwaitingVerification:
			if state != "verified" {
				state = "achieved_unverified"
			}
		default:
			if state == "pending" {
				state = "measured"
			}
		}
	}
	return total, state, nil
}

// survivalRate uses the newest record per activity, so a correction that lands
// later replaces the earlier reading instead of averaging over it.
func (s *Service) survivalRate(activities []models.ConservationActivity) (string, string, int) {
	var alive, planted decimal.Decimal
	var used int
	for _, activity := range activities {
		records, err := s.RecordsFor(activity.ID)
		if err != nil || len(records) == 0 {
			continue
		}
		latest := records[len(records)-1]
		if latest.PlantedCount <= 0 {
			continue
		}
		alive = alive.Add(decimal.NewFromInt(int64(latest.AliveCount)))
		planted = planted.Add(decimal.NewFromInt(int64(latest.PlantedCount)))
		used++
	}
	if planted.IsZero() {
		return "0.00", "pending", used
	}
	rate := alive.Mul(decimal.NewFromInt(100)).Div(planted).Round(2)
	return rate.StringFixed(2), "measured", used
}

func (s *Service) estimatedCO2e(activities []models.ConservationActivity, params EstimationParams) (string, string) {
	rate, err := decimal.NewFromString(params.TonnesCO2PerTree)
	if err != nil || rate.IsZero() {
		return "0.00", "unavailable"
	}
	var trees decimal.Decimal
	basis := "pending"
	for _, activity := range activities {
		records, err := s.RecordsFor(activity.ID)
		if err != nil || len(records) == 0 {
			continue
		}
		latest := records[len(records)-1]
		trees = trees.Add(decimal.NewFromInt(int64(latest.AliveCount)))
		switch activity.Status {
		case models.ActivityVerified:
			basis = "verified"
		case models.ActivityRejected:
			if basis == "pending" {
				basis = "rejected"
			}
		default:
			if basis == "pending" {
				basis = "estimated"
			}
		}
	}
	if trees.IsZero() {
		return "0.00", "pending"
	}
	periodYears := params.PeriodYears
	if periodYears <= 0 {
		periodYears = 1
	}
	estimate := trees.Mul(rate).Mul(decimal.NewFromInt(int64(periodYears))).Round(2)
	return estimate.StringFixed(2), basis
}

func creditsIssued(batches []models.CreditBatch) (string, string, string) {
	var total decimal.Decimal
	mode := ""
	for _, batch := range batches {
		quantity, err := decimal.NewFromString(batch.Quantity)
		if err != nil {
			continue
		}
		total = total.Add(quantity)
		mode = batch.AdapterMode
	}
	if total.IsZero() {
		return "0.000000", "pending", ""
	}
	if mode == "live" {
		return money.Quantity(total), "verified", "registry"
	}
	return money.Quantity(total), "sandbox", "sandbox registry"
}

func (s *Service) alerts(project models.ConservationProject, plots []models.Plot, activities []models.ConservationActivity, byPlot map[string]models.ConservationActivity, params EstimationParams, survivalRecords int, batches []models.CreditBatch) []Alert {
	alerts := []Alert{}
	now := s.now().UTC()

	survival, state, _ := s.survivalRate(activities)
	if state == "measured" && params.SurvivalThreshold > 0 {
		rate, err := decimal.NewFromString(survival)
		if err == nil && rate.LessThan(decimal.NewFromInt(int64(params.SurvivalThreshold))) {
			alerts = append(alerts, Alert{
				ID: project.ID + ":survival", Severity: "critical", Title: "Survival below methodology threshold",
				Detail: fmt.Sprintf("Reported survival is %s%% against a %d%% threshold for %s.", survival, params.SurvivalThreshold, project.Name),
			})
		}
	}

	missingEvidence := 0
	for _, activity := range activities {
		records, err := s.RecordsFor(activity.ID)
		if err != nil {
			continue
		}
		gap, err := s.EvidenceGap(records)
		if err != nil {
			continue
		}
		missingEvidence += gap.Missing
	}
	if missingEvidence > 0 {
		alerts = append(alerts, Alert{
			ID: project.ID + ":evidence", Severity: "warning", Title: "Evidence incomplete",
			Detail:      fmt.Sprintf("%d declared evidence files have not been uploaded for this project.", missingEvidence),
			ActionLabel: "Open field capture", ActionHref: "/app/monitor",
		})
	}

	var failed int64
	if err := s.db.Model(&models.ProvenanceEvent{}).
		Joins("JOIN conservation_activities ON conservation_activities.id = provenance_events.activity_id").
		Where("conservation_activities.project_id = ? AND provenance_events.status = ?", project.ID, provenance.StatusFailed).
		Count(&failed).Error; err == nil && failed > 0 {
		alerts = append(alerts, Alert{
			ID: project.ID + ":anchors", Severity: "critical", Title: "Failed chain anchors",
			Detail:      fmt.Sprintf("%d provenance events failed to anchor. Retry preserves the original commitment.", failed),
			ActionLabel: "Open verification", ActionHref: "/app/verification",
		})
	}

	overdue := 0
	for _, activity := range activities {
		if activity.Status == models.ActivityVerified || activity.Status == models.ActivityRejected {
			continue
		}
		records, err := s.RecordsFor(activity.ID)
		if err == nil && len(records) == 0 && activity.ReportingPeriodEnd.Before(now) {
			overdue++
		}
	}
	if overdue > 0 {
		alerts = append(alerts, Alert{
			ID: project.ID + ":overdue", Severity: "warning", Title: "Overdue monitoring",
			Detail: fmt.Sprintf("%d activities have no monitoring record for a reporting period that has closed.", overdue),
		})
	}

	var openCases int64
	if err := s.db.Model(&models.ReconciliationCase{}).
		Joins("JOIN credit_batches ON credit_batches.id = reconciliation_cases.batch_id").
		Where("credit_batches.project_id = ? AND reconciliation_cases.status = ?", project.ID, "open").
		Count(&openCases).Error; err == nil && openCases > 0 {
		alerts = append(alerts, Alert{
			ID: project.ID + ":registry", Severity: "critical", Title: "Registry mismatch",
			Detail:      fmt.Sprintf("%d reconciliation cases are open against this project's inventory.", openCases),
			ActionLabel: "Open reconciliation", ActionHref: "/app/credits",
		})
	}

	unassigned := 0
	for _, plot := range plots {
		if plot.AssignedMonitorID == "" {
			unassigned++
		}
	}
	if unassigned > 0 {
		alerts = append(alerts, Alert{
			ID: project.ID + ":plots", Severity: "info", Title: "Plots without a monitor",
			Detail: fmt.Sprintf("%d plots in this project have no assigned field monitor.", unassigned),
		})
	}

	if project.SandboxDemo {
		alerts = append(alerts, Alert{
			ID: project.ID + ":sandbox", Severity: "info", Title: "Demonstration project",
			Detail: "Chain, registry, payment and satellite values in this project are sandbox outputs and are labelled as such.",
		})
	}
	return alerts
}

func estimationParams(methodology models.Methodology) (EstimationParams, error) {
	params := EstimationParams{SurvivalThreshold: 70, PeriodYears: 1}
	if len(methodology.Estimation) == 0 || string(methodology.Estimation) == "{}" {
		return params, nil
	}
	if err := json.Unmarshal(methodology.Estimation, &params); err != nil {
		return params, domain.Invalid("methodology %s has invalid estimation parameters", methodology.Code)
	}
	if params.SurvivalThreshold <= 0 {
		params.SurvivalThreshold = 70
	}
	if params.PeriodYears <= 0 {
		params.PeriodYears = 1
	}
	return params, nil
}

func reportingPeriod(activities []models.ConservationActivity) string {
	if len(activities) == 0 {
		return ""
	}
	start := activities[0].ReportingPeriodStart
	end := activities[0].ReportingPeriodEnd
	for _, activity := range activities {
		if activity.ReportingPeriodStart.Before(start) {
			start = activity.ReportingPeriodStart
		}
		if activity.ReportingPeriodEnd.After(end) {
			end = activity.ReportingPeriodEnd
		}
	}
	return start.UTC().Format("2006-01-02") + " to " + end.UTC().Format("2006-01-02")
}

func plotEvidenceState(activity models.ConservationActivity) string {
	switch activity.Status {
	case models.ActivityVerified:
		return "verified"
	case models.ActivityRejected:
		return "rejected"
	case models.ActivityDisputed:
		return "disputed"
	case models.ActivityCompletionClaimed, models.ActivityAwaitingVerification:
		return "achieved_unverified"
	case models.ActivityMonitoring:
		return "measured"
	default:
		return "pending"
	}
}

// chainState reports the newest non-superseded provenance event for an activity.
// It is read from the ledger, never inferred from the activity's own status.
func (s *Service) chainState(activityID string) string {
	events, err := s.provenance.Events(activityID)
	if err != nil || len(events) == 0 {
		return "not_recorded"
	}
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Status != provenance.StatusSuperseded {
			return events[index].Status
		}
	}
	return provenance.StatusSuperseded
}

// CreateProject registers a conservation project with its sites and plots.
type PlotInput struct {
	Code              string          `json:"code"`
	SiteName          string          `json:"siteName"`
	Latitude          float64         `json:"latitude"`
	Longitude         float64         `json:"longitude"`
	AreaHectares      string          `json:"areaHectares"`
	AssignedMonitorID string          `json:"assignedMonitorId,omitempty"`
	Boundary          json.RawMessage `json:"boundary,omitempty"`
	GpsAccuracyM      *float64        `json:"gpsAccuracyM,omitempty"`
}

// PlotPatch is a partial edit of an existing plot. Unset fields keep their stored
// value. A present assignedMonitorId always applies, and "" clears the assignment.
// For boundary, an absent key keeps the stored outline while an explicit JSON null
// clears it.
type PlotPatch struct {
	Code              *string         `json:"code"`
	SiteName          *string         `json:"siteName"`
	Latitude          *float64        `json:"latitude"`
	Longitude         *float64        `json:"longitude"`
	AreaHectares      *string         `json:"areaHectares"`
	AssignedMonitorID *string         `json:"assignedMonitorId"`
	Boundary          json.RawMessage `json:"boundary"`
	GpsAccuracyM      *float64        `json:"gpsAccuracyM"`
}

// mergePlotPatch turns a partial edit plus the stored row into a full PlotInput.
func mergePlotPatch(plot models.Plot, patch PlotPatch) PlotInput {
	input := PlotInput{
		Code:              plot.Code,
		Latitude:          plot.Latitude,
		Longitude:         plot.Longitude,
		AreaHectares:      plot.AreaHectares,
		AssignedMonitorID: plot.AssignedMonitorID,
		Boundary:          json.RawMessage(plot.Boundary),
		GpsAccuracyM:      plot.GpsAccuracyM,
	}
	if patch.Code != nil {
		input.Code = *patch.Code
	}
	if patch.SiteName != nil {
		input.SiteName = *patch.SiteName
	}
	if patch.Latitude != nil {
		input.Latitude = *patch.Latitude
	}
	if patch.Longitude != nil {
		input.Longitude = *patch.Longitude
	}
	if patch.AreaHectares != nil {
		input.AreaHectares = *patch.AreaHectares
	}
	if patch.AssignedMonitorID != nil {
		input.AssignedMonitorID = *patch.AssignedMonitorID
	}
	if len(patch.Boundary) > 0 {
		input.Boundary = patch.Boundary
	}
	if patch.GpsAccuracyM != nil {
		input.GpsAccuracyM = patch.GpsAccuracyM
	}
	return input
}

// resolvedPlot is the geometry a plot is stored with. A surveyed boundary overrides the
// stated area and moves the pin to the polygon centroid; without one the recorded point
// and stated area stand and the satellite layer falls back to the buffered square.
type resolvedPlot struct {
	Latitude     float64
	Longitude    float64
	AreaHectares string
	Boundary     datatypes.JSON
	GpsAccuracyM *float64
}

func resolvePlot(code string, input PlotInput) (resolvedPlot, error) {
	if input.Latitude < -90 || input.Latitude > 90 || input.Longitude < -180 || input.Longitude > 180 {
		return resolvedPlot{}, domain.Invalid("plot %s has coordinates outside valid ranges", code)
	}
	if input.GpsAccuracyM != nil && (*input.GpsAccuracyM < 0 || math.IsNaN(*input.GpsAccuracyM) || math.IsInf(*input.GpsAccuracyM, 0)) {
		return resolvedPlot{}, domain.Invalid("plot %s has an invalid GPS accuracy", code)
	}
	if len(input.Boundary) == 0 || string(input.Boundary) == "null" {
		area, err := money.ParseQuantity(input.AreaHectares)
		if err != nil {
			return resolvedPlot{}, domain.Invalid("plot %s has an invalid area", code)
		}
		return resolvedPlot{Latitude: input.Latitude, Longitude: input.Longitude,
			AreaHectares: money.Quantity(area), GpsAccuracyM: input.GpsAccuracyM}, nil
	}
	ring, err := geo.ParsePolygon(input.Boundary)
	if err != nil {
		return resolvedPlot{}, domain.Invalid("plot %s has an invalid boundary: %v", code, err)
	}
	hectares := geo.AreaHectares(ring)
	if hectares <= 0 || math.IsNaN(hectares) || math.IsInf(hectares, 0) {
		return resolvedPlot{}, domain.Invalid("plot %s boundary encloses no area", code)
	}
	area, err := money.ParseQuantity(strconv.FormatFloat(hectares, 'f', 6, 64))
	if err != nil {
		return resolvedPlot{}, domain.Invalid("plot %s has an invalid area", code)
	}
	lat, lon := geo.Centroid(ring)
	return resolvedPlot{Latitude: lat, Longitude: lon, AreaHectares: money.Quantity(area),
		Boundary: datatypes.JSON(input.Boundary), GpsAccuracyM: input.GpsAccuracyM}, nil
}

type ProjectInput struct {
	Name              string          `json:"name"`
	MethodologyID     string          `json:"methodologyId"`
	CommunityGroup    string          `json:"communityGroup"`
	Species           string          `json:"species"`
	ConservationType  string          `json:"conservationType"`
	CommunityShareBPS int             `json:"communityShareBasisPoints"`
	Boundary          json.RawMessage `json:"boundary,omitempty"`
	Sites             []string        `json:"sites"`
	Plots             []PlotInput     `json:"plots"`
}

// resolveProjectBoundary validates an optional project outline and returns the stored
// bytes plus its outer ring. An absent boundary yields (nil, nil, nil) and imposes no
// containment on the project's plots.
func resolveProjectBoundary(raw json.RawMessage) (datatypes.JSON, geo.Ring, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil, nil
	}
	ring, err := geo.ParsePolygon(raw)
	if err != nil {
		return nil, nil, domain.Invalid("project has an invalid boundary: %v", err)
	}
	if hectares := geo.AreaHectares(ring); hectares <= 0 || math.IsNaN(hectares) || math.IsInf(hectares, 0) {
		return nil, nil, domain.Invalid("project boundary encloses no area")
	}
	return datatypes.JSON(raw), ring, nil
}

// projectBoundary loads a project's stored outline as a ring. It returns has=false for a
// project with no (or an unreadably malformed) boundary so containment silently no-ops
// rather than inventing a rejection from our own stored data.
func (s *Service) projectBoundary(projectID string) (geo.Ring, bool) {
	var project models.ConservationProject
	if err := s.db.Select("boundary").Where("id = ?", projectID).First(&project).Error; err != nil {
		return nil, false
	}
	if len(project.Boundary) == 0 || string(project.Boundary) == "null" {
		return nil, false
	}
	ring, err := geo.ParsePolygon(project.Boundary)
	if err != nil {
		return nil, false
	}
	return ring, true
}

// assertPlotInsideBoundary rejects a plot whose resolved centroid sits outside the
// project's surveyed outline. Projects without a boundary impose nothing.
func assertPlotInsideBoundary(code string, ring geo.Ring, has bool, lat, lon float64) error {
	if has && !geo.PointInRing(ring, lat, lon) {
		return domain.Invalid("plot %s centroid is outside the project boundary", code)
	}
	return nil
}

// assertProjectWritable refuses state-changing writes against an archived project until
// it is unarchived. The record and its history stay intact either way.
func (s *Service) assertProjectWritable(projectID string) error {
	var status string
	if err := s.db.Model(&models.ConservationProject{}).Where("id = ?", projectID).
		Select("status").Scan(&status).Error; err != nil {
		return err
	}
	if status == "" {
		return ErrNotFound
	}
	if status == "archived" {
		return domain.Reject("the project is archived; unarchive it before making changes")
	}
	return nil
}

// CreateProject is manager-only and always flags whether the record is demo data.
func (s *Service) CreateProject(userID string, input ProjectInput, sandboxDemo bool) (models.ConservationProject, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return models.ConservationProject{}, ErrForbidden
	}
	if user.Role != models.RoleManager && user.Role != models.RoleAdmin {
		return models.ConservationProject{}, ErrForbidden
	}
	if strings.TrimSpace(input.Name) == "" {
		return models.ConservationProject{}, domain.Invalid("a project name is required")
	}
	if input.CommunityShareBPS < 0 || input.CommunityShareBPS > 10000 {
		return models.ConservationProject{}, domain.Invalid("the community share must be between 0 and 10000 basis points")
	}
	var methodology models.Methodology
	if err := s.db.Where("id = ?", input.MethodologyID).First(&methodology).Error; err != nil {
		return models.ConservationProject{}, domain.Invalid("the methodology does not exist")
	}
	boundary, boundaryRing, err := resolveProjectBoundary(input.Boundary)
	if err != nil {
		return models.ConservationProject{}, err
	}
	now := s.now().UTC()
	project := models.ConservationProject{
		ID: uuid.NewString(), Name: strings.TrimSpace(input.Name), ManagerID: userID,
		MethodologyID: methodology.ID, Status: "active",
		CommunityGroup: strings.TrimSpace(input.CommunityGroup), Species: strings.TrimSpace(input.Species),
		ConservationType:  normalizeConservationType(input.ConservationType),
		CommunityShareBPS: input.CommunityShareBPS,
		Boundary:          boundary,
		SandboxDemo:       sandboxDemo, CreatedAt: now, UpdatedAt: now,
	}
	siteIDs := map[string]string{}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		for _, name := range input.Sites {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			site := models.Site{ID: uuid.NewString(), ProjectID: project.ID, Name: name, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&site).Error; err != nil {
				return err
			}
			siteIDs[strings.ToLower(name)] = site.ID
		}
		for _, input := range input.Plots {
			siteID := siteIDs[strings.ToLower(strings.TrimSpace(input.SiteName))]
			if siteID == "" {
				return domain.Invalid("plot %s references an unknown site", input.Code)
			}
			resolved, err := resolvePlot(strings.TrimSpace(input.Code), input)
			if err != nil {
				return err
			}
			if err := assertPlotInsideBoundary(strings.TrimSpace(input.Code), boundaryRing, boundaryRing != nil, resolved.Latitude, resolved.Longitude); err != nil {
				return err
			}
			plot := models.Plot{
				ID: uuid.NewString(), ProjectID: project.ID, SiteID: siteID, Code: strings.TrimSpace(input.Code),
				Latitude: resolved.Latitude, Longitude: resolved.Longitude, AreaHectares: resolved.AreaHectares,
				Boundary: resolved.Boundary, GpsAccuracyM: resolved.GpsAccuracyM,
				AssignedMonitorID: input.AssignedMonitorID, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&plot).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return project, err
	}
	return project, nil
}

func normalizeConservationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "seagrass":
		return "seagrass"
	case "reef", "coral", "coral_reef", "coral reef":
		return "reef"
	default:
		return "mangrove"
	}
}
func (s *Service) Methodologies() ([]models.Methodology, error) {
	var rows []models.Methodology
	if err := s.db.Order("created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Activities lists the reporting-period activities opened on a project.
func (s *Service) Activities(projectID string) ([]models.ConservationActivity, error) {
	var rows []models.ConservationActivity
	if err := s.db.Where("project_id = ?", projectID).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Monitors lists active field monitors available for assignment.
func (s *Service) Monitors() ([]models.User, error) {
	var rows []models.User
	if err := s.db.Where("role = ? AND status = ?", models.RoleMonitor, models.IdentityActive).Order("display_name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdatePlot edits a plot's code, site, geometry and assigned monitor. A surveyed
// boundary still derives area + centroid, the centroid still has to sit inside the
// project outline, and a changed code must stay unique in the project. Manager or admin
// only: a plot edit is project state, not a monitor's routine action.
func (s *Service) UpdatePlot(projectID, plotID, userID string, patch PlotPatch) (models.Plot, error) {
	if err := s.AssertProjectManager(userID, projectID); err != nil {
		return models.Plot{}, err
	}
	if err := s.assertProjectWritable(projectID); err != nil {
		return models.Plot{}, err
	}
	var plot models.Plot
	if err := s.db.Where("id = ? AND project_id = ?", plotID, projectID).First(&plot).Error; err != nil {
		return models.Plot{}, ErrNotFound
	}
	input := mergePlotPatch(plot, patch)
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return models.Plot{}, domain.Invalid("a plot code is required")
	}
	resolved, err := resolvePlot(code, input)
	if err != nil {
		return models.Plot{}, err
	}
	boundaryRing, hasBoundary := s.projectBoundary(projectID)
	if err := assertPlotInsideBoundary(code, boundaryRing, hasBoundary, resolved.Latitude, resolved.Longitude); err != nil {
		return models.Plot{}, err
	}
	if code != plot.Code {
		var clash models.Plot
		if err := s.db.Where("project_id = ? AND code = ? AND id <> ?", projectID, code, plotID).First(&clash).Error; err == nil {
			return models.Plot{}, domain.Reject("plot %s already exists in this project", code)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Plot{}, err
		}
	}
	siteID := plot.SiteID
	if name := strings.TrimSpace(input.SiteName); name != "" {
		var site models.Site
		if findErr := s.db.Where("project_id = ? AND LOWER(name) = ?", projectID, strings.ToLower(name)).First(&site).Error; findErr != nil {
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return models.Plot{}, findErr
			}
			site = models.Site{ID: uuid.NewString(), ProjectID: projectID, Name: name, CreatedAt: s.now().UTC(), UpdatedAt: s.now().UTC()}
			if err := s.db.Create(&site).Error; err != nil {
				return models.Plot{}, err
			}
		}
		siteID = site.ID
	}
	assigned := input.AssignedMonitorID
	if assigned != "" {
		var monitor models.User
		if err := s.db.Where("id = ? AND role = ?", assigned, models.RoleMonitor).First(&monitor).Error; err != nil {
			return models.Plot{}, domain.Invalid("the selected monitor is not available")
		}
	}
	previousMonitor := plot.AssignedMonitorID
	plot.Code = code
	plot.SiteID = siteID
	plot.Latitude = resolved.Latitude
	plot.Longitude = resolved.Longitude
	plot.AreaHectares = resolved.AreaHectares
	plot.Boundary = resolved.Boundary
	plot.GpsAccuracyM = resolved.GpsAccuracyM
	plot.AssignedMonitorID = assigned
	plot.UpdatedAt = s.now().UTC()
	if err := s.db.Save(&plot).Error; err != nil {
		return models.Plot{}, err
	}
	// Issue MonitorCap for a newly assigned monitor (non-blocking, idempotent).
	if assigned != "" && assigned != previousMonitor {
		capturedProjectID := projectID
		capturedPlotID := plotID
		capturedMonitorID := assigned
		go func() {
			ctx := context.Background()
			provision, err := s.identities.EnsureProvision(ctx, capturedMonitorID)
			if err != nil || provision.RoleCapObjectID != "" {
				return
			}
			s.identities.IssueMonitorCapForUser(ctx, capturedMonitorID, "monitor", provision, capturedProjectID, capturedPlotID)
		}()
	}
	return plot, nil
}

// DeletePlot removes a plot that no activity depends on. It is a hard delete: a plot
// with no activities has no downstream ledger of its own. A plot already used by a
// monitoring record or activity is refused so anchored history never loses its subject.
func (s *Service) DeletePlot(projectID, plotID, userID string) error {
	if err := s.AssertProjectManager(userID, projectID); err != nil {
		return err
	}
	if err := s.assertProjectWritable(projectID); err != nil {
		return err
	}
	var plot models.Plot
	if err := s.db.Where("id = ? AND project_id = ?", plotID, projectID).First(&plot).Error; err != nil {
		return ErrNotFound
	}
	var used int64
	if err := s.db.Model(&models.ConservationActivity{}).Where("plot_id = ?", plotID).Count(&used).Error; err != nil {
		return err
	}
	if used > 0 {
		return domain.Reject("the plot has activities and cannot be deleted")
	}
	return s.db.Delete(&plot).Error
}

// ProjectUpdateInput carries the manager-editable basics of a project. Methodology is
// deliberately absent: it is fixed at creation. Every field keeps its stored value when
// the key is omitted; an explicit null clears a text field. For boundary, an absent key
// keeps the stored outline while an explicit JSON null clears it.
type ProjectUpdateInput struct {
	Name              string          `json:"name"`
	CommunityGroup    *string         `json:"communityGroup"`
	Species           *string         `json:"species"`
	ConservationType  *string         `json:"conservationType"`
	CommunityShareBPS *int            `json:"communityShareBasisPoints"`
	Boundary          json.RawMessage `json:"boundary"`
}

// UpdateProject edits a project's basics and outline. Manager or admin
// only, and only while the project is not archived. Setting a boundary must still contain
// every plot already registered, so a manager cannot draw an outline around an empty
// area and strand existing parcels outside it.
func (s *Service) UpdateProject(projectID, userID string, input ProjectUpdateInput) (models.ConservationProject, error) {
	if err := s.AssertProjectManager(userID, projectID); err != nil {
		return models.ConservationProject{}, err
	}
	if err := s.assertProjectWritable(projectID); err != nil {
		return models.ConservationProject{}, err
	}
	var project models.ConservationProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return models.ConservationProject{}, ErrNotFound
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.ConservationProject{}, domain.Invalid("a project name is required")
	}
	share := project.CommunityShareBPS
	if input.CommunityShareBPS != nil {
		if *input.CommunityShareBPS < 0 || *input.CommunityShareBPS > 10000 {
			return models.ConservationProject{}, domain.Invalid("the community share must be between 0 and 10000 basis points")
		}
		share = *input.CommunityShareBPS
	}
	rawBoundary := json.RawMessage(project.Boundary)
	if len(input.Boundary) > 0 {
		rawBoundary = input.Boundary
	}
	// Only a freshly supplied outline is re-checked against existing plots; the stored
	// one may predate a plot that was added before any boundary existed.
	boundarySent := len(input.Boundary) > 0
	boundary, boundaryRing, err := resolveProjectBoundary(rawBoundary)
	if err != nil {
		return models.ConservationProject{}, err
	}
	if boundarySent && boundaryRing != nil {
		var plots []models.Plot
		if err := s.db.Where("project_id = ?", projectID).Find(&plots).Error; err != nil {
			return models.ConservationProject{}, err
		}
		for _, p := range plots {
			if err := assertPlotInsideBoundary(p.Code, boundaryRing, true, p.Latitude, p.Longitude); err != nil {
				return models.ConservationProject{}, err
			}
		}
	}
	project.Name = name
	if input.CommunityGroup != nil {
		project.CommunityGroup = strings.TrimSpace(*input.CommunityGroup)
	}
	if input.Species != nil {
		project.Species = strings.TrimSpace(*input.Species)
	}
	if input.ConservationType != nil {
		project.ConservationType = normalizeConservationType(*input.ConservationType)
	}
	project.CommunityShareBPS = share
	project.Boundary = boundary
	project.UpdatedAt = s.now().UTC()
	if err := s.db.Save(&project).Error; err != nil {
		return models.ConservationProject{}, err
	}
	return project, nil
}

// setProjectStatus archives or restores a project. The status column already exists, so
// this is reversible and touches no activity, plot, ledger entry or anchor.
func (s *Service) setProjectStatus(projectID, userID, status string) (models.ConservationProject, error) {
	if err := s.AssertProjectManager(userID, projectID); err != nil {
		return models.ConservationProject{}, err
	}
	var project models.ConservationProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return models.ConservationProject{}, ErrNotFound
	}
	project.Status = status
	project.UpdatedAt = s.now().UTC()
	if err := s.db.Save(&project).Error; err != nil {
		return models.ConservationProject{}, err
	}
	return project, nil
}

// ArchiveProject soft-deletes a project out of the active set without deleting anything.
func (s *Service) ArchiveProject(projectID, userID string) (models.ConservationProject, error) {
	return s.setProjectStatus(projectID, userID, "archived")
}

// UnarchiveProject returns an archived project to active.
func (s *Service) UnarchiveProject(projectID, userID string) (models.ConservationProject, error) {
	return s.setProjectStatus(projectID, userID, "active")
}

// AddPlot registers one more measured parcel on an existing project. Sites are
// created on demand by name, the same rule CreateProject applies, so a manager
// can add a parcel at a new site without re-registering the project.
func (s *Service) AddPlot(projectID, userID string, input PlotInput) (models.Plot, error) {
	if err := s.AssertProjectAccess(userID, projectID); err != nil {
		return models.Plot{}, err
	}
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return models.Plot{}, ErrForbidden
	}
	if user.Role != models.RoleManager && user.Role != models.RoleAdmin {
		return models.Plot{}, ErrForbidden
	}
	if err := s.assertProjectWritable(projectID); err != nil {
		return models.Plot{}, err
	}
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return models.Plot{}, domain.Invalid("a plot code is required")
	}
	siteName := strings.TrimSpace(input.SiteName)
	if siteName == "" {
		return models.Plot{}, domain.Invalid("a site name is required")
	}
	resolved, err := resolvePlot(code, input)
	if err != nil {
		return models.Plot{}, err
	}
	boundaryRing, hasBoundary := s.projectBoundary(projectID)
	if err := assertPlotInsideBoundary(code, boundaryRing, hasBoundary, resolved.Latitude, resolved.Longitude); err != nil {
		return models.Plot{}, err
	}
	if input.AssignedMonitorID != "" {
		var monitor models.User
		if err := s.db.Where("id = ? AND role = ?", input.AssignedMonitorID, models.RoleMonitor).First(&monitor).Error; err != nil {
			return models.Plot{}, domain.Invalid("the assigned monitor is not a field monitor")
		}
	}
	var existing models.Plot
	if err := s.db.Where("project_id = ? AND code = ?", projectID, code).First(&existing).Error; err == nil {
		return models.Plot{}, domain.Reject("plot %s already exists in this project", code)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Plot{}, err
	}

	now := s.now().UTC()
	plot := models.Plot{
		ID: uuid.NewString(), ProjectID: projectID, Code: code,
		Latitude: resolved.Latitude, Longitude: resolved.Longitude, AreaHectares: resolved.AreaHectares,
		Boundary: resolved.Boundary, GpsAccuracyM: resolved.GpsAccuracyM,
		AssignedMonitorID: input.AssignedMonitorID, CreatedAt: now, UpdatedAt: now,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var site models.Site
		if findErr := tx.Where("project_id = ? AND LOWER(name) = ?", projectID, strings.ToLower(siteName)).
			First(&site).Error; findErr != nil {
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			site = models.Site{ID: uuid.NewString(), ProjectID: projectID, Name: siteName, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&site).Error; err != nil {
				return err
			}
		}
		plot.SiteID = site.ID
		return tx.Create(&plot).Error
	})
	if err != nil {
		return models.Plot{}, err
	}
	return plot, nil
}

// CreateActivity opens a reporting-period activity on a plot.
type ActivityInput struct {
	ProjectID            string         `json:"projectId"`
	PlotID               string         `json:"plotId"`
	ActivityType         string         `json:"activityType"`
	Title                string         `json:"title"`
	MethodologyVersion   string         `json:"methodologyVersion"`
	ReportingPeriodStart *time.Time     `json:"reportingPeriodStart"`
	ReportingPeriodEnd   *time.Time     `json:"reportingPeriodEnd"`
	Measurements         map[string]any `json:"measurements"`
}

// maxActivityTitle keeps a manager-supplied name to one readable line.
const maxActivityTitle = 120

func normalizeActivityTitle(value string) (string, error) {
	title := strings.TrimSpace(value)
	if len([]rune(title)) > maxActivityTitle {
		return "", domain.Invalid("the activity name is limited to %d characters", maxActivityTitle)
	}
	return title, nil
}

// UnmarshalJSON accepts reporting periods as either full RFC3339 timestamps or
// plain YYYY-MM-DD dates, since the web and mobile clients submit date inputs.
func (i *ActivityInput) UnmarshalJSON(data []byte) error {
	type raw struct {
		ProjectID            string         `json:"projectId"`
		PlotID               string         `json:"plotId"`
		ActivityType         string         `json:"activityType"`
		Title                string         `json:"title"`
		MethodologyVersion   string         `json:"methodologyVersion"`
		ReportingPeriodStart *string        `json:"reportingPeriodStart"`
		ReportingPeriodEnd   *string        `json:"reportingPeriodEnd"`
		Measurements         map[string]any `json:"measurements"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	i.ProjectID = r.ProjectID
	i.PlotID = r.PlotID
	i.ActivityType = r.ActivityType
	i.Title = r.Title
	i.MethodologyVersion = r.MethodologyVersion
	i.Measurements = r.Measurements
	if r.ReportingPeriodStart != nil && strings.TrimSpace(*r.ReportingPeriodStart) != "" {
		t, err := parseActivityDate(*r.ReportingPeriodStart)
		if err != nil {
			return err
		}
		i.ReportingPeriodStart = &t
	}
	if r.ReportingPeriodEnd != nil && strings.TrimSpace(*r.ReportingPeriodEnd) != "" {
		t, err := parseActivityDate(*r.ReportingPeriodEnd)
		if err != nil {
			return err
		}
		i.ReportingPeriodEnd = &t
	}
	return nil
}

func marshalMeasurements(values map[string]any) datatypes.JSON {
	if len(values) == 0 {
		return datatypes.JSON([]byte(`{}`))
	}
	body, err := json.Marshal(values)
	if err != nil {
		return datatypes.JSON([]byte(`{}`))
	}
	return datatypes.JSON(body)
}

func parseActivityDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid reporting period date %q", value)
}

func (s *Service) CreateActivity(userID string, input ActivityInput) (models.ConservationActivity, error) {
	if err := s.AssertProjectAccess(userID, input.ProjectID); err != nil {
		return models.ConservationActivity{}, err
	}
	if err := s.assertProjectWritable(input.ProjectID); err != nil {
		return models.ConservationActivity{}, err
	}
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return models.ConservationActivity{}, ErrForbidden
	}
	if user.Role != models.RoleManager && user.Role != models.RoleAdmin {
		return models.ConservationActivity{}, ErrForbidden
	}
	var plot models.Plot
	if input.PlotID == "" || input.ProjectID == "" {
		return models.ConservationActivity{}, domain.Invalid("plot and project IDs are required")
	}
	if err := s.db.Where("id = ? AND project_id = ?", input.PlotID, input.ProjectID).First(&plot).Error; err != nil {
		return models.ConservationActivity{}, domain.Invalid("the plot does not belong to this project")
	}
	if input.ReportingPeriodEnd != nil && input.ReportingPeriodStart != nil &&
		input.ReportingPeriodEnd.Before(*input.ReportingPeriodStart) {
		return models.ConservationActivity{}, domain.Invalid("the reporting period ends before it starts")
	}
	if input.ActivityType == "" || input.MethodologyVersion == "" {
		return models.ConservationActivity{}, domain.Invalid("activity type and methodology version are required")
	}
	title, err := normalizeActivityTitle(input.Title)
	if err != nil {
		return models.ConservationActivity{}, err
	}
	now := s.now().UTC()
	// Default reporting period: past 90 days if not supplied
	start := now.AddDate(0, -3, 0)
	end := now
	if input.ReportingPeriodStart != nil {
		start = input.ReportingPeriodStart.UTC()
	}
	if input.ReportingPeriodEnd != nil {
		end = input.ReportingPeriodEnd.UTC()
	}
	project, _, err := s.project(input.ProjectID)
	if err != nil {
		return models.ConservationActivity{}, err
	}
	activity := models.ConservationActivity{
		ID: uuid.NewString(), ProjectID: project.ID, PlotID: plot.ID, ManagerID: project.ManagerID,
		ActivityType: input.ActivityType, Title: title, Status: models.ActivityDraft,
		ReportingPeriodStart: start, ReportingPeriodEnd: end,
		MethodologyVersion: input.MethodologyVersion, Measurements: marshalMeasurements(input.Measurements),
		CreatedAt: now, UpdatedAt: now,
	}
	if activity.ManagerID == "" {
		return activity, errors.New("the project manager is unavailable")
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}
		if activity.ActivityType != "planting" {
			return nil
		}
		center := now
		if input.ReportingPeriodStart != nil {
			center = input.ReportingPeriodStart.UTC()
		}
		request, err := satellite.NewActivityNdviRequest(activity, plot, center, userID)
		if err != nil {
			return err
		}
		return tx.Create(&request).Error
	})
	return activity, err
}
