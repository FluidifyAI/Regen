# Connecting Slack

Fluidify Regen integrates with Slack to automatically create incident channels, post updates, and let your team manage incidents without leaving Slack.

## What you get

- Incident channel auto-created when an alert fires (`#inc-042-redis-memory-high`)
- Status updates posted to the channel on every lifecycle change
- Slack commands: `/incident new`, `/incident ack`, `/incident resolve`, `/incident status`
- Interactive buttons: **Make me Lead**, **Add Note**
- AI summaries and handoff digests posted directly to the channel

## Step 1: Create a Slack app

1. Go to [api.slack.com/apps](https://api.slack.com/apps) and click **Create New App**
2. Choose **From scratch**
3. Name it `Fluidify Regen` and select your workspace
4. Click **Create App**

## Step 2: Configure OAuth scopes

In the left sidebar, go to **OAuth & Permissions** → **Bot Token Scopes** and add:

| Scope | Purpose |
|-------|---------|
| `channels:manage` | Create and archive incident channels |
| `channels:read` | Read channel info |
| `chat:write` | Post messages to channels |
| `chat:write.public` | Post to channels the bot hasn't joined |
| `commands` | Register slash commands |
| `users:read` | Resolve user display names |
| `users:read.email` | Match Slack users to Regen accounts |

## Step 3: Enable Socket Mode

Socket Mode allows interactive buttons (Make me Lead, Add Note) to work. Without it, buttons will show an error when clicked.

1. Go to **Socket Mode** in the left sidebar
2. Toggle **Enable Socket Mode** on
3. Click **Generate an app-level token**
4. Name it `regen-socket`, add the scope `connections:write`
5. Click **Generate** — copy the token starting with `xapp-`

This is your `SLACK_APP_TOKEN`.

## Step 4: Add slash commands

Go to **Slash Commands** → **Create New Command** and add:

| Command | Request URL | Description |
|---------|-------------|-------------|
| `/incident` | `https://your-domain.com/api/v1/slack/commands` | Manage incidents from Slack |

If using Socket Mode (recommended), the Request URL is only needed for the Slack app manifest — actual requests come through the Socket connection.

## Step 5: Enable Event Subscriptions

1. Go to **Event Subscriptions** → toggle **Enable Events**
2. Under **Subscribe to bot events** add:
   - `app_mention` — lets users @mention the bot
   - `message.channels` — syncs Slack replies to the timeline

## Step 6: Install the app to your workspace

1. Go to **OAuth & Permissions** → click **Install to Workspace**
2. Authorize the app
3. Copy the **Bot User OAuth Token** starting with `xoxb-`

This is your `SLACK_BOT_TOKEN`.

## Step 7: Get your Signing Secret

Go to **Basic Information** → scroll to **App Credentials** → copy **Signing Secret**.

This is your `SLACK_SIGNING_SECRET`.

## Step 8: Configure Regen

Add to your `.env`:

```env
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...
SLACK_APP_TOKEN=xapp-...
```

Then restart: `make stop && make start`

Or configure directly in the UI: **Settings → Integrations → Slack**.

## Verifying the connection

Once running, check the logs:

```bash
make logs | grep slack
```

You should see:

```
slack socket mode initialized  bot_id=B... team=YourWorkspace
slack socket mode connected - bidirectional sync active
```

## Slack commands reference

| Command | Description |
|---------|-------------|
| `/incident new <title>` | Create a new incident |
| `/incident ack` | Acknowledge the current incident (in an incident channel) |
| `/incident resolve` | Resolve the current incident |
| `/incident status` | Show current incident status |

Commands also work by @mentioning the bot: `@Fluidify Regen ack`
