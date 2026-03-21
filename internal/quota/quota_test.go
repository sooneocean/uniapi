package quota

import (
	"testing"
	"time"

	"github.com/sooneocean/uniapi/internal/db"
)

func setupTestDB(t *testing.T) *db.Database {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}

func insertUser(t *testing.T, database *db.Database, userID string) {
	t.Helper()
	_, err := database.DB.Exec(
		"INSERT INTO users (id, username, password, role) VALUES (?, ?, 'hash', 'user')",
		userID, userID,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func insertUsage(t *testing.T, database *db.Database, userID string, cost float64, date string) {
	t.Helper()
	_, err := database.DB.Exec(`
		INSERT INTO usage_daily (id, user_id, provider, model, date, tokens_in, tokens_out, cost, request_count)
		VALUES (?, ?, 'test', 'test-model', ?, 0, 0, ?, 1)`,
		userID+"-"+date, userID, date, cost,
	)
	if err != nil {
		t.Fatalf("insert usage: %v", err)
	}
}

func defaultEngine(database *db.Database) *Engine {
	return NewEngine(database.DB, Config{
		DailyLimitUSD:   10.0,
		MonthlyLimitUSD: 100.0,
		WarnThreshold:   0.8,
	})
}

// Test 1: No quota set → Allowed=true
func TestCheckNoQuotaSet(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	engine := NewEngine(database.DB, Config{
		DailyLimitUSD:   0,
		MonthlyLimitUSD: 0,
		WarnThreshold:   0.8,
	})

	insertUser(t, database, "u1")
	result := engine.Check("u1")

	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Message)
	}
	if result.Warning {
		t.Errorf("expected Warning=false, got true")
	}
}

// Test 2: Daily quota exceeded → Allowed=false
func TestCheckDailyQuotaExceeded(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	engine := defaultEngine(database)
	insertUser(t, database, "u2")

	today := time.Now().Format("2006-01-02")
	insertUsage(t, database, "u2", 11.0, today) // over $10 daily limit

	result := engine.Check("u2")

	if result.Allowed {
		t.Errorf("expected Allowed=false (daily exceeded), got true")
	}
	if result.Warning {
		t.Errorf("expected Warning=false when blocked, got true")
	}
	if result.Message == "" {
		t.Errorf("expected non-empty Message")
	}
}

// Test 3: Daily quota at 80% → Warning=true
func TestCheckDailyQuotaWarning(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	engine := defaultEngine(database)
	insertUser(t, database, "u3")

	today := time.Now().Format("2006-01-02")
	insertUsage(t, database, "u3", 8.5, today) // $8.50 of $10.00 = 85% > 80%

	result := engine.Check("u3")

	if !result.Allowed {
		t.Errorf("expected Allowed=true at 85%%, got false: %s", result.Message)
	}
	if !result.Warning {
		t.Errorf("expected Warning=true at 85%%, got false")
	}
}

// Test 4: Monthly quota exceeded → Allowed=false
func TestCheckMonthlyQuotaExceeded(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	engine := defaultEngine(database)
	insertUser(t, database, "u4")

	// Insert costs across several days this month, total > $100
	today := time.Now()
	for i := 0; i < 5; i++ {
		day := today.AddDate(0, 0, -i).Format("2006-01-02")
		insertUsage(t, database, "u4", 21.0, day)
	}
	// total = 5 * $21 = $105 > $100 monthly

	result := engine.Check("u4")

	if result.Allowed {
		t.Errorf("expected Allowed=false (monthly exceeded), got true")
	}
	if result.Message == "" {
		t.Errorf("expected non-empty Message for monthly exceeded")
	}
}

// Test 5: SetUserQuota updates correctly
func TestSetUserQuota(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	engine := defaultEngine(database)
	insertUser(t, database, "u5")

	if err := engine.SetUserQuota("u5", 5.0, 50.0); err != nil {
		t.Fatalf("SetUserQuota: %v", err)
	}

	// Verify via getUserConfig
	cfg := engine.getUserConfig("u5")
	if cfg.DailyLimitUSD != 5.0 {
		t.Errorf("expected DailyLimitUSD=5.0, got %f", cfg.DailyLimitUSD)
	}
	if cfg.MonthlyLimitUSD != 50.0 {
		t.Errorf("expected MonthlyLimitUSD=50.0, got %f", cfg.MonthlyLimitUSD)
	}
}

// Test 6: Per-user quota overrides default
func TestCheckPerUserQuotaOverridesDefault(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Default is $10 daily
	engine := defaultEngine(database)
	insertUser(t, database, "u6")

	// Set user-specific limit of $2 daily
	if err := engine.SetUserQuota("u6", 2.0, 20.0); err != nil {
		t.Fatalf("SetUserQuota: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	insertUsage(t, database, "u6", 3.0, today) // $3 > $2 per-user limit

	result := engine.Check("u6")

	if result.Allowed {
		t.Errorf("expected Allowed=false (per-user daily $2 exceeded by $3), got true")
	}
}
