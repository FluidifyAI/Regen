-- schedule_holiday_configs: which countries are configured per schedule
CREATE TABLE IF NOT EXISTS schedule_holiday_configs (
    schedule_id  UUID         NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    country_code VARCHAR(10)  NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (schedule_id, country_code)
);

-- schedule_holidays: actual holiday dates fetched from ICS feeds
CREATE TABLE IF NOT EXISTS schedule_holidays (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id  UUID         NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    country_code VARCHAR(10)  NOT NULL,
    date         DATE         NOT NULL,
    name         VARCHAR(255) NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_schedule_holiday UNIQUE (schedule_id, country_code, date)
);

CREATE INDEX IF NOT EXISTS idx_schedule_holidays_schedule_date    ON schedule_holidays(schedule_id, date);
CREATE INDEX IF NOT EXISTS idx_schedule_holidays_schedule_country ON schedule_holidays(schedule_id, country_code);
