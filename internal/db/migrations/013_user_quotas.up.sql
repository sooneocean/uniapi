ALTER TABLE users ADD COLUMN daily_token_limit INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN daily_cost_limit REAL DEFAULT 0;
ALTER TABLE users ADD COLUMN monthly_cost_limit REAL DEFAULT 0;
