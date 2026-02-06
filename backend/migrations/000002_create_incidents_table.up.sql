-- Create incidents table
CREATE TABLE IF NOT EXISTS incidents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_number     SERIAL UNIQUE NOT NULL,
    title               VARCHAR(500) NOT NULL,
    slug                VARCHAR(100) NOT NULL UNIQUE,
    status              VARCHAR(20) NOT NULL DEFAULT 'triggered' CHECK (status IN ('triggered', 'acknowledged', 'resolved', 'canceled')),
    severity            VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK (severity IN ('critical', 'high', 'medium', 'low')),
    summary             TEXT,

    -- Slack
    slack_channel_id    VARCHAR(50),
    slack_channel_name  VARCHAR(100),

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    triggered_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at     TIMESTAMPTZ,
    resolved_at         TIMESTAMPTZ,

    -- Ownership
    created_by_type     VARCHAR(20) NOT NULL CHECK (created_by_type IN ('system', 'user')),
    created_by_id       VARCHAR(100),
    commander_id        UUID,

    -- Metadata
    labels              JSONB DEFAULT '{}',
    custom_fields       JSONB DEFAULT '{}'
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
CREATE INDEX IF NOT EXISTS idx_incidents_triggered_at ON incidents(triggered_at);
CREATE INDEX IF NOT EXISTS idx_incidents_incident_number ON incidents(incident_number);

-- Ensure created_at and triggered_at are immutable
CREATE OR REPLACE FUNCTION prevent_incident_timestamp_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.created_at IS DISTINCT FROM NEW.created_at THEN
        RAISE EXCEPTION 'created_at is immutable and cannot be updated';
    END IF;
    IF OLD.triggered_at IS DISTINCT FROM NEW.triggered_at THEN
        RAISE EXCEPTION 'triggered_at is immutable and cannot be updated';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_prevent_incident_timestamp_update
    BEFORE UPDATE ON incidents
    FOR EACH ROW
    EXECUTE FUNCTION prevent_incident_timestamp_update();

-- Comments
COMMENT ON TABLE incidents IS 'Incidents tracked in the system';
COMMENT ON COLUMN incidents.incident_number IS 'Auto-incrementing human-readable incident number';
COMMENT ON COLUMN incidents.created_at IS 'Server-generated timestamp, immutable';
COMMENT ON COLUMN incidents.triggered_at IS 'Server-generated timestamp, immutable';
