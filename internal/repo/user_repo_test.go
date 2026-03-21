package repo

import (
	"testing"
	"github.com/sooneocean/uniapi/internal/db"
)

func setupTestDB(t *testing.T) *db.Database {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestCreateAndGetUser(t *testing.T) {
	database := setupTestDB(t)
	repo := NewUserRepo(database)
	user, err := repo.Create("alice", "hashed-password", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "alice" {
		t.Errorf("expected alice, got %s", user.Username)
	}
	if user.Role != "admin" {
		t.Errorf("expected admin, got %s", user.Role)
	}
	got, err := repo.GetByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != user.ID {
		t.Error("IDs don't match")
	}
}

func TestCreateDuplicateUsername(t *testing.T) {
	database := setupTestDB(t)
	repo := NewUserRepo(database)
	_, err := repo.Create("alice", "hash1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Create("alice", "hash2", "member")
	if err == nil {
		t.Error("duplicate username should fail")
	}
}

func TestListUsers(t *testing.T) {
	database := setupTestDB(t)
	repo := NewUserRepo(database)
	repo.Create("alice", "h1", "admin")
	repo.Create("bob", "h2", "member")
	users, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestDeleteUser(t *testing.T) {
	database := setupTestDB(t)
	repo := NewUserRepo(database)
	user, _ := repo.Create("alice", "h1", "member")
	err := repo.Delete(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.GetByUsername("alice")
	if err == nil {
		t.Error("deleted user should not be found")
	}
}
