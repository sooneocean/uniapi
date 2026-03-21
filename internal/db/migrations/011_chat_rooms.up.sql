CREATE TABLE IF NOT EXISTS chat_rooms (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chat_room_members (
    room_id     TEXT NOT NULL REFERENCES chat_rooms(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id),
    joined_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (room_id, user_id)
);

CREATE TABLE IF NOT EXISTS chat_room_messages (
    id          TEXT PRIMARY KEY,
    room_id     TEXT NOT NULL REFERENCES chat_rooms(id) ON DELETE CASCADE,
    user_id     TEXT REFERENCES users(id),
    username    TEXT NOT NULL,
    role        TEXT NOT NULL,
    content     TEXT NOT NULL,
    model       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_room_messages ON chat_room_messages(room_id, created_at);
