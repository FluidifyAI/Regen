# OpenIncident — Manual Testing Walkthrough

> **Purpose:** Step-by-step verification of every major feature.
> **Companion doc:** `UI-IMPROVEMENTS.md` (separate backlog for polish items found during testing).
> **Start fresh:** Run `make down && make dev` before beginning. Use a clean browser profile or incognito.

---

## Setup

```bash
# Start all services
make dev

# Verify everything is up
curl http://localhost:8080/health   # → {"status":"ok"}
curl http://localhost:8080/ready    # → {"status":"ready","database":"ok","redis":"ok"}
```

Frontend: http://localhost:3000
API: http://localhost:8080

---

## 1. Authentication

### 1.1 First-run setup (no users exist)

1. Open http://localhost:3000 in a fresh browser (incognito)
2. **Expected:** Login page shown — NOT the app. Setup form is auto-expanded with "No accounts exist yet" message
3. Fill in Full name, Email, Password (min 8 chars) and click **Create admin account & sign in**
4. **Expected:** Redirect to the app dashboard, logged in as the new admin
5. Check the Sidebar bottom — should show your name and email

### 1.2 Login

1. Log out (see 1.4), then return to http://localhost:3000
2. **Expected:** Login page (sign-in form, not setup form — account now exists)
3. Enter wrong password → **Expected:** "Invalid email or password" error
4. Enter correct credentials → **Expected:** Redirect to dashboard

### 1.3 Session persistence

1. Log in successfully
2. Hard refresh the page (Cmd+Shift+R)
3. **Expected:** Still logged in, app renders normally — not redirected to login

### 1.4 Logout

1. Click the logout icon (arrow-out icon) in the bottom of the Sidebar
2. **Expected:** "You've been signed out" confirmation page at /logout
3. Click **Sign in again** → **Expected:** Login page shown
4. Open http://localhost:3000 in the same tab → **Expected:** Login page (not the app)

### 1.5 Protected routes

1. While logged out, manually navigate to http://localhost:3000/incidents
2. **Expected:** Login page shown — not the incidents list

### 1.6 Rate limiting on login

1. Submit the login form with wrong credentials 11 times rapidly
2. **Expected:** HTTP 429 response after the 10th attempt within a minute

---

## 2. Alert Ingestion

> Requires backend running. Use `curl` or a tool like Insomnia/Postman.

### 2.1 Prometheus / Alertmanager webhook

```bash
curl -s -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "instance": "web-01"
      },
      "annotations": {
        "summary": "CPU usage above 90%"
      },
      "startsAt": "2026-02-27T10:00:00Z"
    }]
  }'
```

**Expected:** `{"received":1,"created":1,...}` — alert stored, incident auto-created

### 2.2 Grafana webhook

```bash
curl -s -X POST http://localhost:8080/api/v1/webhooks/grafana \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": { "alertname": "HighMemory", "severity": "warning" },
      "annotations": { "summary": "Memory usage above 80%" },
      "startsAt": "2026-02-27T10:00:00Z",
      "generatorURL": "http://grafana.example.com/alert/1"
    }]
  }'
```

**Expected:** Alert created, incident auto-created for warning severity

### 2.3 Generic webhook

```bash
curl -s -X POST http://localhost:8080/api/v1/webhooks/generic \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "title": "Database connection pool exhausted",
      "severity": "critical",
      "description": "Connection pool at 100% capacity"
    }]
  }'
```

**Expected:** Alert and incident created

### 2.4 Alert deduplication

Send the Prometheus payload from 2.1 again (same `alertname` + `instance`).
**Expected:** No duplicate alert created — existing one updated / grouped

### 2.5 Alert list UI

1. Navigate to the app UI
2. Open an incident → **Expected:** Linked alert(s) visible in the incident detail

---

## 3. Incident Management

### 3.1 Incident list

1. Navigate to http://localhost:3000/incidents
2. **Expected:** Incidents from section 2 appear, with severity badge, status, and timestamp

### 3.2 Manual incident creation

1. On the incidents list, click **New Incident** (or equivalent button)
2. Fill in title, severity (Critical), summary
3. **Expected:** New incident appears in list with status "triggered"

### 3.3 Status workflow

On an incident detail page:

1. Click **Acknowledge** → **Expected:** Status changes to "acknowledged", timeline entry added
2. Click **Resolve** → **Expected:** Status changes to "resolved", timeline entry added
3. **Expected:** Cannot go backwards (no "re-open" button)

### 3.4 Timeline entries

1. On an incident detail page, add a manual note to the timeline
2. **Expected:** Note appears immediately with timestamp and "user" actor
3. Refresh the page → **Expected:** Note still there (persisted)

### 3.5 Incident filtering

1. On the incidents list, filter by status "triggered"
2. **Expected:** Only open incidents shown
3. Filter by severity "critical" → **Expected:** Only critical incidents

---

## 4. Routing Rules

### 4.1 Create a routing rule

1. Navigate to http://localhost:3000/routing-rules
2. Create a rule: `severity = critical` → route to a specific escalation policy (create one first in section 6)
3. **Expected:** Rule appears in list

### 4.2 Verify routing fires

Send a critical webhook (section 2.1). Check that the incident was routed correctly per the rule.

---

## 5. Alert Grouping Rules

### 5.1 Create a grouping rule

1. Navigate to the grouping rules section (Settings or main nav)
2. Create a rule to group by `alertname`
3. **Expected:** Rule appears

### 5.2 Verify deduplication

Send the same alert twice (section 2.4). **Expected:** Grouped into one incident, not two.

---

## 6. On-Call Schedules

### 6.1 Create a schedule

1. Navigate to http://localhost:3000/on-call
2. Click **New Schedule**
3. Name it "Primary On-Call", timezone UTC
4. **Expected:** Schedule created and visible in list

### 6.2 Add a rotation layer

1. Open the schedule
2. Add a layer: weekly rotation, starting now, add yourself as participant
3. **Expected:** Layer visible in schedule view

### 6.3 Who's on call

```bash
curl http://localhost:8080/api/v1/schedules/{id}/oncall \
  -H "Cookie: oi_session=YOUR_TOKEN"
```

**Expected:** Returns the current on-call user

### 6.4 Create an override

1. On the schedule detail page, create an override for tomorrow
2. **Expected:** Override appears on the calendar

### 6.5 On-call timeline

1. View the on-call timeline for the schedule
2. **Expected:** Shows rotation shifts over a time window

---

## 7. Escalation Policies

### 7.1 Create a policy

1. Navigate to http://localhost:3000/escalation-policies
2. Click **New Policy**
3. Add Tier 1: notify on-call from the schedule created in section 6, timeout 5 min
4. Add Tier 2: notify a specific user, timeout 10 min
5. **Expected:** Policy with two tiers visible

### 7.2 Verify escalation triggers

1. Create an incident manually (section 3.2) and link the escalation policy
2. Wait or reduce timeout in config
3. **Expected:** Escalation advances through tiers (check logs: `docker logs openincident-api`)

---

## 8. AI Features

> Requires `OPENAI_API_KEY` set in `.env` / `docker-compose.yml`.

### 8.1 Check AI settings

```bash
curl http://localhost:8080/api/v1/settings/ai \
  -H "Cookie: oi_session=YOUR_TOKEN"
```

**Expected:** `{"enabled": true, "model": "gpt-4o-mini"}` (or similar)

### 8.2 Incident summarization

1. Open an incident with at least a few timeline entries
2. Click **Generate Summary**
3. **Expected:** AI-written summary appears in the incident detail

### 8.3 Handoff digest

1. On an incident detail, generate a handoff digest
2. **Expected:** Structured handoff document returned

### 8.4 Post-mortem generation (covered in section 9.2)

---

## 9. Post-Mortems

### 9.1 Post-mortem templates

1. Navigate to http://localhost:3000/post-mortem-templates
2. Create a template with custom sections
3. **Expected:** Template saved and editable
4. Edit the template → save → **Expected:** Changes persisted
5. Delete the template → **Expected:** Gone from list

### 9.2 Generate a post-mortem

1. Open a resolved incident
2. Click **Generate Post-Mortem** (AI)
3. **Expected:** Draft post-mortem appears with timeline, root cause section, action items

### 9.3 Edit a post-mortem

1. Edit the generated draft — modify the root cause text
2. Save → **Expected:** Changes persisted on refresh

### 9.4 Action items

1. Add an action item to the post-mortem (owner, due date, description)
2. **Expected:** Action item appears
3. Mark it complete → **Expected:** Status updates
4. Delete it → **Expected:** Gone

### 9.5 Export

```bash
curl http://localhost:8080/api/v1/incidents/{id}/postmortem/export \
  -H "Cookie: oi_session=YOUR_TOKEN"
```

**Expected:** JSON export of the post-mortem (Confluence/Notion export is pending — see UI-IMPROVEMENTS.md)

---

## 10. User Management (Settings)

> Requires admin account.

### 10.1 User list

1. Navigate to http://localhost:3000/settings/users
2. **Expected:** Your admin account listed with role "admin"

### 10.2 Create a new user

1. Click **Invite User** (or **Add User**)
2. Enter name, email, role "Member", password
3. **Expected:** User appears in list with "Member" role
4. **Expected:** A setup token is returned (would be emailed in production)

### 10.3 Edit a user

1. Click edit on the new user
2. Change their role to "Admin"
3. **Expected:** Role updated in list

### 10.4 Reset password

1. Click **Reset Password** on a user
2. **Expected:** A new setup token returned
3. Log in with the new token / generated password

### 10.5 Cannot deactivate yourself

1. Try to deactivate your own account
2. **Expected:** Error "cannot deactivate your own account"

### 10.6 Cannot remove the last admin

1. Deactivate the second admin, leaving only one
2. Try to deactivate the last admin account
3. **Expected:** Error "cannot deactivate the last admin account"

### 10.7 Deactivate a user

1. Deactivate the "Member" user created in 10.2
2. **Expected:** User disappears from active list (or shows as deactivated)
3. Try to log in as that user → **Expected:** "Invalid email or password" (session revoked immediately)

---

## 11. Slack Integration

> Requires `SLACK_BOT_TOKEN` and `SLACK_SIGNING_SECRET` configured.

### 11.1 Channel auto-creation

1. Send a critical alert (section 2.1)
2. **Expected:** Slack channel created automatically, named after the incident slug (e.g. `#inc-001-high-cpu`)

### 11.2 Incident card in Slack

**Expected:** Adaptive message card posted in the channel with incident details and Ack/Resolve buttons

### 11.3 Acknowledge from Slack

1. Type `/incident ack` in the incident channel
   OR click the Ack button on the card
2. **Expected:** Incident status changes to "acknowledged" in the UI; timeline entry added

### 11.4 Resolve from Slack

1. Type `/incident resolve` in the channel
2. **Expected:** Status changes to "resolved"; channel message updated

### 11.5 Bidirectional sync (Socket Mode)

> Requires `SLACK_APP_TOKEN` set.

1. Post a message in the incident Slack channel
2. **Expected:** Message appears as a timeline entry in the incident UI

### 11.6 Shift notifications

1. Configure a schedule with a shift starting soon
2. **Expected:** On-call engineer receives a Slack DM at shift start

---

## 12. Teams Integration

> Requires `TEAMS_APP_ID`, `TEAMS_CLIENT_SECRET`, `TEAMS_TENANT_ID`, `TEAMS_SERVICE_URL`, `TEAMS_TEAM_ID` configured.

### 12.1 Channel auto-creation

1. Send a critical alert
2. **Expected:** Teams channel created in the configured team, named after the incident

### 12.2 Adaptive Card

**Expected:** Adaptive Card posted in the Teams channel with incident details

### 12.3 Bot commands

In the incident Teams channel:

1. `@OpenIncident ack` → **Expected:** Incident acknowledged, card updated
2. `@OpenIncident resolve` → **Expected:** Incident resolved, card updated
3. `@OpenIncident status` → **Expected:** Current incident status returned

### 12.4 Bidirectional sync

1. Reply in the Teams channel (non-command message)
2. **Expected:** Message appears as a timeline entry in the UI

### 12.5 Known limitations (do not expect these to work)

- Channel archive on resolve → channel gets renamed `[RESOLVED] ...` (not archived — Graph API limitation)
- Auto-invite specific users → no-op for standard channels (private channel model required)

---

## 13. SAML SSO

> Requires `SAML_IDP_METADATA_URL` configured (Okta / Azure AD / Google Workspace).

### 13.1 SSO button visible

1. Navigate to http://localhost:3000/login
2. **Expected:** "Sign in with SSO" button appears below the local login form

### 13.2 SSO login flow

1. Click **Sign in with SSO**
2. **Expected:** Redirected to IdP login page
3. Complete IdP login
4. **Expected:** Redirected back to app, logged in as SSO user

### 13.3 JIT user provisioning

1. Log in via SSO with a user not previously seen
2. Check `settings/users` (admin account)
3. **Expected:** New user record created with `auth_source = "saml"`

### 13.4 SSO logout

1. Log out via the Sidebar button
2. **Expected:** Both local session and SAML session cleared

---

## 14. Health & Observability

### 14.1 Health endpoint

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

### 14.2 Readiness endpoint

```bash
curl http://localhost:8080/ready
# → {"status":"ready","database":"ok","redis":"ok"}
```

### 14.3 Prometheus metrics

```bash
curl http://localhost:8080/metrics
# → Prometheus text format with http_requests_total, response time histograms, etc.
```

---

## 15. Rate Limiting

### 15.1 Auth rate limit (10 req/min)

Already tested in 1.6.

### 15.2 Webhook rate limit (300 req/min)

Send 301 rapid webhook requests. **Expected:** 429 on the 301st within the minute window.

### 15.3 API rate limit (120 req/min)

Send 121 rapid API requests. **Expected:** 429 on the 121st.

---

## Testing Complete ✓

When all sections pass, the instance is production-ready.
Record any failures or UI rough edges in `docs/UI-IMPROVEMENTS.md`.

---

## Related Docs

- `docs/UI-IMPROVEMENTS.md` — UI polish backlog (tracked separately, linked here)
- `docs/SECURITY.md` — Security model and hardening notes
- `docs/OPERATIONS.md` — Production deployment, HA, observability
- `docs/AI-AGENTS-CONCEPT.md` — Future AI agent layer concept (not yet implemented)

---

## Pending Features (not testable yet)

| Feature | Status | Notes |
|---|---|---|
| PagerDuty import | Not implemented | CLI command planned (OI-EPIC-020) |
| Post-mortem export to Confluence/Notion | Not implemented | JSON export works |
| Teams channel archive | Documented limitation | Graph API cannot archive standard channels |
| Teams auto-invite users | Documented limitation | Requires private channel model |
| SCIM provisioning | Enterprise tier | Separate private repo |
| Audit log export | Enterprise tier | Separate private repo |
| RBAC (viewer/responder/admin) | Enterprise tier | Separate private repo |
