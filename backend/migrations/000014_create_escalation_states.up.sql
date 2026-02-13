-- Create escalation_states table.
--
-- Design decisions:
--   - One row per alert that has an active escalation in progress.
--   - current_tier_index tracks which tier was last notified.
--   - last_notified_at is used by the worker to determine when to advance
--     to the next tier (last_notified_at + timeout_seconds < NOW()).
--   - acknowledged_at / acknowledged_by / acknowledged_via record who stopped
--     the escalation, forming part of the immutable audit trail.
--   - status transitions: pending → notified → acknowledged | completed.
--     "completed" covers both acknowledged and alert-resolved cases.
--   - A unique constraint on alert_id prevents duplicate escalation rows for
--     the same alert (idempotent trigger).

CREATE TABLE escalation_states (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id            UUID        NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    policy_id           UUID        NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,

    current_tier_index  INTEGER     NOT NULL DEFAULT 0 CHECK (current_tier_index >= 0),
    status              VARCHAR(50) NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending', 'notified', 'acknowledged', 'completed')),

    -- Populated when the worker fires notifications for the current tier.
    last_notified_at    TIMESTAMPTZ,

    -- Populated when a user or system acknowledges the alert.
    acknowledged_at     TIMESTAMPTZ,
    acknowledged_by     VARCHAR(255),
    acknowledged_via    VARCHAR(50)
                          CHECK (acknowledged_via IS NULL OR
                                 acknowledged_via IN ('slack', 'api', 'cli')),

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Enforce at most one escalation row per alert.
    CONSTRAINT uq_escalation_states_alert_id UNIQUE (alert_id)
);

-- Hot-path query: worker polls for unacknowledged states where the current
-- tier has timed out.  Index covers the WHERE clause.
CREATE INDEX idx_escalation_states_active ON escalation_states (last_notified_at)
    WHERE acknowledged_at IS NULL AND status IN ('pending', 'notified');

CREATE INDEX idx_escalation_states_policy_id ON escalation_states (policy_id);

COMMENT ON TABLE escalation_states IS 'Live escalation tracking: one row per alert under active escalation (v0.5)';
COMMENT ON COLUMN escalation_states.current_tier_index IS 'Index of the last-notified tier; worker advances this when timeout expires';
COMMENT ON COLUMN escalation_states.status IS 'pending: triggered, not yet notified; notified: DM sent; acknowledged: user acked; completed: alert resolved';
COMMENT ON COLUMN escalation_states.acknowledged_via IS 'Tracks channel used for acknowledgment: slack button, REST API, or CLI';
