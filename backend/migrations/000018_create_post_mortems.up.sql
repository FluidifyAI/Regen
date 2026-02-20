CREATE TABLE post_mortems (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id   UUID         NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    template_id   UUID         REFERENCES post_mortem_templates(id) ON DELETE SET NULL,
    template_name VARCHAR(100) NOT NULL DEFAULT 'Standard',
    status        VARCHAR(20)  NOT NULL DEFAULT 'draft'
                               CHECK (status IN ('draft', 'published')),
    content       TEXT         NOT NULL DEFAULT '',
    generated_by  VARCHAR(20)  NOT NULL DEFAULT 'ai'
                               CHECK (generated_by IN ('ai', 'manual')),
    generated_at  TIMESTAMPTZ,
    published_at  TIMESTAMPTZ,
    created_by_id VARCHAR(255) NOT NULL DEFAULT 'system',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One post-mortem per incident
CREATE UNIQUE INDEX idx_post_mortems_incident_id ON post_mortems (incident_id);
CREATE INDEX idx_post_mortems_status ON post_mortems (status);

COMMENT ON TABLE post_mortems IS 'Post-mortem documents. One per incident, enforced by unique index. Content is stored as Markdown. (v0.7)';
COMMENT ON COLUMN post_mortems.template_name IS 'Snapshot of template name at generation time. Preserved even if template is later deleted.';
COMMENT ON COLUMN post_mortems.content IS 'Full post-mortem in Markdown format. AI-generated initially; user-editable thereafter.';
