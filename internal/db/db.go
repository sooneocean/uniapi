package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Database struct {
	DB *sql.DB
}

func Open(dsn string) (*Database, error) {
	if dsn == "" {
		dsn = "file:uniapi.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
	} else if dsn == ":memory:" {
		dsn = "file::memory:?_foreign_keys=on"
	} else if !strings.Contains(dsn, "?") {
		dsn = fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dsn)
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// WAL mode: allow concurrent reads
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)

	database := &Database{DB: sqlDB}
	if err := database.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return database, nil
}

func (d *Database) migrate() error {
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

	for _, f := range upFiles {
		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := d.DB.Exec(string(content)); err != nil {
			return fmt.Errorf("execute migration %s: %w", f, err)
		}
	}

	return nil
}

func (d *Database) NeedsSetup() (bool, error) {
	var count int
	err := d.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}
