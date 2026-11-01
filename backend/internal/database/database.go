package database

import (
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/postgres/*.sql migrations/sqlite/*.sql
var migrationFiles embed.FS

func Open(dsn string, production bool) (*gorm.DB, error) {
	driver := "sqlite"
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		driver = "postgres"
	}
	if production && driver != "postgres" {
		return nil, fmt.Errorf("production requires PostgreSQL")
	}
	config := &gorm.Config{
		Logger: logger.New(slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn), logger.Config{
			SlowThreshold:             250 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
		TranslateError: true,
	}
	var db *gorm.DB
	var err error
	if driver == "postgres" {
		db, err = gorm.Open(postgres.Open(dsn), config)
	} else {
		db, err = gorm.Open(sqlite.Open(dsn), config)
	}
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if driver == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := migrate(db, driver); err != nil {
		return nil, err
	}
	return db, nil
}

// migrate applies every embedded migration for the chosen driver, oldest first,
// skipping those already recorded. Each file is one transaction, so a partially
// applied migration never records its version.
func migrate(db *gorm.DB, driver string) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP NOT NULL)`).Error; err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	entries, err := migrationFiles.ReadDir("migrations/" + driver)
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)

	for _, version := range versions {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		contents, err := migrationFiles.ReadFile("migrations/" + driver + "/" + version)
		if err != nil {
			return fmt.Errorf("read migration: %w", err)
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			// Some drivers (notably pgx) reject multi-statement prepared executions,
			// so each migration statement is applied on its own.
			for _, statement := range splitStatements(string(contents)) {
				if err := tx.Exec(statement).Error; err != nil {
					return fmt.Errorf("apply migration %s: %w", version, err)
				}
			}
			if err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, time.Now().UTC()).Error; err != nil {
				return fmt.Errorf("record migration: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func splitStatements(contents string) []string {
	var statements []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, character := range contents {
		switch {
		case escaped:
			escaped = false
		case quote != 0:
			if character == quote {
				quote = 0
			}
		case character == '\'' || character == '"':
			quote = character
		case character == '\\':
			escaped = true
		case character == ';':
			if statement := strings.TrimSpace(current.String()); statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
			continue
		}
		current.WriteRune(character)
	}
	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}
