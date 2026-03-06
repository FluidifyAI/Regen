ALTER TABLE users ADD COLUMN IF NOT EXISTS teams_user_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_users_teams_user_id ON users(teams_user_id) WHERE teams_user_id IS NOT NULL;
