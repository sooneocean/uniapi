package rag

import (
	"testing"

	"github.com/sooneocean/uniapi/internal/db"
)

func setupRAG(t *testing.T) *Manager {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u1', 'alice', 'h', 'admin')")
	return NewManager(database.DB)
}

func TestUploadAndSearch(t *testing.T) {
	m := setupRAG(t)
	doc, err := m.Upload("u1", "Go Tutorial", "Go is a statically typed compiled language. It has garbage collection and concurrency support.", false)
	if err != nil {
		t.Fatal(err)
	}
	if doc.ChunkCount == 0 {
		t.Error("expected at least 1 chunk")
	}

	results, err := m.Search("u1", "concurrency", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected search results for 'concurrency'")
	}
}

func TestSearchNoResults(t *testing.T) {
	m := setupRAG(t)
	m.Upload("u1", "Test", "Hello world", false)
	results, err := m.Search("u1", "quantum physics", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestSharedDocs(t *testing.T) {
	m := setupRAG(t)
	m.db.Exec("INSERT INTO users (id, username, password, role) VALUES ('u2', 'bob', 'h', 'member')")
	m.Upload("u1", "Shared Doc", "This is shared content about databases", true)

	// u2 should find shared doc
	results, err := m.Search("u2", "databases", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("u2 should find shared doc")
	}
}

func TestListDocs(t *testing.T) {
	m := setupRAG(t)
	m.Upload("u1", "Doc 1", "content one", false)
	m.Upload("u1", "Doc 2", "content two", false)
	docs, err := m.ListDocs("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2, got %d", len(docs))
	}
}

func TestDeleteDoc(t *testing.T) {
	m := setupRAG(t)
	doc, _ := m.Upload("u1", "Temp", "temporary", false)
	err := m.DeleteDoc(doc.ID, "u1")
	if err != nil {
		t.Fatal(err)
	}
	docs, _ := m.ListDocs("u1")
	if len(docs) != 0 {
		t.Error("expected 0 docs after delete")
	}
}

func TestSplitIntoChunks(t *testing.T) {
	// Short text: single chunk
	chunks := splitIntoChunks("short", 500, 50)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}

	// Long text: multiple chunks
	long := make([]byte, 1200)
	for i := range long {
		long[i] = 'a'
	}
	chunks = splitIntoChunks(string(long), 500, 50)
	if len(chunks) < 2 {
		t.Errorf("expected 2+ chunks, got %d", len(chunks))
	}
}
