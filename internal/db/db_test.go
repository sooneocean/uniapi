package db

import (
	"testing"
)

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

func TestOAuthMigration(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var name string
	err = database.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='oauth_states'").Scan(&name)
	if err != nil {
		t.Error("oauth_states table not found")
	}

	// Insert with new columns (models as comma-separated, matching existing format)
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u1', 'test', 'hash', 'admin')")
	_, err = database.DB.Exec(`INSERT INTO accounts (id, provider, label, credential, models, auth_type, owner_user_id) VALUES ('a1', 'openai', 'test', 'enc', 'gpt-4o', 'session_token', 'u1')`)
	if err != nil {
		t.Fatalf("insert with new columns failed: %v", err)
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
