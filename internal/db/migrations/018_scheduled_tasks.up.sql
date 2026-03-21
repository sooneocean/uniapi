CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    model       TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    system_prompt TEXT DEFAULT '',
    cron_expr   TEXT DEFAULT '',
    run_at      DATETIME,
    last_run    DATETIME,
    result      TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_scheduled_user ON scheduled_tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_status ON scheduled_tasks(status, run_at);
