ALTER TABLE schedules
    ADD COLUMN IF NOT EXISTS default_escalation_policy_id UUID
        REFERENCES escalation_policies(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_schedules_default_escalation_policy
    ON schedules (default_escalation_policy_id)
    WHERE default_escalation_policy_id IS NOT NULL;
