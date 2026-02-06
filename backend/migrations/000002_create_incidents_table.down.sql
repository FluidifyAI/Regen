-- Drop trigger
DROP TRIGGER IF EXISTS trigger_prevent_incident_timestamp_update ON incidents;

-- Drop function
DROP FUNCTION IF EXISTS prevent_incident_timestamp_update();

-- Drop indexes
DROP INDEX IF EXISTS idx_incidents_incident_number;
DROP INDEX IF EXISTS idx_incidents_triggered_at;
DROP INDEX IF EXISTS idx_incidents_severity;
DROP INDEX IF EXISTS idx_incidents_status;

-- Drop table
DROP TABLE IF EXISTS incidents;
