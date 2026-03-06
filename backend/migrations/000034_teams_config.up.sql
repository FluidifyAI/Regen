CREATE TABLE IF NOT EXISTS teams_config (
    id             INT PRIMARY KEY DEFAULT 1,
    app_id         TEXT NOT NULL DEFAULT '',
    app_password   TEXT NOT NULL DEFAULT '',
    tenant_id      TEXT NOT NULL DEFAULT '',
    team_id        TEXT NOT NULL DEFAULT '',
    bot_user_id    TEXT,
    service_url    TEXT NOT NULL DEFAULT 'https://smba.trafficmanager.net/amer/',
    team_name      TEXT,
    connected_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    connected_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT teams_config_single_row CHECK (id = 1)
);
