# Connecting Telegram

Fluidify Regen can send incident notifications to a Telegram group or channel. This is useful for teams that use Telegram for operations alerts or want a lightweight notification channel alongside Slack or Teams.

## What you get

- Notification when a new incident is created (with severity and direct link)
- Notification on every status change (acknowledged, resolved)
- AI summary posted to the group when generated
- Severity-coded emoji for quick visual scanning (🔴 critical, 🟠 high, 🟡 medium, 🔵 low)

## Limitations

Telegram is **notification-only** in the current version. This is a deliberate design decision — the Telegram Bot API does not support the same channel management model as Slack or Teams.

| Feature | Slack | Teams | Telegram |
|---------|-------|-------|----------|
| Incident channel auto-created | ✅ | ✅ | ❌ |
| Status updates posted | ✅ | ✅ | ✅ |
| Bot commands (`ack`, `resolve`) | ✅ | ✅ | ❌ |
| Interactive buttons | ✅ | ✅ | ❌ |
| Timeline sync (replies → UI) | ✅ | ✅ | ❌ |
| AI summary notification | ✅ | ✅ | ✅ |
| Shift notifications (on-call) | ✅ | ✅ | ❌ |

**Use Telegram when:** your team already uses it and wants passive alert visibility. Use Slack or Teams for full incident management with commands and two-way sync.

---

## Step 1: Create a Telegram bot

1. Open Telegram and search for **@BotFather**
2. Send `/newbot`
3. Follow the prompts — choose a name (e.g. `Fluidify Regen`) and a username (e.g. `fluidify_regen_bot`)
4. BotFather replies with your **bot token**: `7123456789:AAFxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

Keep this token private — it gives full control of the bot.

## Step 2: Add the bot to a group

1. Create a Telegram group (or use an existing one) for incident notifications
2. Add your bot to the group as a member
3. Send any message in the group (e.g. "hello") — this is needed for Step 3

> For a **Telegram channel** (broadcast-only), add the bot as an **administrator** instead of a member.

## Step 3: Find the Chat ID

The Chat ID is required so Regen knows where to send messages.

**Option A — via the Regen UI (easiest):**

1. Go to **Settings → Integrations → Telegram**
2. Enter your bot token and click **Auto-detect Chat ID**
3. Regen calls `getUpdates` and returns the most recent group the bot has seen

**Option B — manually:**

Send this request (replace `<token>` with your bot token):

```bash
curl "https://api.telegram.org/bot<token>/getUpdates"
```

Look for the `chat.id` field in the response. Group chat IDs are negative numbers (e.g. `-1001234567890`).

## Step 4: Configure in Regen

**Via the UI:**

1. Go to **Settings → Integrations → Telegram**
2. Enter your **Bot Token** and **Chat ID**
3. Click **Test Connection** — Regen sends a test message to verify
4. Click **Save**

**Via environment variables** (alternative, requires restart):

```env
TELEGRAM_BOT_TOKEN=7123456789:AAFxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
TELEGRAM_CHAT_ID=-1001234567890
```

## Verifying the connection

After saving, Regen sends a test message to the group:

> ✅ **Fluidify Regen connected**
> Incident notifications will appear here.

If you don't see this message, check:
- The bot is a member of the group
- You sent a message in the group after adding the bot (required for `getUpdates` to return the chat)
- The Chat ID is correct (group IDs start with `-100`)

## Example notifications

**Incident created:**
> 🔴 **INC-042 — CRITICAL**
> Payments API returning 500s
>
> [View incident →](https://your-domain.com/incidents/...)

**Status update:**
> ✅ **INC-042 Resolved**
> Payments API returning 500s
>
> [View incident →](https://your-domain.com/incidents/...)

## Disconnecting

Go to **Settings → Integrations → Telegram** and click **Disconnect**. Regen stops sending notifications immediately. The bot remains in your group — remove it manually from Telegram if no longer needed.
