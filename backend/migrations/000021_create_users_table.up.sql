-- Create users table for SAML SSO provisioning.
-- Users are auto-created on first SAML login (JIT provisioning).
-- Passwords are never stored — authentication is entirely delegated to the IdP.

CREATE TABLE users (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL DEFAULT '',

    -- saml_subject is the NameID from the SAML assertion.
    -- This is the primary correlation key between IdP and local record.
    -- Immutable after first login.
    saml_subject    VARCHAR(500) NOT NULL UNIQUE,

    -- saml_idp_issuer is the EntityID of the IdP that provisioned this user.
    -- Useful for audit trails and multi-IdP scenarios.
    saml_idp_issuer VARCHAR(500) NOT NULL DEFAULT '',

    role            VARCHAR(50)  NOT NULL DEFAULT 'member'
                      CHECK (role IN ('admin', 'member', 'viewer')),

    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email        ON users(email);
CREATE INDEX idx_users_saml_subject ON users(saml_subject);

COMMENT ON TABLE  users                  IS 'Users provisioned via SAML SSO — no local passwords stored';
COMMENT ON COLUMN users.saml_subject     IS 'IMMUTABLE: NameID from the SAML assertion; primary IdP correlation key';
COMMENT ON COLUMN users.saml_idp_issuer  IS 'EntityID of the IdP that last authenticated this user';
