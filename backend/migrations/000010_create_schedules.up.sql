-- Create schedules table for on-call schedule definitions.
--
-- Design decisions:
--   - notification_channel is free-text (Slack channel name, webhook URL, etc.)
--   - timezone is IANA string — validated in application layer, not DB
--   - Layers and participants are in separate tables (000011)

CREATE TABLE schedules (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name                 VARCHAR(255) NOT NULL,
    description          TEXT         NOT NULL DEFAULT '',
    timezone             VARCHAR(100) NOT NULL DEFAULT 'UTC',
    notification_channel VARCHAR(255) NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_schedules_name ON schedules (name);

COMMENT ON TABLE schedules IS 'On-call schedule definitions (v0.4)';
COMMENT ON COLUMN schedules.timezone IS 'IANA timezone string for shift boundary calculations (e.g., America/New_York)';
COMMENT ON COLUMN schedules.notification_channel IS 'Optional Slack channel or destination for shift-change notifications';
