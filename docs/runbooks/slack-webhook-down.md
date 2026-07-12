# Runbook: Slack webhook down

**Symptom:** Incidents are created in the Regen UI and timeline entries appear, but no Slack channels are created and no messages are posted.

---

## Diagnose

**1. Check the app logs for Slack errors:**

```bash
# Docker Compose
docker logs fluidify-regen --since 30m 2>&1 | grep -i "slack\|channel\|webhook"

# Kubernetes
kubectl logs -n fluidify deploy/fluidify-regen --since=30m | grep -i "slack\|channel\|webhook"
```

Common log patterns:
- `slack: channel creation failed: invalid_auth` — bot token is wrong or revoked
- `slack: channel creation failed: missing_scope` — bot is missing `channels:manage` scope
- `slack: post message failed: not_in_channel` — bot was removed from the channel
- `slack: post message failed: channel_not_found` — channel was archived or deleted

**2. Test the bot token directly:**

```bash
curl -s -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  https://slack.com/api/auth.test | jq .
```

Expected: `"ok": true`. If `"ok": false`, the token is invalid or revoked.

**3. Check signing secret:**

If the Slack Events API is delivering events but actions (ack, resolve commands) don't work, the signing secret may be wrong. Check for `invalid_signature` errors in logs.

---

## Mitigate

**If token is invalid/revoked:**

1. Go to your Slack app at [api.slack.com/apps](https://api.slack.com/apps)
2. **OAuth & Permissions → Bot tokens** — reinstall the app to your workspace to regenerate the token
3. Update `SLACK_BOT_TOKEN` in your `.env` or Kubernetes secret
4. Restart the app:

```bash
# Docker Compose
make start

# Kubernetes
kubectl rollout restart deploy/fluidify-regen -n fluidify
```

**If missing scope:**

1. Go to your Slack app → **OAuth & Permissions → Scopes**
2. Ensure these bot token scopes are present: `channels:manage`, `chat:write`, `chat:write.public`, `commands`, `users:read`, `users:read.email`
3. Reinstall the app to your workspace after adding scopes

**If the bot was removed from a channel:**

The bot will rejoin automatically when the next incident is created for that channel. For existing channels, manually invite the bot:

```
/invite @YourBotName
```

---

## Resolve

Once the token/scopes are fixed and the app is restarted:

1. Verify with `auth.test` again
2. Create a test incident from the UI
3. Confirm the Slack channel appears within 10 seconds

---

## Prevention

- Configure an uptime monitor on `GET /ready` — it checks Slack connectivity is not verified there, but app restarts will surface token errors in logs
- Rotate bot tokens via Slack's app management, not by deleting and recreating the app
- Store `SLACK_BOT_TOKEN` and `SLACK_SIGNING_SECRET` in a secret manager, not in a `.env` file committed to source control
