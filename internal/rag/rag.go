package rag

import (
	"database/sql"
	"strings"

	"github.com/google/uuid"
)

type Manager struct {
	db *sql.DB
}

type Document struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Title      string `json:"title"`
	Content    string `json:"content,omitempty"`
	ChunkCount int    `json:"chunk_count"`
	Shared     bool   `json:"shared"`
	CreatedAt  string `json:"created_at"`
}

type Chunk struct {
	ID         string `json:"id"`
	DocID      string `json:"doc_id"`
	Content    string `json:"content"`
	ChunkIndex int    `json:"chunk_index"`
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Upload stores a document and splits into chunks (~500 chars each with 50 char overlap)
func (m *Manager) Upload(userID, title, content string, shared bool) (*Document, error) {
	docID := uuid.New().String()
	chunks := splitIntoChunks(content, 500, 50)

	tx, err := m.db.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("INSERT INTO knowledge_docs (id, user_id, title, content, chunk_count, shared) VALUES (?,?,?,?,?,?)",
		docID, userID, title, content, len(chunks), shared)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	for i, chunk := range chunks {
		_, err = tx.Exec("INSERT INTO knowledge_chunks (id, doc_id, content, chunk_index) VALUES (?,?,?,?)",
			uuid.New().String(), docID, chunk, i)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	tx.Commit()
	return &Document{ID: docID, UserID: userID, Title: title, ChunkCount: len(chunks), Shared: shared}, nil
}

// Search finds relevant chunks using simple keyword matching (no embeddings needed)
func (m *Manager) Search(userID, query string, limit int) ([]Chunk, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil, nil
	}

	// Build LIKE conditions for each word
	conditions := make([]string, len(words))
	args := make([]interface{}, 0)
	for i, w := range words {
		conditions[i] = "LOWER(kc.content) LIKE ?"
		args = append(args, "%"+w+"%")
	}

	// Only search user's own docs + shared docs
	querySql := `
        SELECT kc.id, kc.doc_id, kc.content, kc.chunk_index
        FROM knowledge_chunks kc
        JOIN knowledge_docs kd ON kd.id = kc.doc_id
        WHERE (kd.user_id = ? OR kd.shared = 1)
        AND (` + strings.Join(conditions, " OR ") + `)
        LIMIT ?
    `
	args = append([]interface{}{userID}, args...)
	args = append(args, limit)

	rows, err := m.db.Query(querySql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Chunk
	for rows.Next() {
		var c Chunk
		rows.Scan(&c.ID, &c.DocID, &c.Content, &c.ChunkIndex)
		results = append(results, c)
	}
	return results, nil
}

func (m *Manager) ListDocs(userID string) ([]Document, error) {
	rows, err := m.db.Query(
		"SELECT id, user_id, title, chunk_count, shared, created_at FROM knowledge_docs WHERE user_id = ? OR shared = 1 ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var d Document
		rows.Scan(&d.ID, &d.UserID, &d.Title, &d.ChunkCount, &d.Shared, &d.CreatedAt)
		docs = append(docs, d)
	}
	return docs, nil
}

func (m *Manager) DeleteDoc(id, userID string) error {
	_, err := m.db.Exec("DELETE FROM knowledge_docs WHERE id = ? AND user_id = ?", id, userID)
	return err
}

func splitIntoChunks(text string, chunkSize, overlap int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}
	var chunks []string
	for i := 0; i < len(text); i += chunkSize - overlap {
		end := i + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[i:end])
		if end == len(text) {
			break
		}
	}
	return chunks
}
