-- Create timeline_entries table
CREATE TABLE IF NOT EXISTS timeline_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id     UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    type            VARCHAR(50) NOT NULL,
    actor_type      VARCHAR(20) NOT NULL CHECK (actor_type IN ('user', 'system', 'slack_bot')),
    actor_id        VARCHAR(100),
    content         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_timeline_incident_id ON timeline_entries(incident_id);
CREATE INDEX IF NOT EXISTS idx_timeline_incident_timestamp ON timeline_entries(incident_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_timeline_type ON timeline_entries(type);

-- Prevent UPDATE operations on timeline_entries (immutable audit log)
CREATE OR REPLACE FUNCTION prevent_timeline_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'timeline_entries are immutable and cannot be updated';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_prevent_timeline_update
    BEFORE UPDATE ON timeline_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_timeline_update();

-- Prevent DELETE operations on timeline_entries (immutable audit log)
CREATE OR REPLACE FUNCTION prevent_timeline_delete()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'timeline_entries are immutable and cannot be deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_prevent_timeline_delete
    BEFORE DELETE ON timeline_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_timeline_delete();

-- Ensure timestamp and created_at are immutable (redundant since UPDATE is prevented, but explicit)
CREATE OR REPLACE FUNCTION prevent_timeline_timestamp_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.timestamp IS DISTINCT FROM NEW.timestamp THEN
        RAISE EXCEPTION 'timestamp is immutable and cannot be updated';
    END IF;
    IF OLD.created_at IS DISTINCT FROM NEW.created_at THEN
        RAISE EXCEPTION 'created_at is immutable and cannot be updated';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Comments
COMMENT ON TABLE timeline_entries IS 'Immutable audit log of incident timeline events - cannot be updated or deleted';
COMMENT ON COLUMN timeline_entries.timestamp IS 'Server-generated timestamp, immutable';
COMMENT ON COLUMN timeline_entries.created_at IS 'Server-generated timestamp, immutable';
