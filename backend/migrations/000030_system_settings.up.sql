-- Generic key-value store for system-wide settings.
CREATE TABLE IF NOT EXISTS system_settings (
    key        VARCHAR(255) PRIMARY KEY,
    value      JSONB        NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Seed the escalation fallback row with null value.
INSERT INTO system_settings (key, value)
VALUES ('escalation.global_fallback_policy_id', 'null')
ON CONFLICT DO NOTHING;
