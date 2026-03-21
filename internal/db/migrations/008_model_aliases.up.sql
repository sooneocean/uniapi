CREATE TABLE IF NOT EXISTS model_aliases (
    alias    TEXT PRIMARY KEY,
    model_id TEXT NOT NULL,
    user_id  TEXT REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
