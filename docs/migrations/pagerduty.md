# Migrating from PagerDuty to Regen

PagerDuty's pricing scales fast — $21–50/user/month adds up quickly. Fluidify Regen gives you the same on-call and escalation coverage, self-hosted, for free. The 1-click import wizard moves your schedules and escalation policies in under 60 seconds.

---

## What transfers

| PagerDuty concept | Regen equivalent | Status |
|---|---|---|
| On-call schedules (layers, rotations) | Schedules + rotation layers | ✅ Full |
| Escalation policies (rules, timeouts) | Escalation policies | ✅ Full |
| Users (matched by email address) | Users | ✅ Matched by email — see note below |
| Services | — | ⚠️ No equivalent — use alert routing rules |
| Integrations (webhook URLs) | Webhook endpoints | ✅ New URLs provided after import |
| Overrides | — | ⚠️ Planned for a future release |
| Notification rules | — | ⚠️ Not imported |
| Alert/incident history | — | ❌ Not imported |

**User matching:** Regen matches PagerDuty users to Regen accounts by email address. Users who exist in Regen before the import are linked automatically. PagerDuty users with no matching Regen account are listed in the import report as warnings — the schedule layers are still imported with the user's email as a placeholder.

---

## 1-click import (UI)

1. Go to **Settings → Migrations → PagerDuty**
2. Paste your PagerDuty API key (read-only key is sufficient — [generate one here](https://support.pagerduty.com/docs/api-access-keys))
3. Select your region: **US** (default) or **EU** if your PagerDuty account is on `app.eu.pagerduty.com`
4. Click **Preview** — review exactly what will be imported before committing
5. Click **Import everything**

The import is idempotent. Running it a second time skips records that already exist (matched by name). Pass `force: true` via the API if you want to overwrite existing records with the same name.

---

## API import

For scripted or CI-driven migrations:

**Step 1 — Preview (dry run)**

```bash
curl -X POST https://your-regen-host/api/v1/migrations/pagerduty/preview \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <regen-api-token>" \
  -d '{
    "api_key": "<pagerduty-api-key>",
    "region": "us"
  }'
```

Response:
```json
{
  "schedules": [
    { "name": "Primary On-Call", "timezone": "America/New_York", "layer_count": 2, "user_count": 4 }
  ],
  "policies": [
    { "name": "Default Escalation", "tier_count": 3 }
  ],
  "warnings": []
}
```

**Step 2 — Run the import**

```bash
curl -X POST https://your-regen-host/api/v1/migrations/pagerduty/import \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <regen-api-token>" \
  -d '{
    "api_key": "<pagerduty-api-key>",
    "region": "us",
    "force": false
  }'
```

Set `"region": "eu"` if your PagerDuty account is on the EU data residency cluster.

---

## EU accounts

If your PagerDuty account lives on `app.eu.pagerduty.com`, you must set **region = EU** in the import wizard (or `"region": "eu"` in the API request). Regen routes the import calls to `api.eu.pagerduty.com` instead of `api.pagerduty.com`. Using the wrong region returns a 401 from the PagerDuty API.

---

## After import

1. **Verify schedules** — go to **On-Call → Schedules** and confirm layers and participants look correct
2. **Verify escalation policies** — go to **On-Call → Escalation Policies** and check the tier timeouts
3. **Update alert sources** — point your monitoring tools (Prometheus Alertmanager, Grafana, CloudWatch) at Regen's webhook URLs instead of PagerDuty's inbound integrations
4. **Invite unmatched users** — check the import report for any PagerDuty users that weren't matched to a Regen account; invite them and re-run the import or assign them manually

---

## Troubleshooting

**401 Unauthorized**
- Check that the API key is valid (test it at `GET https://api.pagerduty.com/users/me` with `Authorization: Token token=<key>`)
- EU accounts: confirm you set `region: eu`

**Users not matching**
- User matching is by email address. Ensure your team members have the same email in Regen as in PagerDuty before running the import.
- Mismatched users appear in the import report `warnings` array — the schedule is still imported; assign users manually afterwards.

**Schedule already exists**
- The import skips records with a matching name by default. Pass `"force": true` to overwrite existing schedules and policies with the same name.

**API key permissions**
- A read-only API key is enough for the import (only GET calls are made against PagerDuty). A full-access key works too.
