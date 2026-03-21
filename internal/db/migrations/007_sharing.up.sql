ALTER TABLE conversations ADD COLUMN share_token TEXT;
CREATE INDEX IF NOT EXISTS idx_conversations_share ON conversations(share_token);
