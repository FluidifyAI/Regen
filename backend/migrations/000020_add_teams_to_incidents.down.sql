ALTER TABLE incidents
    DROP COLUMN IF EXISTS teams_channel_id,
    DROP COLUMN IF EXISTS teams_channel_name,
    DROP COLUMN IF EXISTS teams_activity_id;
