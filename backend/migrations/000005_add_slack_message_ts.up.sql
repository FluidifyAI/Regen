-- Add slack_message_ts column to store the initial pinned message timestamp
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS slack_message_ts VARCHAR(64);

COMMENT ON COLUMN incidents.slack_message_ts IS 'Slack message timestamp of the pinned incident card; used to update buttons on status changes';
