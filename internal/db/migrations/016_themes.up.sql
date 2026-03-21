CREATE TABLE IF NOT EXISTS themes (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    colors      TEXT NOT NULL,
    shared      BOOLEAN NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_themes_user ON themes(user_id);
CREATE INDEX IF NOT EXISTS idx_themes_shared ON themes(shared);

ALTER TABLE users ADD COLUMN active_theme TEXT DEFAULT '';
