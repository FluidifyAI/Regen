CREATE TABLE incident_attachment_data (
    attachment_id UUID  PRIMARY KEY REFERENCES incident_attachments(id) ON DELETE CASCADE,
    data          BYTEA NOT NULL
);
