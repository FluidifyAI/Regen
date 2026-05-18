CREATE TABLE custom_field_definitions (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name          TEXT        NOT NULL,
    key           TEXT        NOT NULL UNIQUE,
    field_type    TEXT        NOT NULL CHECK (field_type IN ('string', 'number', 'dropdown')),
    options       JSONB       NOT NULL DEFAULT '[]',
    display_order INT         NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incidents_custom_fields ON incidents USING GIN (custom_fields);
