# Local Auth + User Admin Design

> **Status:** Approved — ready for implementation planning
> **Date:** 2026-02-24
> **Scope:** Non-SSO authentication (email + password) and a `/settings/users` admin panel

---

## Problem

When SAML SSO is not configured, the app runs in "open mode" — `/api/v1/auth/me` returns
`{ authenticated: false, mode: "open" }` with no `name` or `email`. The sidebar shows `"?"` avatar
and `"You"` as the display name. There is no way for users to have an identity, log in, or be
managed in a team context without a corporate identity provider.

**Target users:** Small orgs (10–50 people) that need proper user accounts and team management
but don't have Okta/Azure AD/Google Workspace.

---

## Approach

Extend the existing `users` table with a `password_hash` column and add a `local_sessions` table.
`RequireAuth` middleware checks a local session cookie first, then falls through to SAML — so the
SAML path is entirely unchanged. One user table serves both auth paths, which keeps on-call
assignments, incident commanders, and timeline actors unified.

---

## Backend Design

### Migration 000024

```sql
-- Add local auth columns to users table
ALTER TABLE users
  ADD COLUMN password_hash TEXT,                    -- bcrypt hash; NULL for SAML-only users
  ADD COLUMN auth_source   TEXT DEFAULT 'saml';     -- 'saml' | 'local'

-- Local session store
CREATE TABLE local_sessions (
  token       TEXT        PRIMARY KEY,              -- crypto/rand 32-byte hex token
  user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at  TIMESTAMPTZ NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_local_sessions_user_id    ON local_sessions(user_id);
CREATE INDEX idx_local_sessions_expires_at ON local_sessions(expires_at);  -- for cleanup
```

### New API endpoints

```
POST   /api/v1/auth/login                    { email, password } → set oi_session cookie
POST   /api/v1/auth/logout                   clear oi_session cookie (works for local + SAML)

GET    /api/v1/settings/users                list all users (admin only)
POST   /api/v1/settings/users                create user; returns one-time password setup link
PATCH  /api/v1/settings/users/:id            update name / role / password
DELETE /api/v1/settings/users/:id            deactivate (soft delete) user
```

### `RequireAuth` middleware extension

```
request arrives
  → check oi_session cookie
      → look up local_sessions by token
          → found + not expired → attach user to context → continue
          → not found / expired → fall through to SAML check (existing behaviour)
  → no cookie → SAML check (existing behaviour)
```

SAML sessions are completely unaffected. In open mode (no SAML + no local session), the middleware
continues to pass all requests through.

### `/auth/me` response changes

Add `ssoEnabled` and `role` fields:

```json
// Open mode, no session
{ "authenticated": false, "mode": "open", "ssoEnabled": false }

// Local session
{ "authenticated": true, "email": "jane@acme.com", "name": "Jane Smith",
  "role": "admin", "ssoEnabled": false }

// SAML session
{ "authenticated": true, "email": "jane@acme.com", "name": "Jane Smith",
  "role": "user", "ssoEnabled": true }
```

`ssoEnabled` tells the frontend whether to show the "Sign in with SSO" button on the login page.

### First-run bootstrap

When the application starts with no users in the database and SAML is not configured, the
`POST /api/v1/auth/login` endpoint accepts a special bootstrap flow: if zero users exist,
the first `POST /api/v1/settings/users` call (unauthenticated) creates the initial admin account.
This avoids a chicken-and-egg problem (can't log in to create users; can't create users without
logging in).

### Password storage

- bcrypt with cost factor 12
- Minimum 8 characters enforced at API layer
- Passwords never returned in any API response

### Session management

- Cookie name: `oi_session`
- Expiry: 7 days (sliding window — refreshed on each authenticated request)
- `HttpOnly`, `SameSite=Lax`, `Secure` in production
- Cleanup: expired sessions purged on each login (lazy cleanup)

---

## Frontend Design

### Sidebar fix

`userDisplayName()` will work correctly once `/auth/me` returns a real name for local users.
For unauthenticated open mode (no session), display `"Open Mode"` with a login link rather than
`"You"` + `"?"`. If `user.mode === 'open' && !user.authenticated`, show a "Sign in" button.

### Settings nav item

Add a **Settings** entry to the sidebar nav:

```
Your organization
├── Incidents
├── Routing Rules
├── On-call
├── Escalation
└── PM Templates

⚙️ Settings          → /settings/users
```

### Login page update

The existing `LoginPage` shows only an SSO button. Extend it with a local auth form:

```
┌─────────────────────────────────────┐
│        Sign in to OpenIncident      │
│                                     │
│  Email                              │
│  ┌─────────────────────────────┐    │
│  │ you@company.com             │    │
│  └─────────────────────────────┘    │
│                                     │
│  Password                           │
│  ┌─────────────────────────────┐    │
│  │ ••••••••••••                │    │
│  └─────────────────────────────┘    │
│                                     │
│  [        Sign in         ]         │
│                                     │
│  ──────── or ────────               │  (only if ssoEnabled=true)
│  [   Sign in with SSO    ]          │  (only if ssoEnabled=true)
└─────────────────────────────────────┘
```

- SSO button only renders when `ssoEnabled: true` in the `/auth/me` response
- On successful login: redirect to `/`
- On failure: inline error message ("Invalid email or password")

### `/settings/users` page

Admin-only. Redirects to `/` for non-admin users.

```
Users                                         [+ Invite user]
──────────────────────────────────────────────────────────────
Name            Email                Role    Auth   Actions
─────────────────────────────────────────────────────────────
John Doe        john@acme.com        Admin   Local  Edit  ···
Jane Smith      jane@acme.com        User    Local  Edit  ···
alice@co.com    alice@co.com         User    SSO    —     ···
```

**Invite user modal** (`+ Invite user`):
- Fields: Name, Email, Role (Admin / User)
- On submit: creates user, returns a one-time password setup URL
- Operator copies link and shares it with the new user (no SMTP required)
- One-time link expires after 24 hours

**Edit modal:**
- Change name, role
- "Reset password" option — generates a new one-time setup link

**SSO users:**
- Listed as read-only (no password management, auth column shows `SSO`)
- Can be deactivated (sets `deleted_at`)

---

## Error handling

- Login with wrong credentials: `401` with generic message (`"Invalid email or password"`)
- Non-admin accessing `/settings/users`: `403`
- Invite with duplicate email: `409 Conflict`
- Expired/invalid session token: `401` (middleware clears the cookie)

---

## What this does NOT change

- SAML SSO path is completely unchanged
- Existing `users` table rows are untouched (new columns are nullable/defaulted)
- `RequireAuth` middleware open-mode pass-through is unchanged
- No breaking changes to existing API responses (new fields are additive)

---

## Pending (out of scope for this feature)

- MFA / TOTP
- Password reset via email (requires SMTP; use admin reset link for now)
- OAuth (Google, GitHub) — post-v1.0
- General UI polish pass (deferred to separate work item)
