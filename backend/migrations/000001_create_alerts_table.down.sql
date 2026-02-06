-- Drop trigger
DROP TRIGGER IF EXISTS trigger_prevent_received_at_update ON alerts;

-- Drop trigger function
DROP FUNCTION IF EXISTS prevent_received_at_update();

-- Drop indexes
DROP INDEX IF EXISTS idx_alerts_external_id;
DROP INDEX IF EXISTS idx_alerts_received_at;
DROP INDEX IF EXISTS idx_alerts_severity;
DROP INDEX IF EXISTS idx_alerts_status;
DROP INDEX IF EXISTS idx_alerts_source;

-- Drop table
DROP TABLE IF EXISTS alerts;
