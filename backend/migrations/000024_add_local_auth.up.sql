-- Allow saml_subject to be NULL for locally-created users.
-- SAML-provisioned users keep their existing saml_subject value.
ALTER TABLE users
    ALTER COLUMN saml_subject DROP NOT NULL;

-- Local auth columns
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash TEXT,
    ADD COLUMN IF NOT EXISTS auth_source   VARCHAR(20) NOT NULL DEFAULT 'saml'
        CHECK (auth_source IN ('saml', 'local'));

-- Local session store
CREATE TABLE IF NOT EXISTS local_sessions (
    token       TEXT        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_local_sessions_user_id    ON local_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_local_sessions_expires_at ON local_sessions(expires_at);
