-- Revert to the 000025 constraint (excludes 'deactivated').
-- WARNING: any deactivated users will violate this constraint.
-- Re-activate or delete them before rolling back.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_auth_source_check;
ALTER TABLE users ADD CONSTRAINT users_auth_source_check
    CHECK (auth_source IN ('saml', 'local', 'ai'));
