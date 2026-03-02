-- Fix auth_source CHECK to include 'deactivated'.
-- Migration 000025 narrowed the constraint to ('saml','local','ai') but the
-- Deactivate() repository method sets auth_source='deactivated', which caused
-- a constraint violation for any user being deactivated.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_auth_source_check;
ALTER TABLE users ADD CONSTRAINT users_auth_source_check
    CHECK (auth_source IN ('saml', 'local', 'ai', 'deactivated'));
