package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
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
