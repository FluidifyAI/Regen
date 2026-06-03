CREATE TABLE incident_attachments (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID        NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    file_name   TEXT        NOT NULL,
    file_size   BIGINT      NOT NULL,
    mime_type   TEXT        NOT NULL,
    uploaded_by TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incident_attachments_incident_id ON incident_attachments(incident_id);
