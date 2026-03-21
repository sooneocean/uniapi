package usage

import (
	"testing"
	"time"

	"github.com/sooneocean/uniapi/internal/db"
)

func TestCalculateCost_KnownModel(t *testing.T) {
	// gpt-4o: InputPerM=2.5, OutputPerM=10.0
	// 1M tokens in = $2.50, 1M tokens out = $10.00
	cost := CalculateCost("gpt-4o", 1_000_000, 1_000_000)
	expected := 12.5
	if cost != expected {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestCalculateCost_Partial(t *testing.T) {
	// 500k tokens in = $1.25, 200k tokens out = $2.00
	cost := CalculateCost("gpt-4o", 500_000, 200_000)
	expected := 1.25 + 2.0
	if cost != expected {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	cost := CalculateCost("unknown-model-xyz", 1_000_000, 1_000_000)
	if cost != 0 {
		t.Errorf("expected 0 for unknown model, got %f", cost)
	}
}

func openTestDB(t *testing.T) *db.Database {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestRecordAndGetUserUsage(t *testing.T) {
	database := openTestDB(t)

	// Create a user first (usage_daily has FK to users)
	_, err := database.DB.Exec(
		"INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"user-1", "testuser", "hash", "member", time.Now(),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	recorder := NewRecorder(database.DB)

	rec := UsageRecord{
		UserID:    "user-1",
		Model:     "gpt-4o",
		Provider:  "openai",
		TokensIn:  100,
		TokensOut: 50,
		Cost:      0.001,
		LatencyMs: 200,
	}

	recorder.RecordUsage(rec)
	recorder.Stop()

	from := time.Now().AddDate(0, 0, -1)
	to := time.Now().AddDate(0, 0, 1)

	results, err := recorder.GetUserUsage("user-1", from, to)
	if err != nil {
		t.Fatalf("GetUserUsage: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Provider != "openai" {
		t.Errorf("expected provider openai, got %s", r.Provider)
	}
	if r.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", r.Model)
	}
	if r.TokensIn != 100 {
		t.Errorf("expected 100 tokens_in, got %d", r.TokensIn)
	}
	if r.TokensOut != 50 {
		t.Errorf("expected 50 tokens_out, got %d", r.TokensOut)
	}
	if r.RequestCount != 1 {
		t.Errorf("expected request_count 1, got %d", r.RequestCount)
	}
}

func TestRecorderFlushOnStop(t *testing.T) {
	database, _ := db.Open(":memory:")
	defer database.Close()
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u1', 'alice', 'h', 'admin')")

	recorder := NewRecorder(database.DB)
	recorder.RecordUsage(UsageRecord{UserID: "u1", Provider: "openai", Model: "gpt-4o", TokensIn: 100, TokensOut: 50, Cost: 0.001})
	recorder.RecordUsage(UsageRecord{UserID: "u1", Provider: "openai", Model: "gpt-4o", TokensIn: 200, TokensOut: 100, Cost: 0.002})

	// Stop should flush
	recorder.Stop()

	// Verify data was flushed
	var count int
	database.DB.QueryRow("SELECT request_count FROM usage_daily WHERE user_id = 'u1' AND model = 'gpt-4o'").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 requests flushed, got %d", count)
	}
}

func TestRecordUsage_Upsert(t *testing.T) {
	database := openTestDB(t)

	_, err := database.DB.Exec(
		"INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"user-2", "testuser2", "hash", "member", time.Now(),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	recorder := NewRecorder(database.DB)

	rec := UsageRecord{
		UserID:    "user-2",
		Model:     "gpt-4o",
		Provider:  "openai",
		TokensIn:  100,
		TokensOut: 50,
		Cost:      0.001,
	}

	// Record twice - should upsert (same user/model/date)
	recorder.RecordUsage(rec)
	recorder.RecordUsage(rec)
	recorder.Stop()

	from := time.Now().AddDate(0, 0, -1)
	to := time.Now().AddDate(0, 0, 1)

	results, err := recorder.GetUserUsage("user-2", from, to)
	if err != nil {
		t.Fatalf("GetUserUsage: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result after upsert, got %d", len(results))
	}

	r := results[0]
	if r.TokensIn != 200 {
		t.Errorf("expected tokens_in=200 after upsert, got %d", r.TokensIn)
	}
	if r.TokensOut != 100 {
		t.Errorf("expected tokens_out=100 after upsert, got %d", r.TokensOut)
	}
	if r.RequestCount != 2 {
		t.Errorf("expected request_count=2 after upsert, got %d", r.RequestCount)
	}
}
