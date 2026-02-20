ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS teams_channel_id   VARCHAR(255),
    ADD COLUMN IF NOT EXISTS teams_channel_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS teams_activity_id  VARCHAR(1024);

-- Index for bot command lookups: every ack/resolve/status command queries by teams_channel_id
CREATE INDEX IF NOT EXISTS idx_incidents_teams_channel_id ON incidents (teams_channel_id)
    WHERE teams_channel_id IS NOT NULL;
