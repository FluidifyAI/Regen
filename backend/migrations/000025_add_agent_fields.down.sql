ALTER TABLE users DROP COLUMN IF EXISTS agent_type;
ALTER TABLE users DROP COLUMN IF EXISTS active;
ALTER TABLE incidents DROP COLUMN IF EXISTS ai_enabled;

-- Revert auth_source CHECK to original ('saml', 'local') only.
-- WARNING: This permanently deletes all AI agent user accounts.
-- Rolled back environments will need to re-seed agents after re-applying the migration.
DELETE FROM users WHERE auth_source = 'ai';
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_auth_source_check;
ALTER TABLE users ADD CONSTRAINT users_auth_source_check
    CHECK (auth_source IN ('saml', 'local'));

-- Revert actor_type CHECK to exclude 'ai_agent'.
ALTER TABLE timeline_entries DROP CONSTRAINT IF EXISTS timeline_entries_actor_type_check;
ALTER TABLE timeline_entries ADD CONSTRAINT timeline_entries_actor_type_check
    CHECK (actor_type IN ('user', 'system', 'slack_bot', 'slack_user'));
