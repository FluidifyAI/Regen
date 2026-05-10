CREATE TABLE schedule_unavailabilities (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID        NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    user_name   VARCHAR(255) NOT NULL,
    start_date  DATE        NOT NULL,
    end_date    DATE        NOT NULL,
    reason      VARCHAR(500),
    created_by  VARCHAR(255) NOT NULL DEFAULT 'system',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_unavailability_dates CHECK (end_date >= start_date)
);

CREATE INDEX idx_unavailabilities_schedule_window
    ON schedule_unavailabilities (schedule_id, user_name, start_date, end_date);
