-- Drop triggers
DROP TRIGGER IF EXISTS trigger_prevent_timeline_delete ON timeline_entries;
DROP TRIGGER IF EXISTS trigger_prevent_timeline_update ON timeline_entries;

-- Drop functions
DROP FUNCTION IF EXISTS prevent_timeline_timestamp_update();
DROP FUNCTION IF EXISTS prevent_timeline_delete();
DROP FUNCTION IF EXISTS prevent_timeline_update();

-- Drop indexes
DROP INDEX IF EXISTS idx_timeline_type;
DROP INDEX IF EXISTS idx_timeline_incident_timestamp;
DROP INDEX IF EXISTS idx_timeline_incident_id;

-- Drop table
DROP TABLE IF EXISTS timeline_entries;
