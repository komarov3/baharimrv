package database

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/baharilink/baharimrv/internal/models"
	"gorm.io/gorm"
)

type tableColumnInfo struct {
	Name string
}

// TestMigrationMatchesModelColumns guards the hand-written migrations against the
// GORM schema: a column the model expects but the migration does not create fails
// at runtime as "no column named ...".
func TestMigrationMatchesModelColumns(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "drift.db") + "?_busy_timeout=5000"
	db, err := Open(dsn, false)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	for _, model := range models.All() {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(model); err != nil {
			t.Errorf("parse schema for %T: %v", model, err)
			continue
		}
		table := statement.Schema.Table

		var columns []tableColumnInfo
		if err := db.Raw("SELECT name FROM pragma_table_info(?)", table).Scan(&columns).Error; err != nil {
			t.Errorf("read columns for %s: %v", table, err)
			continue
		}
		if len(columns) == 0 {
			t.Errorf("table %s does not exist", table)
			continue
		}

		available := make(map[string]bool, len(columns))
		for _, column := range columns {
			available[column.Name] = true
		}

		var missing []string
		for _, field := range statement.Schema.Fields {
			if !available[field.DBName] {
				missing = append(missing, field.Name+" -> "+field.DBName)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("table %s is missing columns: %v", table, missing)
		}
	}
}
