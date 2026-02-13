-- Create schedule_layers and schedule_participants tables.
--
-- Design decisions:
--   - shift_duration_seconds is the single source of truth for shift length.
--     rotation_type is a UI hint; the evaluator always uses shift_duration_seconds.
--   - rotation_start defaults to midnight UTC on creation day — set by the application.
--   - order_index on both layers and participants is explicit; no UNIQUE constraint
--     to allow flexible reordering without constraint violations during updates.

CREATE TABLE schedule_layers (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id           UUID         NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,

    name                  VARCHAR(255) NOT NULL,
    order_index           INTEGER      NOT NULL DEFAULT 0,
    rotation_type         VARCHAR(50)  NOT NULL DEFAULT 'weekly'
                            CHECK (rotation_type IN ('daily', 'weekly', 'custom')),
    rotation_start        TIMESTAMPTZ  NOT NULL,
    shift_duration_seconds INTEGER     NOT NULL DEFAULT 604800
                            CHECK (shift_duration_seconds > 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_schedule_layers_schedule_id ON schedule_layers (schedule_id, order_index);

COMMENT ON TABLE schedule_layers IS 'Rotation layers within a schedule, evaluated in order_index order (v0.4)';
COMMENT ON COLUMN schedule_layers.rotation_type IS 'UI hint: daily (86400s), weekly (604800s), or custom';
COMMENT ON COLUMN schedule_layers.rotation_start IS 'Epoch for shift slot computation: slot boundaries are rotation_start + N * shift_duration_seconds';

CREATE TABLE schedule_participants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    layer_id    UUID         NOT NULL REFERENCES schedule_layers(id) ON DELETE CASCADE,

    user_name   VARCHAR(255) NOT NULL,
    order_index INTEGER      NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_schedule_participants_layer_id ON schedule_participants (layer_id, order_index);

COMMENT ON TABLE schedule_participants IS 'Users rotating within a schedule layer (v0.4)';
COMMENT ON COLUMN schedule_participants.user_name IS 'Free-text display name or identifier; not a foreign key';
