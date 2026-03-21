CREATE TABLE IF NOT EXISTS prompt_templates (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL,
    description TEXT DEFAULT '',
    system_prompt TEXT NOT NULL,
    user_prompt  TEXT DEFAULT '',
    tags        TEXT DEFAULT '',
    shared      BOOLEAN NOT NULL DEFAULT 0,
    use_count   INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_templates_shared ON prompt_templates(shared);
CREATE INDEX IF NOT EXISTS idx_templates_user ON prompt_templates(user_id);
