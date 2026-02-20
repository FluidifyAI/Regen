-- Add AI summary fields to incidents table
-- ai_summary: the AI-generated summary text (separate from manual 'summary' field)
-- ai_summary_generated_at: when the summary was last generated (for display purposes)
ALTER TABLE incidents
    ADD COLUMN ai_summary              TEXT,
    ADD COLUMN ai_summary_generated_at TIMESTAMPTZ;
