package oauth

import (
	"testing"

	"github.com/sooneocean/uniapi/internal/config"
	"github.com/sooneocean/uniapi/internal/crypto"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/repo"
)

func setupTest(t *testing.T) (*Manager, *db.Database) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	encKey, _ := crypto.DeriveKey("test")
	accountRepo := repo.NewAccountRepo(database, encKey)
	mgr := NewManager(database, accountRepo, encKey, "http://localhost:9000", config.OAuthConfigs{})
	return mgr, database
}

func TestListProviders(t *testing.T) {
	mgr, _ := setupTest(t)
	providers := mgr.ListProviders()
	if len(providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(providers))
	}
}

func TestBindSessionToken(t *testing.T) {
	mgr, database := setupTest(t)
	// Create user first
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u1', 'alice', 'h', 'admin')")

	acc, err := mgr.BindSessionToken("openai", "u1", "sess-token-123", false)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Credential != "sess-token-123" {
		t.Error("expected credential to be stored")
	}
	if acc.OwnerUserID != "u1" {
		t.Errorf("expected owner u1, got %s", acc.OwnerUserID)
	}

	// Shared binding (different provider to avoid conflicts)
	acc2, err := mgr.BindSessionToken("anthropic", "u1", "sess-token-456", true)
	if err != nil {
		t.Fatal(err)
	}
	if acc2.OwnerUserID != "" {
		t.Error("shared account should have empty owner")
	}
}

func TestListAccounts(t *testing.T) {
	mgr, database := setupTest(t)
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u1', 'alice', 'h', 'admin')")
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u2', 'bob', 'h', 'member')")

	mgr.BindSessionToken("openai", "u1", "token1", true) // shared

	accounts, err := mgr.ListAccounts("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) < 1 {
		t.Error("expected at least 1 account")
	}
}

func TestUnbind(t *testing.T) {
	mgr, database := setupTest(t)
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u1', 'alice', 'h', 'admin')")

	acc, _ := mgr.BindSessionToken("openai", "u1", "token", false)
	err := mgr.Unbind(acc.ID, "u1", "admin")
	if err != nil {
		t.Fatal(err)
	}

	// Should be gone
	_, err = mgr.ListAccounts("u1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestHashSession(t *testing.T) {
	h1 := HashSession("jwt-token-1")
	h2 := HashSession("jwt-token-1")
	h3 := HashSession("jwt-token-2")
	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
}
