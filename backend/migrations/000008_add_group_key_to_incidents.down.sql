-- Remove group_key column and index from incidents table
--
-- WARNING: This will delete all grouping information.
-- Incidents themselves are not deleted, but their grouping associations are lost.

DROP INDEX IF EXISTS idx_incidents_group_key_status_created;

ALTER TABLE incidents
DROP COLUMN IF EXISTS group_key;
