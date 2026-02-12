-- Revert actor_type check constraint to original values
ALTER TABLE timeline_entries DROP CONSTRAINT timeline_entries_actor_type_check;
ALTER TABLE timeline_entries ADD CONSTRAINT timeline_entries_actor_type_check
    CHECK (actor_type IN ('user', 'system', 'slack_bot'));
