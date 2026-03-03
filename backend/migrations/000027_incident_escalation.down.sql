ALTER TABLE escalation_states DROP CONSTRAINT IF EXISTS chk_escalation_states_source;
DROP INDEX IF EXISTS idx_escalation_states_incident_id;
ALTER TABLE escalation_states
    DROP COLUMN IF EXISTS incident_id,
    DROP COLUMN IF EXISTS source_type;
ALTER TABLE escalation_states ALTER COLUMN alert_id SET NOT NULL;
