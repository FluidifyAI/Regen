DROP INDEX IF EXISTS idx_users_teams_user_id;
ALTER TABLE users DROP COLUMN IF EXISTS teams_user_id;
