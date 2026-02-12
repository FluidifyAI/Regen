-- Add group_key column to incidents table for alert grouping support
--
-- The group_key is derived from alert labels according to matching grouping rules.
-- Alerts with the same group_key (within a time window) are grouped into the same incident.
--
-- Design decisions:
--   - Nullable: Not all incidents are created from grouped alerts (manual incidents)
--   - Indexed: Fast lookups for finding existing incidents with same group_key
--   - VARCHAR(64): SHA256 hex hash (64 characters)

ALTER TABLE incidents
ADD COLUMN group_key VARCHAR(64);

-- Index for efficient group_key lookups during alert processing
-- Used by grouping engine to find existing open incidents with matching group_key
CREATE INDEX idx_incidents_group_key_status_created
ON incidents (group_key, status, created_at)
WHERE group_key IS NOT NULL;

-- Note: Existing incidents will have NULL group_key (pre-grouping behavior)
-- This is intentional - only new incidents created after v0.3 will have group_key
