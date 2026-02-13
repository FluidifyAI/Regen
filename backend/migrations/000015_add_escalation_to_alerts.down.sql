DROP INDEX IF EXISTS idx_alerts_acknowledgment_status;
DROP INDEX IF EXISTS idx_alerts_escalation_policy_id;

ALTER TABLE alerts
    DROP COLUMN IF EXISTS acknowledgment_status,
    DROP COLUMN IF EXISTS escalation_policy_id;
