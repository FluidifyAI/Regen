-- Create escalation_policies and escalation_tiers tables.
--
-- Design decisions:
--   - An escalation policy is a named, ordered chain of tiers.
--   - Each tier targets either a schedule (on-call resolver), a list of users
--     (free-text user_names stored as JSONB), or both.
--   - timeout_seconds defines how long to wait at this tier before advancing.
--   - tier_index is 0-based and must be unique within a policy.
--   - Deleting a policy cascades to all tiers.
--   - Schedules referenced by a tier use ON DELETE SET NULL so that removing
--     a schedule doesn't silently destroy the escalation tier.

CREATE TABLE escalation_policies (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    enabled     BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_escalation_policies_enabled ON escalation_policies (enabled) WHERE enabled = true;

COMMENT ON TABLE escalation_policies IS 'Named escalation chains; each has an ordered list of notification tiers (v0.5)';
COMMENT ON COLUMN escalation_policies.enabled IS 'Only enabled policies are evaluated by the escalation engine';

-- Target types for an escalation tier:
--   schedule  → notify the currently on-call user from schedule_id
--   users     → notify every user_name in the user_names JSONB array
--   both      → notify schedule on-call AND all listed users
CREATE TABLE escalation_tiers (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id        UUID        NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,

    tier_index       INTEGER     NOT NULL CHECK (tier_index >= 0),
    timeout_seconds  INTEGER     NOT NULL DEFAULT 300 CHECK (timeout_seconds > 0),
    target_type      VARCHAR(50) NOT NULL DEFAULT 'schedule'
                       CHECK (target_type IN ('schedule', 'users', 'both')),

    -- Nullable FK: if the referenced schedule is deleted the tier survives
    -- with schedule_id = NULL and the engine skips the schedule target.
    schedule_id      UUID        REFERENCES schedules(id) ON DELETE SET NULL,

    -- Free-text user names; mirrors the user_name convention used in
    -- schedule_participants (no FK to a users table).
    -- Example: ["alice", "bob"]
    user_names       JSONB       NOT NULL DEFAULT '[]',

    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Enforce unique tier ordering within a policy.
    CONSTRAINT uq_escalation_tiers_policy_tier UNIQUE (policy_id, tier_index)
);

CREATE INDEX idx_escalation_tiers_policy_id ON escalation_tiers (policy_id, tier_index);
CREATE INDEX idx_escalation_tiers_schedule_id ON escalation_tiers (schedule_id) WHERE schedule_id IS NOT NULL;

COMMENT ON TABLE escalation_tiers IS 'Ordered notification tiers within an escalation policy (v0.5)';
COMMENT ON COLUMN escalation_tiers.tier_index IS '0-based; unique within policy. Tier 0 is notified first.';
COMMENT ON COLUMN escalation_tiers.timeout_seconds IS 'Seconds to wait at this tier before advancing to tier_index+1';
COMMENT ON COLUMN escalation_tiers.target_type IS 'schedule: on-call from schedule_id; users: user_names list; both: union';
COMMENT ON COLUMN escalation_tiers.user_names IS 'JSON array of free-text user identifiers, e.g. ["alice", "@bob"]';
