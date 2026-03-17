CREATE TABLE IF NOT EXISTS telegram_config (
    id           INT PRIMARY KEY DEFAULT 1,
    bot_token    TEXT NOT NULL DEFAULT '',
    chat_id      TEXT NOT NULL DEFAULT '',
    chat_name    TEXT,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    connected_by UUID REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT telegram_config_single_row CHECK (id = 1)
);
