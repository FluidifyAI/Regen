-- Create incident_alerts join table
CREATE TABLE IF NOT EXISTS incident_alerts (
    incident_id     UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    alert_id        UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    linked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    linked_by_type  VARCHAR(20) NOT NULL CHECK (linked_by_type IN ('system', 'user')),
    linked_by_id    VARCHAR(100),

    PRIMARY KEY (incident_id, alert_id)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_incident_alerts_incident_id ON incident_alerts(incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_alerts_alert_id ON incident_alerts(alert_id);

-- Comments
COMMENT ON TABLE incident_alerts IS 'Many-to-many relationship between incidents and alerts';
