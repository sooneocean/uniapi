ALTER TABLE accounts ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'api_key';
ALTER TABLE accounts ADD COLUMN oauth_provider TEXT;
ALTER TABLE accounts ADD COLUMN refresh_token TEXT;
ALTER TABLE accounts ADD COLUMN token_expires_at DATETIME;
ALTER TABLE accounts ADD COLUMN owner_user_id TEXT REFERENCES users(id);
ALTER TABLE accounts ADD COLUMN needs_reauth BOOLEAN NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS oauth_states (
    state        TEXT PRIMARY KEY,
    provider     TEXT NOT NULL,
    user_id      TEXT NOT NULL REFERENCES users(id),
    session_hash TEXT NOT NULL,
    shared       BOOLEAN NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
