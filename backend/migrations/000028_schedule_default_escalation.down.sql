DROP INDEX IF EXISTS idx_schedules_default_escalation_policy;
ALTER TABLE schedules DROP COLUMN IF EXISTS default_escalation_policy_id;
