-- Add agent_type to distinguish AI agent users from human users.
-- NULL for all human users; non-null only for auth_source='ai' rows.
ALTER TABLE users ADD COLUMN IF NOT EXISTS agent_type VARCHAR(50);

-- Add active flag to users so agents can be toggled on/off from the UI.
-- Default true so all existing human users remain active.
ALTER TABLE users ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;

-- Extend auth_source CHECK to allow 'ai' agent accounts.
-- Migration 000024 added an inline (unnamed) CHECK on auth_source.
-- We must find and drop it by scanning pg_constraint regardless of its auto-generated name,
-- then add a new named constraint that includes 'ai'.
DO $$
DECLARE r RECORD;
BEGIN
    FOR r IN
        SELECT c.conname
        FROM pg_constraint c
        JOIN pg_class t ON c.conrelid = t.oid
        WHERE t.relname = 'users' AND c.contype = 'c'
          AND pg_get_constraintdef(c.oid) LIKE '%auth_source%'
    LOOP
        EXECUTE format('ALTER TABLE users DROP CONSTRAINT IF EXISTS %I', r.conname);
    END LOOP;
END;
$$;
ALTER TABLE users ADD CONSTRAINT users_auth_source_check
    CHECK (auth_source IN ('saml', 'local', 'ai'));

-- Add ai_enabled flag to incidents.
-- Default true so existing incidents and manually-created incidents get AI by default.
-- Set to false via: routing rules, per-integration default, or incident Properties panel.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS ai_enabled BOOLEAN NOT NULL DEFAULT true;

-- Extend actor_type CHECK on timeline_entries to allow 'ai_agent'.
-- Migration 000004 created this constraint with ('user', 'system', 'slack_bot').
-- 000006 extended it to also include 'slack_user'.
ALTER TABLE timeline_entries DROP CONSTRAINT IF EXISTS timeline_entries_actor_type_check;
ALTER TABLE timeline_entries ADD CONSTRAINT timeline_entries_actor_type_check
    CHECK (actor_type IN ('user', 'system', 'slack_bot', 'slack_user', 'ai_agent'));
