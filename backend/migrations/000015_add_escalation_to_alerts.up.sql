-- Extend alerts with escalation policy linkage and acknowledgment status.
--
-- escalation_policy_id: set by the routing engine when a matching rule has an
--   escalation_policy_id action.  NULL means no escalation is configured for
--   this alert.  ON DELETE SET NULL preserves the alert if the policy is later
--   deleted.
--
-- acknowledgment_status: tracks the alert-level ack state driven by the
--   escalation engine.  Default 'pending' (no escalation triggered or no ack
--   yet).  This field is denormalized from escalation_states for fast querying
--   without a join.

ALTER TABLE alerts
    ADD COLUMN IF NOT EXISTS escalation_policy_id  UUID
        REFERENCES escalation_policies(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS acknowledgment_status VARCHAR(50) NOT NULL DEFAULT 'pending'
        CHECK (acknowledgment_status IN ('pending', 'acknowledged', 'timed_out'));

CREATE INDEX idx_alerts_escalation_policy_id ON alerts (escalation_policy_id)
    WHERE escalation_policy_id IS NOT NULL;

CREATE INDEX idx_alerts_acknowledgment_status ON alerts (acknowledgment_status)
    WHERE acknowledgment_status != 'pending';

COMMENT ON COLUMN alerts.escalation_policy_id IS 'FK to escalation_policies; set by routing engine when a policy applies';
COMMENT ON COLUMN alerts.acknowledgment_status IS 'Denormalized ack state: pending (default), acknowledged, timed_out';
