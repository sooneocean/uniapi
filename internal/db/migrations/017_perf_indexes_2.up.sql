CREATE INDEX IF NOT EXISTS idx_model_aliases_user ON model_aliases(user_id);
CREATE INDEX IF NOT EXISTS idx_plugins_user ON plugins(user_id);
CREATE INDEX IF NOT EXISTS idx_workflows_user ON workflows(user_id);
CREATE INDEX IF NOT EXISTS idx_themes_user ON themes(user_id);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_user ON prompt_templates(user_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_docs_user ON knowledge_docs(user_id);
CREATE INDEX IF NOT EXISTS idx_room_messages_room ON chat_room_messages(room_id, created_at);
