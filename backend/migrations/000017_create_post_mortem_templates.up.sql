CREATE TABLE post_mortem_templates (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT         NOT NULL DEFAULT '',
    sections    JSONB        NOT NULL DEFAULT '[]',
    is_built_in BOOLEAN      NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE post_mortem_templates IS 'User-manageable post-mortem templates. Built-in templates are seeded at migration time (v0.7).';
COMMENT ON COLUMN post_mortem_templates.sections IS 'Ordered array of section names. E.g. ["Summary","Impact","Timeline","Root Cause","Action Items"]';
COMMENT ON COLUMN post_mortem_templates.is_built_in IS 'True for templates shipped with OpenIncident. Users may still edit or delete these.';

-- Seed built-in templates
INSERT INTO post_mortem_templates (name, description, sections, is_built_in) VALUES
(
    'Standard',
    'Balanced post-mortem for most incidents. Covers impact, root cause, and follow-up actions.',
    '["Summary","Impact","Timeline","Root Cause","Contributing Factors","Action Items"]',
    true
),
(
    'Infrastructure',
    'Focused on infrastructure changes and operational follow-up. Ideal for deployment or configuration-related outages.',
    '["Summary","Impact","Timeline","Root Cause","Infrastructure Changes","Remediation","Action Items","Runbook Updates"]',
    true
),
(
    'Security',
    'Blameless security incident template. Covers security impact, remediation steps, and compliance notes.',
    '["Executive Summary","Security Impact","Timeline","Root Cause Analysis","Remediation Steps","Action Items","Compliance Notes"]',
    true
);
