-- Allow incident-sourced escalation states (alert_id becomes nullable).
-- PostgreSQL unique constraints treat NULL != NULL so the existing UNIQUE on
-- alert_id already permits multiple NULL rows — no extra change needed there.
-- We just drop NOT NULL and add the incident FK + source discriminator.

ALTER TABLE escalation_states
    ALTER COLUMN alert_id DROP NOT NULL;

ALTER TABLE escalation_states
    ADD COLUMN IF NOT EXISTS incident_id UUID
        REFERENCES incidents(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(20)
        NOT NULL DEFAULT 'alert'
        CHECK (source_type IN ('alert', 'incident'));

-- Exactly one of alert_id / incident_id must be populated.
ALTER TABLE escalation_states
    ADD CONSTRAINT chk_escalation_states_source
        CHECK (
            (source_type = 'alert'    AND alert_id    IS NOT NULL AND incident_id IS NULL) OR
            (source_type = 'incident' AND incident_id IS NOT NULL AND alert_id    IS NULL)
        );

CREATE INDEX IF NOT EXISTS idx_escalation_states_incident_id
    ON escalation_states (incident_id)
    WHERE incident_id IS NOT NULL;
