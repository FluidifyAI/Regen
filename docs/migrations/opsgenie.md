# Migrating from Opsgenie to Regen

Opsgenie charges per user and is now bundled into Atlassian's pricing. Fluidify Regen gives you the same on-call and escalation coverage, self-hosted, for free. The 1-click import wizard moves your schedules and escalation policies in under 60 seconds.

---

## What transfers

| Opsgenie concept | Regen equivalent | Status |
|---|---|---|
| Schedules (rotations, participants) | Schedules + rotation layers | ✅ Full |
| Escalation policies (rules, timeouts) | Escalation policies | ✅ Full |
| Users (matched by email address) | Users | ✅ Matched by email — see note below |
| Teams | — | ⚠️ No equivalent — assign users to schedules directly |
| Integrations / API keys | Webhook endpoints | ✅ New URLs provided after import |
| Overrides | — | ⚠️ Planned for a future release |
| Notification rules | — | ⚠️ Not imported |
| Alert / incident history | — | ❌ Not imported |

**User matching:** Regen matches Opsgenie users to Regen accounts by email address. Users who exist in Regen before the import are linked automatically. Opsgenie users with no matching Regen account are listed in the import report as warnings — the schedule layers are still imported with the user's email as a placeholder.

---

## 1-click import (UI)

1. Go to **Settings → Migrations → Opsgenie**
2. Paste your Opsgenie API key — [generate one](https://support.atlassian.com/opsgenie/docs/api-key-management/) under **Settings → API key management** in Opsgenie (read-only access is sufficient)
3. Select your region: **US** (default) or **EU** if your Opsgenie account is on `app.eu.opsgenie.com`
4. Click **Preview** — review exactly what will be imported before committing
5. Click **Import everything**

The import is idempotent. Running it a second time skips records that already exist (matched by name). Pass `force: true` via the API if you want to overwrite existing records with the same name.

---

## API import

For scripted or CI-driven migrations:

**Step 1 — Preview (dry run)**

```bash
curl -X POST https://your-regen-host/api/v1/migrations/opsgenie/preview \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <regen-api-token>" \
  -d '{
    "api_key": "<opsgenie-api-key>",
    "region": "us"
  }'
```

Response:

```json
{
  "schedules": [
    { "name": "Primary On-Call", "timezone": "America/New_York", "rotation_count": 2, "user_count": 4 }
  ],
  "policies": [
    { "name": "Default Escalation", "rule_count": 3 }
  ],
  "warnings": []
}
```

**Step 2 — Run the import**

```bash
curl -X POST https://your-regen-host/api/v1/migrations/opsgenie/import \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <regen-api-token>" \
  -d '{
    "api_key": "<opsgenie-api-key>",
    "region": "us",
    "force": false
  }'
```

Set `"region": "eu"` if your Opsgenie account is on the EU data residency cluster (`app.eu.opsgenie.com`).

---

## EU accounts

If your Opsgenie account lives on `app.eu.opsgenie.com`, you must set **region = EU** in the import wizard (or `"region": "eu"` in the API request). Regen routes import calls to `api.eu.opsgenie.com` instead of `api.opsgenie.com`. Using the wrong region returns a 401 from the Opsgenie API.

---

## After import

1. **Verify schedules** — go to **On-Call → Schedules** and confirm rotations and participants look correct
2. **Verify escalation policies** — go to **On-Call → Escalation Policies** and check the tier timeouts
3. **Update alert sources** — point your monitoring tools (Prometheus Alertmanager, Grafana, CloudWatch) at Regen's webhook URLs instead of Opsgenie's inbound integrations
4. **Invite unmatched users** — check the import report for any Opsgenie users that weren't matched to a Regen account; invite them and re-run the import or assign them manually

---

## Troubleshooting

**401 Unauthorized**
- Verify the API key is active in Opsgenie under **Settings → API key management**
- EU accounts: confirm you set `region: eu`

**Users not matching**
- User matching is by email address. Ensure your team members have the same email in Regen as in Opsgenie before running the import.
- Mismatched users appear in the import report `warnings` array — the schedule is still imported; assign users manually afterwards.

**Schedule already exists**
- The import skips records with a matching name by default. Pass `"force": true` to overwrite existing schedules and policies with the same name.

**API key permissions**
- A read-only API key is sufficient for the import (only GET calls are made against Opsgenie). A full-access key works too.

**Rate limiting**
- Opsgenie's API rate limits apply during the import. For accounts with hundreds of schedules, the import may take 30–60 seconds. If you hit a rate limit error, wait a minute and retry — the import is idempotent.
