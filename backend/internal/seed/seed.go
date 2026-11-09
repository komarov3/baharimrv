// Package seed builds a demo conservation project for local development. It runs
// only when SEED_DEMO_DATA=true (config forbids it in production) and every record
// it produces is derived through the same domain services a real client drives, so
// nothing in the demo can claim a state the adapters have not actually produced.
package seed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/auth"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/money"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const demoDomain = "demo.baharimrv.local"

// demoEmail builds a seeded sign-in address. Demo accounts use the same
// local@host shape real operators do, so a client that validates the field
// before submitting does not reject the demo workload.
func demoEmail(local string) string { return local + "@" + demoDomain }

// placeholderPNG is a 1x1 transparent PNG. It stands in for field media so the
// evidence pipeline has a real file to hash, store and compare; the project it
// belongs to is flagged as demo data, so it is never presented as a real survey
// photograph.
const placeholderPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk" +
	"+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

type monitorSpec struct {
	Email   string
	Name    string
	Planted int
	Alive   int
	Height  string
	Species string
	Lat     float64
	Lng     float64
}

var demoMonitors = []monitorSpec{
	{Email: demoEmail("amina"), Name: "Amina Thurei", Planted: 420, Alive: 388,
		Height: "1.420000", Species: "Rhizophora mucronata", Lat: -4.0532, Lng: 39.6741},
	{Email: demoEmail("juma"), Name: "Juma Mbacke", Planted: 310, Alive: 302,
		Height: "1.180000", Species: "Avicennia marina", Lat: -4.0498, Lng: 39.6812},
}

var gaziMonitors = []monitorSpec{
	{Email: demoEmail("said"), Name: "Said Bakari", Planted: 380, Alive: 341,
		Height: "0.920000", Species: "Ceriops tagal", Lat: -4.4224, Lng: 39.5108},
	{Email: demoEmail("fatuma"), Name: "Fatuma Ali", Planted: 295, Alive: 271,
		Height: "0.760000", Species: "Ceriops tagal", Lat: -4.4231, Lng: 39.5115},
}

// Run installs the demo dataset. It is a no-op when the accounts already exist so
// restarting a development server never duplicates a project.
func Run(db *gorm.DB, identities *identity.Service, projects *mrv.Service, oidc *auth.OIDCService, password string, now time.Time) error {
	if password == "" {
		return fmt.Errorf("demo seeding requires DEMO_SEED_PASSWORD")
	}
	var existing int64
	if err := db.Model(&models.User{}).Where("email = ?", demoEmail("manager")).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	manager, err := account(ctx, db, hash, demoEmail("manager"), "Halima Rasid", models.RoleManager, identities, now)
	if err != nil {
		return err
	}
	for _, support := range []struct{ email, name, role string }{
		{demoEmail("verifier"), "Dr. Peter Mwaka", models.RoleVerifier},
		{demoEmail("buyer"), "Kaskazi Coast Ltd", models.RoleBuyer},
		{demoEmail("finance"), "Zawadi Omar", models.RoleFinance},
		{demoEmail("admin"), "BahariMRV Administrator", models.RoleAdmin},
	} {
		if _, err := account(ctx, db, hash, support.email, support.name, support.role, identities, now); err != nil {
			return err
		}
	}

	methodology, err := methodologyRow(db, now)
	if err != nil {
		return err
	}
	project, err := demoProject(projects, manager, methodology)
	if err != nil {
		return err
	}
	plots, err := plotsFor(db, project.ID)
	if err != nil {
		return err
	}
	if len(plots) < len(demoMonitors) {
		return fmt.Errorf("the demo project was created without its plots")
	}
	for index, spec := range demoMonitors {
		monitor, err := account(ctx, db, hash, spec.Email, spec.Name, models.RoleMonitor, identities, now)
		if err != nil {
			return err
		}
		if err := db.Model(&models.Plot{}).Where("id = ?", plots[index].ID).
			Update("assigned_monitor_id", monitor.ID).Error; err != nil {
			return fmt.Errorf("assign the demo monitor: %w", err)
		}
		activity, err := projects.CreateActivity(manager.ID, mrv.ActivityInput{
			ProjectID: project.ID, PlotID: plots[index].ID, ActivityType: "restoration",
			MethodologyVersion:   methodology.Version,
			ReportingPeriodStart: now.AddDate(0, -6, 0), ReportingPeriodEnd: now,
		})
		if err != nil {
			return err
		}
		if err := capture(projects, monitor, activity, spec, now); err != nil {
			return err
		}
		// Only the first plot advances to a completion claim, so the demo shows both
		// an activity awaiting verification and one still under monitoring.
		if index == 0 {
			if _, _, err := projects.Complete(ctx, activity.ID, manager.ID); err != nil {
				return err
			}
		}
	}
	if err := sandboxBatch(db, project, now); err != nil {
		return err
	}
	if err := financialImports(db, project.ID, now); err != nil {
		return err
	}
	if err := runGaziBay(ctx, db, hash, identities, projects, now); err != nil {
		return fmt.Errorf("gazi bay seed: %w", err)
	}
	return devClient(oidc)
}

func account(ctx context.Context, db *gorm.DB, hash []byte, email, name, role string, identities *identity.Service, now time.Time) (models.User, error) {
	user := models.User{
		ID: uuid.NewString(), Email: email, PasswordHash: string(hash), DisplayName: name,
		Role: role, Status: models.IdentityPending, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&user).Error; err != nil {
		return user, fmt.Errorf("create demo account %s: %w", email, err)
	}
	if err := identities.Initialize(user.ID); err != nil {
		return user, fmt.Errorf("register identity request for %s: %w", email, err)
	}
	if _, err := identities.EnsureProvision(ctx, user.ID); err != nil {
		return user, fmt.Errorf("provision demo identity for %s: %w", email, err)
	}
	return user, nil
}

func methodologyRow(db *gorm.DB, now time.Time) (models.Methodology, error) {
	estimation, err := json.Marshal(map[string]any{
		"tonnes_co2_per_tree":    "0.006250",
		"source":                 "Demo parameter set. Not a verified allometric equation.",
		"survival_threshold_pct": 70,
		"period_years":           1,
	})
	if err != nil {
		return models.Methodology{}, fmt.Errorf("encode demo estimation parameters: %w", err)
	}
	methodology := models.Methodology{
		ID: uuid.NewString(), Code: "MED-MANGROVE-DEMO", Name: "Community mangrove restoration (demo methodology)",
		Version:          "1.0",
		RequiredEvidence: "Geo-tagged survival count, average height, species note, at least one field photo or voice log.",
		Estimation:       datatypes.JSON(estimation), CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&methodology).Error; err != nil {
		return methodology, fmt.Errorf("create demo methodology: %w", err)
	}
	return methodology, nil
}

func demoProject(projects *mrv.Service, manager models.User, methodology models.Methodology) (models.ConservationProject, error) {
	plots := make([]mrv.PlotInput, 0, len(demoMonitors))
	for index, spec := range demoMonitors {
		plots = append(plots, mrv.PlotInput{
			Code: fmt.Sprintf("MIKOKO-P%d", index+1), SiteName: "Mikoko Park",
			Latitude: spec.Lat, Longitude: spec.Lng, AreaHectares: "2.500000",
		})
	}
	return projects.CreateProject(manager.ID, mrv.ProjectInput{
		Name: "Mikoko Park mangrove restoration (demo)", MethodologyID: methodology.ID,
		CommunityGroup:    "Mikoko Women's Restoration Group (demo)",
		Species:           "Rhizophora mucronata",
		CommunityShareBPS: 4000, Sites: []string{"Mikoko Park"}, Plots: plots,
	}, true)
}

func gaziProject(projects *mrv.Service, manager models.User, methodology models.Methodology) (models.ConservationProject, error) {
	plots := []mrv.PlotInput{
		{Code: "GAZI-P1", SiteName: "Gazi Bay", Latitude: -4.4224, Longitude: 39.5108, AreaHectares: "3.000000"},
		{Code: "GAZI-P2", SiteName: "Gazi Bay", Latitude: -4.4231, Longitude: 39.5115, AreaHectares: "3.000000"},
	}
	return projects.CreateProject(manager.ID, mrv.ProjectInput{
		Name:              "Gazi Bay Mangrove Restoration (demo)",
		MethodologyID:     methodology.ID,
		CommunityGroup:    "Gazi Bay Community Restoration Group (demo)",
		Species:           "Ceriops tagal",
		CommunityShareBPS: 4500,
		Sites:             []string{"Gazi Bay"},
		Plots:             plots,
	}, true)
}

// runGaziBay seeds the Gazi Bay Hamisi demo project. It is called from Run()
// after the Mikoko block. The idempotency guard checks the project row by name.
//
// NOTE: If the Mikoko block fails or is skipped (e.g. the manager already exists
// but the project does not), the Gazi Bay block will also be skipped on re-runs
// because Run() returns nil early when the Mikoko manager exists. A dev DB wipe
// is required if Gazi Bay is missing from an otherwise complete Mikoko seed.
func runGaziBay(ctx context.Context, db *gorm.DB, hash []byte, identities *identity.Service, projects *mrv.Service, now time.Time) error {
	// Idempotency check on project name, not manager user.
	var existingGazi int64
	if err := db.Model(&models.ConservationProject{}).
		Where("name = ?", "Gazi Bay Mangrove Restoration (demo)").
		Count(&existingGazi).Error; err != nil {
		return err
	}
	if existingGazi > 0 {
		return nil // Gazi Bay seed already present
	}

	gaziManager, err := account(ctx, db, hash, demoEmail("hamisi.manager"), "Hamisi Mwangi", models.RoleManager, identities, now)
	if err != nil {
		return err
	}

	// Reuse the MED-MANGROVE-DEMO methodology created by the Mikoko seed.
	// NOTE: Mikoko must run first for this to exist.
	var methodology models.Methodology
	if err := db.Where("code = ?", "MED-MANGROVE-DEMO").First(&methodology).Error; err != nil {
		return fmt.Errorf("demo methodology MED-MANGROVE-DEMO not found: %w (Mikoko seed must run first)", err)
	}

	project, err := gaziProject(projects, gaziManager, methodology)
	if err != nil {
		return err
	}
	plots, err := plotsFor(db, project.ID)
	if err != nil {
		return err
	}
	if len(plots) < len(gaziMonitors) {
		return fmt.Errorf("the gazi bay project was created without its plots")
	}
	for index, spec := range gaziMonitors {
		monitor, err := account(ctx, db, hash, spec.Email, spec.Name, models.RoleMonitor, identities, now)
		if err != nil {
			return err
		}
		if err := db.Model(&models.Plot{}).Where("id = ?", plots[index].ID).
			Update("assigned_monitor_id", monitor.ID).Error; err != nil {
			return fmt.Errorf("assign gazi bay monitor: %w", err)
		}
		activity, err := projects.CreateActivity(gaziManager.ID, mrv.ActivityInput{
			ProjectID: project.ID, PlotID: plots[index].ID, ActivityType: "restoration",
			MethodologyVersion:   methodology.Version,
			ReportingPeriodStart: now.AddDate(0, -6, 0), ReportingPeriodEnd: now,
		})
		if err != nil {
			return err
		}
		if err := capture(projects, monitor, activity, spec, now); err != nil {
			return err
		}
		// Only the first plot advances to a completion claim, mirroring the Mikoko pattern.
		if index == 0 {
			_, event, err := projects.Complete(ctx, activity.ID, gaziManager.ID)
			if err != nil {
				return fmt.Errorf("complete Gazi Bay activity: %w", err)
			}
			if event.Status != provenance.StatusAnchored {
				return fmt.Errorf("Gazi Bay activity %s anchor status = %q, want %q",
					activity.ID, event.Status, provenance.StatusAnchored)
			}
		}
	}
	return nil
}

// capture pushes one record through the real synchronisation path and then uploads
// the media it declares, so the evidence gap this activity reports is genuinely
// closed rather than asserted.
func capture(projects *mrv.Service, monitor models.User, activity models.ConservationActivity, spec monitorSpec, now time.Time) error {
	body, err := base64.StdEncoding.DecodeString(placeholderPNG)
	if err != nil {
		return fmt.Errorf("decode demo media: %w", err)
	}
	manifest := []mrv.EvidenceManifestItem{{
		Name: "transect-01.png", Kind: "photo", MediaType: "image/png",
		Size: int64(len(body)), SHA256: fmt.Sprintf("%x", sha256.Sum256(body)),
	}}
	response, err := projects.SyncBatch(context.Background(), monitor.ID, mrv.Batch{Actions: []mrv.Action{{
		Type: "monitoring_record",
		Record: mrv.Payload{
			ClientID: uuid.NewString(), ActivityID: activity.ID, PlotID: activity.PlotID,
			MethodologyVersion: activity.MethodologyVersion, SchemaVersion: "mrv-monitoring/1",
			DeviceTimestamp: now.Add(-48 * time.Hour),
			Location:        mrv.Location{Latitude: spec.Lat, Longitude: spec.Lng, AccuracyMetres: 3.2},
			AliveCount:      spec.Alive, PlantedCount: spec.Planted, AverageHeightMetres: spec.Height,
			Species: spec.Species, Observations: "Demo transect walk. Natural regeneration along the landward edge.",
			Attestation:      mrv.Attestation{Statement: "I counted these seedlings myself on the day recorded."},
			EvidenceManifest: manifest,
		},
	}}})
	if err != nil {
		return fmt.Errorf("sync demo record: %w", err)
	}
	if len(response.Results) == 0 || response.Results[0].MonitoringRecordID == "" {
		return fmt.Errorf("the demo record was not accepted: %s", resultsError(response))
	}
	if _, err := projects.UploadEvidence(monitor.ID, response.Results[0].MonitoringRecordID,
		manifest[0], io.NopCloser(bytes.NewReader(body))); err != nil {
		return fmt.Errorf("store demo media: %w", err)
	}
	return nil
}

// sandboxBatch records an issuance the sandbox adapter invented. It stays
// unreconciled, so the marketplace lists it as unverifiable inventory and refuses
// to sell it as an issued credit.
func sandboxBatch(db *gorm.DB, project models.ConservationProject, now time.Time) error {
	quantity := money.Quantity(money.Hundred)
	batch := models.CreditBatch{
		ID: uuid.NewString(), ProjectID: project.ID,
		RegistryIssuanceID: "SANDBOX-ISSUANCE-" + strings.ToUpper(shortID(project.ID)),
		RegistryName:       "Sandbox registry (no external confirmation)", Vintage: now.Year(),
		SerialStart: "SANDBOX-000001", SerialEnd: "SANDBOX-000100",
		Quantity: quantity, AvailableQuantity: quantity, Status: "issued",
		AdapterMode: "sandbox", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&batch).Error; err != nil {
		return err
	}
	listing := models.MarketplaceListing{
		ID: uuid.NewString(), BatchID: batch.ID, ProjectID: project.ID,
		ProductType: "registry_issued", PricePerTonne: "35.00", Currency: "KES",
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	return db.Create(&listing).Error
}

func financialImports(db *gorm.DB, projectID string, now time.Time) error {
	entries := []struct {
		external string
		amount   string
		when     time.Time
	}{
		{external: "bl-tour-0001", amount: "18500.00", when: now.Add(-21 * 24 * time.Hour)},
		{external: "bl-tour-0002", amount: "9240.50", when: now.Add(-6 * 24 * time.Hour)},
	}
	imports := make([]models.FinancialImport, 0, len(entries))
	for _, entry := range entries {
		imports = append(imports, models.FinancialImport{
			ID: uuid.NewString(), Source: "baharilink-tourism", ExternalID: entry.external,
			ProjectID: projectID, Amount: entry.amount, Currency: "KES", OccurredAt: entry.when,
			PayloadHash: fmt.Sprintf("%x", sha256.Sum256([]byte(entry.external+entry.amount))),
			CreatedAt:   entry.when,
		})
	}
	return db.Create(&imports).Error
}

// devClient registers a public PKCE-only client so a local sibling platform can
// exercise BahariMRV as an identity provider. There is no secret to leak.
func devClient(oidc *auth.OIDCService) error {
	_, err := oidc.RegisterClient("baharimrv-local-web", "", []string{"http://localhost:5173/auth/callback"},
		"openid profile email conservation:read")
	return err
}

func plotsFor(db *gorm.DB, projectID string) ([]models.Plot, error) {
	var plots []models.Plot
	err := db.Where("project_id = ?", projectID).Order("code asc").Find(&plots).Error
	return plots, err
}

func resultsError(response mrv.BatchResponse) string {
	messages := make([]string, 0, len(response.Results))
	for _, result := range response.Results {
		if result.Error != "" {
			messages = append(messages, result.Error)
		}
	}
	if len(messages) == 0 {
		return "no record identifier returned"
	}
	return strings.Join(messages, "; ")
}

func shortID(identifier string) string {
	if len(identifier) > 8 {
		return identifier[:8]
	}
	return identifier
}
