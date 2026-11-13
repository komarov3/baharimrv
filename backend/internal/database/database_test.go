package database

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openTemporary(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	database, err := Open(dsn, false)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return database
}

func TestMigrationAppliesAndRoundTrips(t *testing.T) {
	db := openTemporary(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	user := models.User{
		ID: uuid.NewString(), Email: "monitor@baharilink.test", PasswordHash: "hash",
		DisplayName: "Asha Wanjiru", Phone: "+254700000001", Role: models.RoleMonitor,
		Status: models.IdentityActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	provision := models.IdentityProvision{
		ID: uuid.NewString(), UserID: user.ID, State: models.IdentityActive,
		Network: "sandbox", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&provision).Error; err != nil {
		t.Fatalf("create provision with omitted optional strings: %v", err)
	}

	batch := models.CreditBatch{
		ID: uuid.NewString(), ProjectID: user.ID, RegistryIssuanceID: "VCS-1234",
		RegistryName: "Sandbox Registry", Vintage: 2026, SerialStart: "S-1", SerialEnd: "S-9",
		Quantity: "1250.000500", AvailableQuantity: "1250.000500", Status: "available",
		AdapterMode: "sandbox", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatalf("create decimal batch: %v", err)
	}

	record := models.MonitoringRecord{
		ID: uuid.NewString(), ActivityID: user.ID, MonitorID: user.ID, ClientID: "client-1",
		DeviceTimestamp: now, ServerReceiptTime: now, MethodologyVersion: "v1", SchemaVersion: "1.0",
		Latitude: -4.0435, Longitude: 39.6682, GPSAccuracyMeters: 4.5, AliveCount: 120, PlantedCount: 140,
		AverageHeightMetres: "1.250000",
		Species:             datatypes.JSON([]byte(`["Rhizophora mucronata"]`)), Attestation: "signed",
		AttestationHash: "hash", EvidenceManifest: datatypes.JSON([]byte(`[]`)),
		ContentCommitment: "commitment", SyncStatus: "submitted",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create monitoring record: %v", err)
	}

	var loadedUser models.User
	if err := db.First(&loadedUser, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !loadedUser.CreatedAt.Equal(now) {
		t.Fatalf("timestamp round-trip lost: want %v got %v", now, loadedUser.CreatedAt)
	}

	var loadedProvision models.IdentityProvision
	if err := db.First(&loadedProvision, "user_id = ?", user.ID).Error; err != nil {
		t.Fatalf("reload provision: %v", err)
	}
	if loadedProvision.DID != "" || loadedProvision.KeyReferenceID != "" {
		t.Fatalf("omitted optional strings did not default to empty: %+v", loadedProvision)
	}

	var loadedBatch models.CreditBatch
	if err := db.First(&loadedBatch, "id = ?", batch.ID).Error; err != nil {
		t.Fatalf("reload batch: %v", err)
	}
	if loadedBatch.Quantity != "1250.000500" || loadedBatch.AvailableQuantity != "1250.000500" {
		t.Fatalf("decimal string corrupted: quantity=%q available=%q", loadedBatch.Quantity, loadedBatch.AvailableQuantity)
	}

	var loadedRecord models.MonitoringRecord
	if err := db.First(&loadedRecord, "id = ?", record.ID).Error; err != nil {
		t.Fatalf("reload monitoring record: %v", err)
	}
	if string(loadedRecord.Species) != `["Rhizophora mucronata"]` {
		t.Fatalf("json round-trip changed: %s", loadedRecord.Species)
	}
	if loadedRecord.AverageHeightMetres != "1.250000" {
		t.Fatalf("decimal string corrupted: average height round-tripped as %q", loadedRecord.AverageHeightMetres)
	}
	if string(loadedRecord.EvidenceManifest) != `[]` {
		t.Fatalf("evidence manifest round-trip changed: %s", loadedRecord.EvidenceManifest)
	}
}

func TestPostgresSatelliteTileZoomUpgrade(t *testing.T) {
	dsn := os.Getenv("BAHARIMRV_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("BAHARIMRV_TEST_POSTGRES_DSN is not configured")
	}
	for _, test := range []struct {
		name    string
		missing []string
	}{
		{"original-schema", []string{"tile_min_zoom", "tile_max_zoom"}},
		{"missing-minimum", []string{"tile_min_zoom"}},
		{"missing-maximum", []string{"tile_max_zoom"}},
		{"columns-already-present", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			tx := db.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()
			schema := "ndvi_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			for _, statement := range []string{"CREATE SCHEMA " + schema, "SET LOCAL search_path TO " + schema} {
				if err := tx.Exec(statement).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := migrate(tx, "postgres"); err != nil {
				t.Fatalf("initial migration: %v", err)
			}
			if err := tx.Exec("DELETE FROM schema_migrations WHERE version = ?", "000010_satellite_tile_zoom.sql").Error; err != nil {
				t.Fatal(err)
			}
			for _, column := range test.missing {
				if err := tx.Exec("ALTER TABLE project_ndvi_summaries DROP COLUMN " + column).Error; err != nil {
					t.Fatal(err)
				}
			}

			now := time.Now().UTC()
			user := models.User{ID: uuid.NewString(), Email: "upgrade@example.test", PasswordHash: "test",
				DisplayName: "Upgrade test", Role: models.RoleManager, Status: models.IdentityActive,
				CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			project := models.ConservationProject{ID: uuid.NewString(), Name: "Upgrade test", ManagerID: user.ID,
				MethodologyID: "c6454811-b062-43b6-bbb7-5c380b8e4ca5", Status: "active", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&project).Error; err != nil {
				t.Fatal(err)
			}
			minZoom, maxZoom := 13, 15
			stored := models.ProjectNdviSummary{ID: uuid.NewString(), ProjectID: project.ID,
				PeriodStart: now.Add(-time.Hour), PeriodEnd: now, CurrentSceneID: "S2_UPGRADE",
				Quality: "complete", AdapterMode: "live", Provider: "opendata-s2",
				Result: datatypes.JSON([]byte(`{"scene":"S2_UPGRADE"}`)), ResultHash: "sha256:preserve-this",
				ProcessingVersion: "test", TileMinZoom: &minZoom, TileMaxZoom: &maxZoom, ComputedAt: now, CreatedAt: now}
			if err := tx.Omit(test.missing...).Create(&stored).Error; err != nil {
				t.Fatalf("seed historical summary: %v", err)
			}
			fresh := stored
			fresh.ID = uuid.NewString()
			if len(test.missing) > 0 {
				if err := tx.SavePoint("before_failed_insert").Error; err != nil {
					t.Fatal(err)
				}
				if err := tx.Create(&fresh).Error; !errors.Is(err, gorm.ErrInvalidField) {
					t.Fatalf("unmigrated insert = %v, want invalid field", err)
				}
				if err := tx.RollbackTo("before_failed_insert").Error; err != nil {
					t.Fatal(err)
				}
			}

			for attempt := 0; attempt < 2; attempt++ {
				if err := migrate(tx, "postgres"); err != nil {
					t.Fatalf("upgrade attempt %d: %v", attempt, err)
				}
			}
			if err := tx.Create(&fresh).Error; err != nil {
				t.Fatalf("insert after upgrade: %v", err)
			}
			var loaded models.ProjectNdviSummary
			if err := tx.First(&loaded, "id = ?", stored.ID).Error; err != nil {
				t.Fatal(err)
			}
			if loaded.ResultHash != stored.ResultHash || string(loaded.Result) != `{"scene": "S2_UPGRADE"}` {
				t.Fatalf("historical result changed: %s %s", loaded.ResultHash, loaded.Result)
			}
			expectedZoom := map[string]int{"tile_min_zoom": minZoom, "tile_max_zoom": maxZoom}
			for column, value := range map[string]*int{"tile_min_zoom": loaded.TileMinZoom, "tile_max_zoom": loaded.TileMaxZoom} {
				missing := false
				for _, absent := range test.missing {
					missing = missing || column == absent
				}
				if missing && value != nil {
					t.Errorf("new column %s invented a rendered zoom", column)
				} else if !missing && (value == nil || *value != expectedZoom[column]) {
					t.Errorf("existing column %s lost its value", column)
				}
			}
			var applied int64
			if err := tx.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", "000010_satellite_tile_zoom.sql").Scan(&applied).Error; err != nil || applied != 1 {
				t.Fatalf("migration recorded %d times: %v", applied, err)
			}
		})
	}
}

func TestMigrationIsIdempotent(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "repeat.db") + "?_busy_timeout=5000"
	for attempt := 0; attempt < 2; attempt++ {
		database, err := Open(dsn, false)
		if err != nil {
			t.Fatalf("open pass %d: %v", attempt, err)
		}
		if err := database.Exec("SELECT COUNT(*) FROM users").Error; err != nil {
			t.Fatalf("pass %d schema check: %v", attempt, err)
		}
	}
}

// TestSplitKeepsThePostgresDoBlockWhole pins the defect that stopped a fresh PostgreSQL
// database from booting. 000008 writes its conditional ALTER as a DO block, and a
// splitter that only knew about quotes cut it into "DO $$ BEGIN ...", "END IF" and
// "END $$", none of which a driver can execute. The SQLite drift test cannot see this:
// it never reads the postgres directory.
func TestSplitKeepsThePostgresDoBlockWhole(t *testing.T) {
	contents, err := migrationFiles.ReadFile("migrations/postgres/000008_base_methodology.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	statements := splitStatements(string(contents))
	if len(statements) != 2 {
		t.Fatalf("got %d statements, want the DO block and the INSERT: %q", len(statements), statements)
	}
	if !strings.Contains(statements[0], "DO $$") || !strings.HasSuffix(statements[0], "END $$") {
		t.Errorf("first statement is not one whole DO block: %q", statements[0])
	}
	if !strings.HasPrefix(statements[1], "INSERT INTO methodologies") {
		t.Errorf("second statement = %q", statements[1])
	}
}

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{"two statements", "SELECT 1; SELECT 2;", []string{"SELECT 1", "SELECT 2"}},
		{"no trailing semicolon", "SELECT 1", []string{"SELECT 1"}},
		{"empty", " ; \n; ", nil},
		{"semicolon in a literal", "INSERT INTO t VALUES ('a;b');", []string{"INSERT INTO t VALUES ('a;b')"}},
		{"doubled quote", "INSERT INTO t VALUES ('it''s; fine');", []string{"INSERT INTO t VALUES ('it''s; fine')"}},
		{
			"do block",
			"DO $$ BEGIN IF true THEN ALTER TABLE t ADD COLUMN c int; END IF; END $$; SELECT 1;",
			[]string{"DO $$ BEGIN IF true THEN ALTER TABLE t ADD COLUMN c int; END IF; END $$", "SELECT 1"},
		},
		{
			"tagged body with a quote inside",
			"CREATE FUNCTION f() RETURNS int AS $body$ BEGIN RAISE NOTICE 'hi; there'; RETURN 1; END; $body$ LANGUAGE plpgsql;",
			[]string{"CREATE FUNCTION f() RETURNS int AS $body$ BEGIN RAISE NOTICE 'hi; there'; RETURN 1; END; $body$ LANGUAGE plpgsql"},
		},
		{"parameter is not a quote", "SELECT $1; SELECT $2;", []string{"SELECT $1", "SELECT $2"}},
		{"unterminated body is left for the driver", "DO $$ BEGIN SELECT 1;", []string{"DO $$ BEGIN SELECT 1"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := splitStatements(testCase.sql)
			if len(got) != len(testCase.want) {
				t.Fatalf("got %d statements %q, want %d %q", len(got), got, len(testCase.want), testCase.want)
			}
			for i := range got {
				if got[i] != testCase.want[i] {
					t.Errorf("statement %d = %q, want %q", i, got[i], testCase.want[i])
				}
			}
		})
	}
}
