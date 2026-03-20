package repo

import (
	"testing"

	"github.com/user/uniapi/internal/crypto"
)

func testEncKey() []byte {
	key, err := crypto.DeriveKey("test-secret")
	if err != nil {
		panic(err)
	}
	return key
}

func TestAccountCreateStoresEncryptedCredential(t *testing.T) {
	database := setupTestDB(t)
	repo := NewAccountRepo(database, testEncKey())

	acc, err := repo.Create("openai", "My OpenAI Key", "sk-test-key", []string{"gpt-4", "gpt-3.5"}, 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if acc.ID == "" {
		t.Error("expected non-empty ID")
	}
	if acc.Credential != "sk-test-key" {
		t.Errorf("expected decrypted credential 'sk-test-key', got %s", acc.Credential)
	}

	// Check that the raw value in DB is NOT the plaintext
	var rawCred string
	err = database.DB.QueryRow("SELECT credential FROM accounts WHERE id = ?", acc.ID).Scan(&rawCred)
	if err != nil {
		t.Fatal(err)
	}
	if rawCred == "sk-test-key" {
		t.Error("credential should be encrypted in DB, not stored as plaintext")
	}
	if rawCred == "" {
		t.Error("encrypted credential should not be empty")
	}
}

func TestAccountGetByIDReturnsDecrypted(t *testing.T) {
	database := setupTestDB(t)
	repo := NewAccountRepo(database, testEncKey())

	created, err := repo.Create("anthropic", "Claude Key", "sk-ant-secret", []string{"claude-3"}, 3, false)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "sk-ant-secret" {
		t.Errorf("expected decrypted 'sk-ant-secret', got %s", got.Credential)
	}
	if got.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %s", got.Provider)
	}
	if got.Label != "Claude Key" {
		t.Errorf("expected label 'Claude Key', got %s", got.Label)
	}
	if len(got.Models) != 1 || got.Models[0] != "claude-3" {
		t.Errorf("expected models ['claude-3'], got %v", got.Models)
	}
	if got.MaxConcurrent != 3 {
		t.Errorf("expected max_concurrent 3, got %d", got.MaxConcurrent)
	}
	if !got.Enabled {
		t.Error("expected account to be enabled by default")
	}
}

func TestAccountListAll(t *testing.T) {
	database := setupTestDB(t)
	repo := NewAccountRepo(database, testEncKey())

	_, err := repo.Create("openai", "Key 1", "key-one", []string{"gpt-4"}, 5, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Create("anthropic", "Key 2", "key-two", []string{"claude-3"}, 3, true)
	if err != nil {
		t.Fatal(err)
	}

	accounts, err := repo.ListAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}

	// Verify both credentials are decrypted
	creds := map[string]bool{}
	for _, a := range accounts {
		creds[a.Credential] = true
	}
	if !creds["key-one"] {
		t.Error("expected 'key-one' in decrypted credentials")
	}
	if !creds["key-two"] {
		t.Error("expected 'key-two' in decrypted credentials")
	}
}

func TestAccountDelete(t *testing.T) {
	database := setupTestDB(t)
	repo := NewAccountRepo(database, testEncKey())

	acc, err := repo.Create("openai", "To Delete", "sk-delete-me", []string{"gpt-4"}, 5, false)
	if err != nil {
		t.Fatal(err)
	}

	err = repo.Delete(acc.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.GetByID(acc.ID)
	if err == nil {
		t.Error("deleted account should not be found")
	}
}

func TestAccountSetEnabled(t *testing.T) {
	database := setupTestDB(t)
	repo := NewAccountRepo(database, testEncKey())

	acc, err := repo.Create("openai", "Toggle Me", "sk-toggle", []string{"gpt-4"}, 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if !acc.Enabled {
		t.Error("expected account enabled by default")
	}

	// Disable
	err = repo.SetEnabled(acc.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Error("expected account to be disabled")
	}

	// Re-enable
	err = repo.SetEnabled(acc.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Enabled {
		t.Error("expected account to be re-enabled")
	}
}

func TestAccountGetByIDNotFound(t *testing.T) {
	database := setupTestDB(t)
	repo := NewAccountRepo(database, testEncKey())

	_, err := repo.GetByID("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent account")
	}
}
