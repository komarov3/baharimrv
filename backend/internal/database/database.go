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

// splitStatements cuts a migration file on the semicolons that end a statement. A
// semicolon inside a quoted literal or a dollar-quoted body does not: Postgres DO
// blocks and function bodies are written $$ ... $$ and carry their own semicolons, so
// a splitter that only knows about quotes hands the driver fragments of one statement.
func splitStatements(contents string) []string {
	runes := []rune(contents)
	var statements []string
	var current strings.Builder
	flush := func() {
		if statement := strings.TrimSpace(current.String()); statement != "" {
			statements = append(statements, statement)
		}
		current.Reset()
	}
	for i := 0; i < len(runes); i++ {
		switch character := runes[i]; {
		case character == '\'' || character == '"':
			end := quotedEnd(runes, i)
			current.WriteString(string(runes[i:end]))
			i = end - 1
		case character == '$':
			if end := dollarQuotedEnd(runes, i); end > i {
				current.WriteString(string(runes[i:end]))
				i = end - 1
			} else {
				current.WriteRune(character)
			}
		case character == ';':
			flush()
		default:
			current.WriteRune(character)
		}
	}
	flush()
	return statements
}

// quotedEnd returns the index just past a string literal, or the end of the file when
// the literal is never closed. A backslash escapes the next character.
func quotedEnd(runes []rune, start int) int {
	quote := runes[start]
	for i := start + 1; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(runes)
}

// dollarQuotedEnd returns the index just past a $tag$ ... $tag$ body, or 0 when the
// dollar sign opens nothing: $1 is a parameter, and an unterminated body is left for
// the driver to reject rather than silently swallowing the rest of the file.
func dollarQuotedEnd(runes []rune, start int) int {
	tag := []rune(dollarTag(runes, start))
	if len(tag) == 0 {
		return 0
	}
	for i := start + len(tag); i+len(tag) <= len(runes); i++ {
		if string(runes[i:i+len(tag)]) == string(tag) {
			return i + len(tag)
		}
	}
	return 0
}

// dollarTag reads the opening tag at start, both dollar signs included. The tag is
// empty ($$) or an identifier that may not begin with a digit.
func dollarTag(runes []rune, start int) string {
	i := start + 1
	if i < len(runes) && runes[i] == '$' {
		return "$$"
	}
	for i < len(runes) {
		character := runes[i]
		letter := character == '_' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
		digit := character >= '0' && character <= '9'
		if !letter && !(digit && i > start+1) {
			break
		}
		i++
	}
	if i > start+1 && i < len(runes) && runes[i] == '$' {
		return string(runes[start : i+1])
	}
	return ""
}
