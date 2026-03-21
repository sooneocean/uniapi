package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Database wraps a SQLite connection and manages schema migrations.
type Database struct {
	DB   *sql.DB
	path string
}

// Open opens (or creates) the SQLite database at dsn and applies any pending migrations.
func Open(dsn string) (*Database, error) {
	rawPath := dsn
	if dsn == "" {
		dsn = "file:uniapi.db?_journal_mode=WAL&_busy_timeout=3000&_foreign_keys=on"
		rawPath = "uniapi.db"
	} else if dsn == ":memory:" {
		dsn = "file::memory:?_foreign_keys=on"
		rawPath = ":memory:"
	} else if !strings.Contains(dsn, "?") {
		dsn = fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=3000&_foreign_keys=on", dsn)
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// WAL mode: allow concurrent reads
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	database := &Database{DB: sqlDB, path: rawPath}
	if err := database.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return database, nil
}

func (d *Database) Path() string { return d.path }

func (d *Database) migrate() error {
	// Ensure schema_version table exists
	d.DB.Exec("CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY)")

	// Get current version
	var currentVersion int
	d.DB.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}

	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for i, f := range upFiles {
		version := i + 1
		if version <= currentVersion {
			continue
		}

		tx, err := d.DB.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration %s: %w", f, err)
		}

		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", f, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record version %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", f, err)
		}
	}
	return nil
}

// NeedsSetup returns true when no admin user exists (first-run setup required).
func (d *Database) NeedsSetup() (bool, error) {
	var count int
	err := d.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// Close closes the underlying database connection.
func (d *Database) Close() error {
	return d.DB.Close()
}
