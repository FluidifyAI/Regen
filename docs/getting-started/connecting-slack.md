# Connecting Slack

Fluidify Regen integrates with Slack to automatically create incident channels, post updates, and let your team manage incidents without leaving Slack.

## What you get

- Incident channel auto-created when an alert fires (`#inc-042-redis-memory-high`)
- Status updates posted to the channel on every lifecycle change
- Slash commands: `/regen new`, `/regen ack`, `/regen resolve`, `/regen status`, `/regen note`, `/regen list`
- Interactive buttons: **Make me Lead**, **Add Note**, **Overview**
- Slack replies synced back into the incident timeline
- AI summaries and handoff digests posted directly to the channel

## How it works (read this first)

Regen uses Slack's **HTTP Events API**. Slack delivers everything inbound — messages, button clicks, slash commands — by sending an HTTPS **POST to your server**, signed with your app's signing secret:

| What the user does in Slack | Slack POSTs to | Slack app setting |
|---|---|---|
| Sends a message / @mentions the bot | `…/api/v1/slack/events` | **Event Subscriptions** |
| Clicks a button (Make me Lead, etc.) | `…/api/v1/slack/interactions` | **Interactivity & Shortcuts** |
| Runs a slash command (`/regen …`) | `…/api/v1/slack/commands` | **Slash Commands** |

Two consequences that trip people up:

1. **Your server must be reachable from the public internet.** Slack can't POST to `localhost` — for local development you need a tunnel (see [Local development](#local-development-ngrok)).
2. **The signing secret stored in Regen must match your Slack app's signing secret**, or every inbound POST is rejected with **403** (signature verification failure).

> Regen does **not** use Socket Mode. There is no `SLACK_APP_TOKEN` / `xapp-…` token — if you see that in older notes, ignore it; it does nothing.

## The fast path: the in-app wizard

The easiest setup is **Settings → Integrations → Slack → Connect Slack** in the Regen UI. It generates a pre-filled Slack **app manifest** (correct scopes, slash command, and all three Request URLs pointing at your Regen URL) so you don't configure them by hand. Follow the wizard, paste your Bot Token and Signing Secret, and you're done.

The manual steps below are for when you'd rather configure the Slack app yourself.

## Manual setup

### Step 1: Create a Slack app

1. Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **From scratch**
2. Name it `Fluidify Regen`, select your workspace, **Create App**

### Step 2: Bot Token Scopes

**OAuth & Permissions → Bot Token Scopes** — add:

| Scope | Purpose |
|-------|---------|
| `channels:manage` | Create and archive incident channels |
| `channels:read` | Read channel info |
| `channels:history` | Read channel messages (timeline sync) |
| `channels:write.invites` | Invite responders to incident channels |
| `chat:write` | Post messages |
| `chat:write.public` | Post to channels the bot hasn't joined |
| `commands` | Register the slash command |
| `app_mentions:read` | Respond to `@Fluidify Regen` mentions |
| `reactions:read` | Read reactions (e.g. ack via emoji) |
| `im:write` | Send DMs (shift handoffs, escalations) |
| `users:read` | Resolve user display names |
| `users:read.email` | Match Slack users to Regen accounts |

### Step 3: Set the three Request URLs

Replace `https://your-domain.com` with your public Regen URL (or your tunnel URL for local dev).

**Event Subscriptions** → toggle **Enable Events**:
- **Request URL**: `https://your-domain.com/api/v1/slack/events`
  Slack immediately sends a `url_verification` challenge — it must show **Verified ✓**. If it returns 403, your signing secret doesn't match (see Step 6).
- **Subscribe to bot events**: `app_mention`, `message.channels`, `reaction_added`

**Interactivity & Shortcuts** → toggle **Interactivity** **On**:
- **Request URL**: `https://your-domain.com/api/v1/slack/interactions`
  (No verification challenge — this one works the moment you save. **Buttons do nothing until this is set.**)

**Slash Commands** → **Create New Command**:

| Command | Request URL | Description |
|---------|-------------|-------------|
| `/regen` | `https://your-domain.com/api/v1/slack/commands` | Manage incidents — new, ack, resolve, status, note, lead, list |

### Step 4: Install the app

**OAuth & Permissions → Install to Workspace** → authorize. Copy the **Bot User OAuth Token** (`xoxb-…`).

### Step 5: Signing secret

**Basic Information → App Credentials → Signing Secret** → copy it.

### Step 6: Configure Regen

Either in the UI — **Settings → Integrations → Slack** (recommended; takes effect immediately, no restart) — or via `.env`:

```env
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...
```

> The signing secret here **must be identical** to the one on the Slack app's Basic Information page. A mismatch is the #1 cause of inbound 403s.

There is no `SLACK_APP_TOKEN` — Regen uses the HTTP Events API, not Socket Mode.

## Local development (ngrok)

Slack can't reach `localhost`, so you need a public tunnel to your backend (port `8080`):

```bash
ngrok http 8080
```

ngrok prints a public URL, e.g. `https://abc123.ngrok-free.app`. Use it as the base for **all three** Request URLs in Step 3:

```
https://abc123.ngrok-free.app/api/v1/slack/events
https://abc123.ngrok-free.app/api/v1/slack/interactions
https://abc123.ngrok-free.app/api/v1/slack/commands
```

> ⚠️ **ngrok's free URL changes every time you restart it.** When that happens, the old URLs go dead and you must update **all three** in the Slack app again. A paid ngrok **static domain** (`ngrok http 8080 --domain=…`) avoids this — recommended if you demo often.

## Verifying the connection

```bash
make logs | grep slack
```

On startup you should see (note: **not** "socket mode"):

```
slack service initialized            workspace=YourWorkspace
slack http event handler initialized bot_id=B... team=YourWorkspace
```

Then send a message in an incident channel and watch a request arrive:

```bash
make logs | grep "/api/v1/slack/events"
# POST /api/v1/slack/events  status=200
```

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Messages typed in Slack don't appear in the UI | No inbound request reaching the server | Tunnel down / wrong Events Request URL — restart ngrok, update the URL |
| Buttons show a ⚠️ and do nothing | Interactivity Request URL not set (or stale) | Set **Interactivity & Shortcuts** Request URL to `…/api/v1/slack/interactions` |
| Slack shows the Request URL won't verify, or logs show **403** | Signing secret in Regen ≠ Slack app's signing secret | Copy the exact Signing Secret from Basic Information into Settings → Slack |
| Slash commands do nothing | Slash command Request URL not set/stale | Set it to `…/api/v1/slack/commands` |
| `/regen` works but channel messages don't sync | Missing `message.channels` bot event | Add it under Event Subscriptions, reinstall if prompted |

## Slack commands reference

| Command | Description |
|---------|-------------|
| `/regen new <title>` | Create a new incident |
| `/regen ack` | Acknowledge the current incident (in an incident channel) |
| `/regen resolve` | Resolve the current incident |
| `/regen status` | Show current incident status |
| `/regen note <text>` | Add a note to the incident timeline |
| `/regen list` | List open incidents |

Commands also work by @mentioning the bot: `@Fluidify Regen ack`
