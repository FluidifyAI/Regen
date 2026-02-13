-- Create schedule_overrides table for temporary on-call substitutions.
--
-- Design decisions:
--   - start_time is inclusive, end_time is exclusive (standard interval convention).
--   - No UNIQUE constraint on overlapping windows — the evaluator returns the
--     first matching override (ordered by start_time DESC) to prefer newer ones.
--   - created_by is free-text user_name, not a foreign key.

CREATE TABLE schedule_overrides (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id   UUID         NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,

    override_user VARCHAR(255) NOT NULL,
    start_time    TIMESTAMPTZ  NOT NULL,
    end_time      TIMESTAMPTZ  NOT NULL,
    created_by    VARCHAR(255) NOT NULL DEFAULT 'system',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT schedule_overrides_valid_window CHECK (end_time > start_time)
);

-- Index for efficient active override lookup at a specific time
CREATE INDEX idx_schedule_overrides_schedule_time
    ON schedule_overrides (schedule_id, start_time, end_time);

COMMENT ON TABLE schedule_overrides IS 'Temporary on-call substitutions within a schedule (v0.4)';
COMMENT ON COLUMN schedule_overrides.override_user IS 'User taking over on-call during [start_time, end_time)';
COMMENT ON COLUMN schedule_overrides.start_time IS 'Override window start (inclusive)';
COMMENT ON COLUMN schedule_overrides.end_time IS 'Override window end (exclusive)';
