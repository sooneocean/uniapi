CREATE TABLE IF NOT EXISTS audit_log (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    username   TEXT NOT NULL,
    action     TEXT NOT NULL,
    resource   TEXT NOT NULL,
    resource_id TEXT,
    details    TEXT,
    ip         TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
