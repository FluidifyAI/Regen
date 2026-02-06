-- Drop indexes
DROP INDEX IF EXISTS idx_incident_alerts_alert_id;
DROP INDEX IF EXISTS idx_incident_alerts_incident_id;

-- Drop table
DROP TABLE IF EXISTS incident_alerts;
