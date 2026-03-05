DROP INDEX IF EXISTS idx_users_slack_user_id;
ALTER TABLE users DROP COLUMN IF EXISTS slack_user_id;
