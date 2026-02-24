-- WARNING: rolling back this migration destroys all locally-created user accounts.
-- Any user with auth_source='local' has saml_subject=NULL and cannot satisfy the
-- restored NOT NULL constraint. They must be deleted before the constraint can be applied.
DELETE FROM users WHERE auth_source = 'local';

DROP TABLE IF EXISTS local_sessions;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users DROP COLUMN IF EXISTS auth_source;
ALTER TABLE users ALTER COLUMN saml_subject SET NOT NULL;
