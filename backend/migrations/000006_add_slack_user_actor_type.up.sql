-- Add 'slack_user' to the actor_type check constraint on timeline_entries
-- Required for Slack channel messages synced to the incident timeline
ALTER TABLE timeline_entries DROP CONSTRAINT timeline_entries_actor_type_check;
ALTER TABLE timeline_entries ADD CONSTRAINT timeline_entries_actor_type_check
    CHECK (actor_type IN ('user', 'system', 'slack_bot', 'slack_user'));
