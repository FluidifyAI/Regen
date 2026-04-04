# Connecting Microsoft Teams

Fluidify Regen integrates with Microsoft Teams to automatically create incident channels, post Adaptive Cards, and let your team manage incidents without leaving Teams.

## What you get

- Incident channel auto-created when an alert fires
- Adaptive Card posted in the channel with incident details and action buttons
- Card updated on every status change (acknowledged, resolved)
- Bot commands: `@Fluidify Regen ack`, `resolve`, `new <title>`, `status`
- Slack parity — same incident lifecycle, same timeline sync
- Shift notifications via DM to the incoming on-call user

## Architecture overview

The Teams integration uses **two separate Azure credentials**:

| Client | OAuth scope | Used for |
|--------|-------------|----------|
| Graph API | `https://graph.microsoft.com/.default` | Creating incident channels |
| Bot Framework | `https://api.botframework.com/.default` | Posting messages to channels |

This split is required because `ChannelMessage.Send` in the Graph API only works with delegated (user) auth — app-only tokens get a 403. Bot Framework Proactive Messaging uses a different scope that works with client credentials, giving full Slack parity.

## Prerequisites

- Microsoft Azure account with admin access to your tenant
- Microsoft Teams admin access to install apps
- Regen must be accessible over HTTPS

---

## Step 1: Create an App Registration

1. Go to [portal.azure.com](https://portal.azure.com) → **Azure Active Directory → App registrations → New registration**
2. Name: `Fluidify Regen`
3. Supported account types: **Single tenant**
4. Redirect URI: leave blank
5. Click **Register**

Note down:
- **Application (client) ID** → `TEAMS_APP_ID`
- **Directory (tenant) ID** → `TEAMS_TENANT_ID`

## Step 2: Create a client secret

1. In your App Registration → **Certificates & secrets → New client secret**
2. Description: `regen-prod`, Expires: 24 months
3. Click **Add** and immediately copy the **Value**

This is your `TEAMS_APP_PASSWORD`. It is only shown once.

## Step 3: Add Graph API permissions

1. Go to **API permissions → Add a permission → Microsoft Graph → Application permissions**
2. Add these permissions:

| Permission | Purpose |
|------------|---------|
| `Channel.Create` | Create incident channels |
| `Channel.ReadBasic.All` | Read channel info |
| `ChannelMember.ReadWrite.All` | Add members to channels |
| `Team.ReadBasic.All` | Read team info (validates on startup) |
| `TeamMember.ReadWrite.All` | Add users to the team |

3. Click **Grant admin consent** (requires Azure AD admin role)

## Step 4: Create an Azure Bot Service

> This step is critical and commonly missed. Without it, the Bot Framework API permission does not exist in your tenant to grant.

1. In Azure portal, search for **Azure Bot** → **Create**
2. Bot handle: `fluidify-regen`
3. Subscription and resource group: your choice
4. **Type of App**: Multi Tenant → change to **Single Tenant**
5. **Creation type**: Use existing app registration → enter your App ID from Step 1
6. Click **Review + Create → Create**

Once created, go to the Bot resource → **Configuration**:
- Note the **Microsoft App ID** (should match your App Registration)
- Set the **Messaging endpoint**: `https://your-domain.com/api/v1/teams/events`

## Step 5: Add Bot Framework API permission

1. Go back to your **App Registration → API permissions → Add a permission**
2. Click **APIs my organization uses**
3. Search for **Bot Framework** (only appears after Step 4 creates the Azure Bot Service resource)
4. Select **Application permissions → access_as_application**
5. Click **Add permissions**
6. Click **Grant admin consent**

## Step 6: Get the Team ID

Find the ID of the Team where Regen should create incident channels:

**Option A — from the Teams desktop app:**
1. Right-click the Team → **Get link to team**
2. The link contains `groupId=<your-team-id>` — copy that value

**Option B — via Graph API:**
```bash
curl -H "Authorization: Bearer <token>" \
  "https://graph.microsoft.com/v1.0/teams?$select=id,displayName"
```

This is your `TEAMS_TEAM_ID`.

## Step 7: Get the Bot User ID

1. In Azure portal → **Azure Active Directory → Users**
2. Search for the bot user (named after your app registration, type: Service Principal)
3. Copy the **Object ID**

Alternatively, use the Graph API:
```bash
curl -H "Authorization: Bearer <token>" \
  "https://graph.microsoft.com/v1.0/servicePrincipals?$filter=appId eq '<TEAMS_APP_ID>'&$select=id"
```

This is your `TEAMS_BOT_USER_ID`.

## Step 8: Sideload the bot into your Team

Regen provides a script to generate the Teams app package:

```bash
TEAMS_APP_ID=your-app-id bash scripts/teams-app-package.sh
```

This creates `fluidify-regen-teams-app.zip`. Install it:

1. In Teams → **Apps → Manage your apps → Upload an app**
2. Select `fluidify-regen-teams-app.zip`
3. Add it to the specific Team where incidents should be created

> The bot must be installed in the Team before it can post messages. Without this step, Bot Framework returns `BadSyntax` on the first post.

## Step 9: Configure Regen

Add to your `.env`:

```env
TEAMS_APP_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
TEAMS_APP_PASSWORD=your-client-secret
TEAMS_TENANT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
TEAMS_TEAM_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
TEAMS_BOT_USER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Region-specific relay URL (default is US/amer)
# Change if your tenant is in a different region:
# EU:    https://smba.trafficmanager.net/emea/
# India: https://smba.trafficmanager.net/in/
# APAC:  https://smba.trafficmanager.net/apac/
TEAMS_SERVICE_URL=https://smba.trafficmanager.net/amer/
```

Then restart: `make stop && make start`

## Verifying the connection

Check logs:

```bash
make logs | grep teams
```

On startup you should see:
```
teams integration initialized  team_id=...
```

If you see an error about Graph API failing, check that admin consent was granted for all permissions (Step 3).

## Bot commands

Send these in any incident channel or @mention the bot anywhere:

| Command | Description |
|---------|-------------|
| `@Fluidify Regen ack` | Acknowledge the incident |
| `@Fluidify Regen resolve` | Resolve the incident |
| `@Fluidify Regen new <title>` | Create a new incident |
| `@Fluidify Regen status` | Show current incident status |

Regular messages in an incident channel (non-commands) are synced to the incident timeline in the UI.

## Region-specific setup

The Bot Framework relay URL is region-specific. If messages aren't posting, verify your `TEAMS_SERVICE_URL` matches your tenant's region. You can find the correct URL in the `serviceUrl` field of any inbound activity payload from Teams.

| Region | URL |
|--------|-----|
| Americas (default) | `https://smba.trafficmanager.net/amer/` |
| Europe | `https://smba.trafficmanager.net/emea/` |
| India | `https://smba.trafficmanager.net/in/` |
| Asia Pacific | `https://smba.trafficmanager.net/apac/` |

## Troubleshooting

| Error | Cause | Fix |
|-------|-------|-----|
| 403 on Graph API | Admin consent not granted | Re-do Step 3, click Grant admin consent |
| 401 on Bot Framework | Bot Framework permission missing | Complete Steps 4–5 |
| `BadSyntax` on first post | Bot not installed in the Team | Complete Step 8 |
| Messages not posting | Wrong `TEAMS_SERVICE_URL` | Match your tenant's region |
| Bot not responding | Bot not sideloaded or app package stale | Re-run `scripts/teams-app-package.sh` and re-upload |
