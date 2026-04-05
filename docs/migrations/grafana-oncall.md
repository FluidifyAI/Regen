# Migrating from Grafana OnCall OSS to Regen

Grafana OnCall OSS was archived in March 2026. If you're running a self-hosted
OnCall instance, Regen is the direct replacement — and you don't need to change
anything else in your Grafana stack. Keep using Grafana for dashboards, alerting,
and panels. Just replace the OnCall part.

---

## Why migrate to Regen?

- **Keep Grafana for everything else** — Regen receives webhooks from Grafana
  Alertmanager exactly like OnCall did. No observability stack changes required.
- **One import, done** — your schedules, escalation policies, and team members
  transfer in under 60 seconds via the built-in migration wizard.
- **Open source, self-hosted, free** — AGPLv3, no user limits in the community
  edition on a self-hosted install, no SaaS fees.
- **On-call + incident management in one tool** — Regen handles the full loop:
  alert → incident → timeline → post-mortem.

---

## What transfers

| OnCall concept | Regen equivalent | Status |
|---|---|---|
| Users | Users | ✅ Full (email, name, role, Slack ID) |
| Schedules (rotation layers) | Schedules + layers | ✅ Full |
| Escalation chains | Escalation policies | ✅ Full |
| Integrations (webhook URLs) | Webhook endpoints | ✅ New URLs provided |
| Teams | — | ⚠️ Not imported (no equivalent) |
| Schedule overrides | — | ⚠️ Planned for next release |
| Notification policies | — | ⚠️ Planned for v1.1 |
| Alert history | — | ⚠️ Optional in a future release |
| Mobile push | — | ❌ Neither product supports mobile push |

---

## What doesn't transfer

- **Notification policies** — per-user paging rules (SMS, phone call, push) are
  on the roadmap for Regen v1.1. After import, users will be notified via
  Slack/Teams using the channels you configure.
- **Mobile push** — Grafana OnCall doesn't have a mobile app and neither does
  Regen (same situation — no gap here).
- **Alert history** — historical alert groups are not imported. Your new Regen
  instance starts fresh; OnCall can continue receiving alerts in parallel during
  transition.
- **iCal schedules** — schedules defined by an external iCal URL cannot be
  converted to Regen's layer model automatically. Recreate these manually.

---

## Prerequisites

1. **Regen is installed and running.** Follow the [installation guide](../README.md).
2. **You have admin access to Regen** — the migration wizard is admin-only.
3. **You have a Grafana OnCall API token.** Generate one in Grafana OnCall →
   Settings → API Tokens → Create. Copy the token — you'll only see it once.
4. **Your Grafana OnCall instance is reachable** from the machine running Regen
   (the backend makes direct HTTP calls to the OnCall API).

---

## Step-by-step import walkthrough

### 1. Open the migration wizard

In Regen, go to **Settings → Migrations** in the left sidebar.

### 2. Connect to Grafana OnCall

Enter your Grafana instance URL (e.g. `https://grafana.yourcompany.com`) and
your API token. Click **Preview import**.

Regen connects to OnCall, fetches all your data, and shows you exactly what will
be created — without writing anything yet.

### 3. Review the preview

The preview screen shows:

- **Summary cards** — counts of users, schedules, escalation policies, and
  webhook integrations to be created.
- **Conflicts** — anything that already exists in Regen (matching email or name)
  will be skipped. These are shown in an amber banner.
- **Detail tables** — expand each section to see exactly what will be created.

If everything looks right, click **Import everything**.

### 4. Import complete

The results screen shows:

- A green success banner with counts of what was created.
- **Webhook URL update table** — the most important part. For each integration
  that existed in OnCall, you get the new Regen URL to put in its place. You
  **must** update these URLs in Grafana Alertmanager (or wherever you send alerts)
  before you can receive alerts in Regen.
- **User setup links** — each imported user gets a one-time setup link. Share
  these so your team members can set their own passwords and log in.

---

## After the import

### Update webhook URLs in Grafana Alertmanager

This is the critical step. Until you update these URLs, your alerts still go to
OnCall, not Regen.

For Prometheus Alertmanager, update your `alertmanager.yml`:

```yaml
receivers:
  - name: 'regen'
    webhook_configs:
      - url: 'https://regen.yourcompany.com/api/v1/webhooks/prometheus'
        send_resolved: true
```

For Grafana Alerting contact points:
1. Go to **Grafana → Alerting → Contact points**
2. Edit the OnCall contact point
3. Change the URL to your new Regen webhook URL (shown in the results screen)
4. Click **Test** to verify it reaches Regen

### Update Grafana contact points (Grafana Alerting)

If you use Grafana's built-in alerting (not Prometheus Alertmanager), update
your Grafana contact point:

1. Grafana → Alerting → Contact points → edit your existing webhook
2. Replace the URL with: `https://regen.yourcompany.com/api/v1/webhooks/grafana`
3. Test the contact point

### Share setup links with your team

Each imported user received a one-time setup link. Copy these from the results
screen (or use **Copy all as CSV**) and share them via Slack or email. Users
click the link, set their password, and are immediately logged in.

### Verify your schedules

Go to **On-call → Schedules** and confirm:

- All your rotation schedules are present
- Layer participant order looks correct
- Timezones are correct

If you had iCal-based schedules, recreate these manually using **On-call →
Schedules → Create schedule**.

### Test end-to-end

1. Send a test alert from Grafana Alertmanager or your monitoring tool
2. Confirm it appears as an alert in Regen's alert list
3. Confirm an incident is created automatically (if you have routing rules configured)
4. Confirm any on-call notifications fire correctly

---

## Running OnCall and Regen in parallel

You can run both systems simultaneously during the transition. Point your
monitoring tools at both endpoints:

```yaml
# alertmanager.yml — send to both during transition
receivers:
  - name: 'all'
    webhook_configs:
      - url: 'https://grafana.yourcompany.com/integrations/v1/alertmanager/OLD_TOKEN/'
      - url: 'https://regen.yourcompany.com/api/v1/webhooks/prometheus'
```

Once you're confident Regen is working correctly, remove the OnCall URL.

---

## Frequently asked questions

**Will my on-call history transfer?**
No. Historical alert groups from OnCall are not imported in this version. Your
new Regen instance starts with a clean slate. Historical data can be viewed in
OnCall until you decommission it.

**What about mobile notifications?**
Neither Grafana OnCall OSS nor Regen has a mobile app. This is not a gap — the
situation is the same on both sides.

**Can I run both at the same time?**
Yes. See "Running OnCall and Regen in parallel" above. Both can receive the same
alerts simultaneously during your transition window.

**What if some users already exist in Regen?**
Users with matching email addresses are skipped (not overwritten). They appear in
the "conflicts" section of the preview. Their existing Regen accounts are unaffected.

**What if I import twice?**
The import is idempotent. Running it a second time skips anything that already
exists (matching email, schedule name, or policy name). No duplicates are created.

**What happens to my notification policies?**
Grafana OnCall lets each user configure their own paging rules (SMS, phone call,
push notification). Regen doesn't yet have per-user notification policies —
this feature is on the roadmap for v1.1. In the meantime, users are notified via
Slack (if you have the Slack integration configured) and via the Regen web UI.

**What if my Grafana OnCall is behind a VPN?**
The Regen backend must be able to reach your Grafana OnCall API during the import.
If your OnCall instance is behind a VPN, run the migration from a machine on the
same network, or temporarily expose the API endpoint.

---

## Verification checklist

- [ ] Webhook URLs updated in Grafana Alertmanager / Grafana contact points
- [ ] Test alert received in Regen
- [ ] Schedules look correct (correct participants, timezones, rotation order)
- [ ] Escalation policies have the right tiers
- [ ] All team members received setup links and can log in
- [ ] On-call rotation is active (check **On-call → who's on call now**)
