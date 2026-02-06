-- Create alerts table
-- This table stores alerts received from monitoring systems (Prometheus, Grafana, etc.)
-- The received_at field is IMMUTABLE for audit compliance

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(255) NOT NULL,
    source VARCHAR(100) NOT NULL,
    fingerprint VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'firing',
    severity VARCHAR(50) NOT NULL DEFAULT 'info',
    title VARCHAR(500) NOT NULL,
    description TEXT,
    labels JSONB DEFAULT '{}'::jsonb,
    annotations JSONB DEFAULT '{}'::jsonb,
    raw_payload JSONB,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT alerts_status_check CHECK (status IN ('firing', 'resolved')),
    CONSTRAINT alerts_severity_check CHECK (severity IN ('critical', 'warning', 'info')),
    CONSTRAINT alerts_source_external_id_unique UNIQUE (source, external_id)
);

-- Indexes for performance
CREATE INDEX idx_alerts_source ON alerts(source);
CREATE INDEX idx_alerts_status ON alerts(status);
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_received_at ON alerts(received_at DESC);
CREATE INDEX idx_alerts_external_id ON alerts(external_id);

-- Trigger function to prevent updates to received_at (immutability)
CREATE OR REPLACE FUNCTION prevent_received_at_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.received_at IS DISTINCT FROM NEW.received_at THEN
        RAISE EXCEPTION 'received_at is immutable and cannot be modified';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to enforce received_at immutability
CREATE TRIGGER trigger_prevent_received_at_update
    BEFORE UPDATE ON alerts
    FOR EACH ROW
    EXECUTE FUNCTION prevent_received_at_update();

-- Comments for documentation
COMMENT ON TABLE alerts IS 'Stores alerts from monitoring systems with immutable audit trail';
COMMENT ON COLUMN alerts.received_at IS 'IMMUTABLE: Server-generated timestamp when alert was received';
