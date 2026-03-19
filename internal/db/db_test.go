package db

import "testing"

func TestOpenAndMigrate(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	tables := []string{"users", "accounts", "conversations", "messages", "usage_daily", "api_keys"}
	for _, table := range tables {
		var name string
		err := database.DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestNeedsSetup(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	needs, err := database.NeedsSetup()
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Error("fresh database should need setup")
	}
}
