ALTER TABLE incidents
    DROP COLUMN IF EXISTS ai_summary,
    DROP COLUMN IF EXISTS ai_summary_generated_at;
