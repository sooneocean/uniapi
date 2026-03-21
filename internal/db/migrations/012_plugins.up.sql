CREATE TABLE IF NOT EXISTS plugins (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    endpoint    TEXT NOT NULL,
    method      TEXT NOT NULL DEFAULT 'POST',
    headers     TEXT DEFAULT '{}',
    input_schema TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT 1,
    shared      BOOLEAN NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);
