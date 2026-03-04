CREATE TABLE IF NOT EXISTS slack_config (
    id                  INT PRIMARY KEY DEFAULT 1,
    bot_token           TEXT NOT NULL DEFAULT '',
    signing_secret      TEXT NOT NULL DEFAULT '',
    app_token           TEXT,
    workspace_id        TEXT,
    workspace_name      TEXT,
    bot_user_id         TEXT,
    oauth_client_id     TEXT,
    oauth_client_secret TEXT,
    connected_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    connected_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT slack_config_single_row CHECK (id = 1)
);
