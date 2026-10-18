ALTER TABLE schedule_holidays DROP CONSTRAINT IF EXISTS chk_schedule_holidays_source;
ALTER TABLE schedule_holidays
    DROP COLUMN IF EXISTS hidden,
    DROP COLUMN IF EXISTS source;
