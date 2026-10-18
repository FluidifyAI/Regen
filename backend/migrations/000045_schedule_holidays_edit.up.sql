-- Admins can hide, rename and add holidays. The feed sync must not undo those edits:
--   source: 'feed' (imported) or 'manual' (added or renamed by an admin)
--   hidden: admin removed a feed holiday; the row is kept so the sync cannot re-add it
ALTER TABLE schedule_holidays
    ADD COLUMN IF NOT EXISTS source VARCHAR(10) NOT NULL DEFAULT 'feed',
    ADD COLUMN IF NOT EXISTS hidden BOOLEAN     NOT NULL DEFAULT FALSE;

ALTER TABLE schedule_holidays
    ADD CONSTRAINT chk_schedule_holidays_source CHECK (source IN ('feed', 'manual'));
