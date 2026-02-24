# OpenIncident

**Open-source incident management for teams who own their data.**

incident.io + PagerDuty, self-hosted, with BYO-AI.

---

## Why OpenIncident?

| Problem | Our Solution |
|---------|--------------|
| **$100k/year** on incident tooling for a 200-person team | **Free** open-source core, flat enterprise pricing |
| **Data sovereignty** concerns blocking SaaS adoption | **Self-hosted** — your data never leaves your infrastructure |
| **Tool fragmentation** — alerts here, incidents there, post-mortems somewhere else | **Unified platform** — alerts, incidents, scheduling, AI in one place |
| **Grafana OnCall archived** in March 2026 | **Spiritual successor** with full incident lifecycle |

---

## Features

### Core (Free, AGPLv3)

- **Alert Ingestion** — Prometheus, Grafana, CloudWatch, generic webhooks
- **Incident Management** — Full lifecycle with immutable timeline
- **Slack Integration** — Auto-create channels, bidirectional sync
- **Microsoft Teams Integration** — Auto-create channels, Adaptive Cards, bot commands
- **On-Call Scheduling** — Rotations, layers, overrides
- **Escalation Policies** — Multi-tier escalation with timeouts
- **AI Summarization** — Incident summaries, post-mortem drafts (BYO OpenAI key)
- **Docker & Kubernetes** — Deploy anywhere

### Enterprise (Paid License)

- SSO/SAML (Okta, Azure AD, Google)
- SCIM user provisioning
- Audit log export
- Role-based access control (RBAC)
- Data retention policies
- Priority support

---

## Quick Start

### Prerequisites

Before you begin, ensure you have:

- **Docker**: v20.10 or later ([install guide](https://docs.docker.com/get-docker/))
- **Docker Compose**: v2.0 or later (included with Docker Desktop)
- **Slack Workspace**: Admin access to create apps (optional but recommended)
- **Git**: For cloning the repository

### 1. Clone and Configure

Clone the repository:

```bash
git clone https://github.com/yourusername/openincident.git
cd openincident
```

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` with your configuration. **At minimum**, verify these are set:

```env
DATABASE_URL=postgresql://openincident:secret@db:5432/openincident?sslmode=disable
REDIS_URL=redis://redis:6379
PORT=8080
```

For Slack integration (optional but recommended), add:

```env
SLACK_BOT_TOKEN=xoxb-your-token-here
SLACK_SIGNING_SECRET=your-signing-secret-here
SLACK_APP_TOKEN=xapp-your-app-token-here   # Required for Socket Mode (interactive features)
```

To auto-invite specific users (e.g. SRE leads) to every incident channel:

```env
SLACK_AUTO_INVITE_USER_IDS=U01234ABCDE,U56789FGHIJ
```

See [Slack App Setup](#slack-app-setup) below for how to obtain these credentials.

### 2. Start Services

Start all services (PostgreSQL, Redis, Backend, Frontend):

```bash
docker-compose up -d
```

**Wait 10-15 seconds** for database migrations to complete. Check status:

```bash
docker-compose logs backend | grep "server starting"
```

You should see: `"server starting" port=8080`

### 3. Verify Health

Check that all services are ready:

```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}

curl http://localhost:8080/ready
# Expected: {"database":"ok","redis":"ok","status":"ready"}
```

### 4. Access the UI

Open your browser to:

- **Frontend**: http://localhost:3000
- **API**: http://localhost:8080
- **Metrics**: http://localhost:8080/metrics

### 5. Test with a Sample Alert

Send a test Prometheus alert to verify the webhook is working:

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "openincident",
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighErrorRate",
        "severity": "critical",
        "service": "api"
      },
      "annotations": {
        "summary": "Error rate above 5%",
        "description": "The API service is experiencing elevated error rates"
      },
      "startsAt": "2024-01-15T10:00:00Z"
    }]
  }'
```

Check the response and verify an incident was created:

```bash
curl http://localhost:8080/api/v1/incidents
```

If Slack is configured, you should see a new channel created like `#incident-001-high-error-rate`.

### 6. Configure Prometheus Alertmanager (Optional)

To receive alerts from your existing Prometheus setup, add to your `alertmanager.yml`:

```yaml
receivers:
  - name: openincident
    webhook_configs:
      - url: http://localhost:8080/api/v1/webhooks/prometheus
        send_resolved: true
```

Reload Alertmanager:

```bash
curl -X POST http://localhost:9093/-/reload
```

---

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Prometheus    │     │     Grafana     │     │   CloudWatch    │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │    OpenIncident API    │
                    │  (Go + Gin)            │
                    └────────────┬───────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
              ▼                  ▼                  ▼
       ┌──────────┐       ┌──────────┐       ┌──────────┐
       │PostgreSQL│       │  Redis   │       │  Slack   │
       └──────────┘       └──────────┘       └──────────┘
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [CLAUDE.md](docs/CLAUDE.md) | Project context and build guide |
| [PRODUCT.md](docs/PRODUCT.md) | Product vision, roadmap, business model |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | System design, data models, APIs |
| [DECISIONS.md](docs/DECISIONS.md) | Architecture Decision Records |

---

## Roadmap

- [x] **v0.1** — Prometheus → Incident → Slack
- [ ] **v0.2** — Incident lifecycle, timeline
- [ ] **v0.3** — Multi-source alerts, routing
- [ ] **v0.4** — On-call rotations
- [ ] **v0.5** — Escalation policies
- [ ] **v0.6** — AI summarization
- [ ] **v0.7** — Post-mortem generation
- [x] **v0.8** — Microsoft Teams integration (channels, Adaptive Cards, bot commands)
- [ ] **v0.9** — Enterprise features (SSO, RBAC)
- [ ] **v1.0** — Production ready

---

## API Example

### Create an Incident

```bash
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "title": "Database connection errors",
    "severity": "high",
    "description": "Multiple services reporting DB timeouts"
  }'
```

### Receive a Prometheus Alert

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "openincident",
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighErrorRate",
        "severity": "critical"
      },
      "annotations": {
        "summary": "Error rate above 5%"
      },
      "startsAt": "2024-01-15T10:00:00Z"
    }]
  }'
```

---

## Configuration

### Environment Variables

```env
# Required
DATABASE_URL=postgresql://user:pass@localhost:5432/openincident
REDIS_URL=redis://localhost:6379

# Slack Integration
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...
SLACK_APP_TOKEN=xapp-...              # Socket Mode (interactive features)
SLACK_AUTO_INVITE_USER_IDS=           # Comma-separated user IDs, e.g. U01234,U56789

# Microsoft Teams Integration (optional)
TEAMS_APP_ID=<azure-app-registration-id>
TEAMS_APP_PASSWORD=<client-secret>
TEAMS_TENANT_ID=<directory-tenant-id>
TEAMS_TEAM_ID=<team-id>
TEAMS_BOT_USER_ID=<bot-service-principal-id>   # optional, needed for DMs
TEAMS_SERVICE_URL=https://smba.trafficmanager.net/amer/  # see region table below

# Optional: AI Features
OPENAI_API_KEY=sk-...

# Optional: App Settings
PORT=8080
LOG_LEVEL=info
APP_ENV=production
```

### Slack App Setup

#### Step-by-Step Guide

1. **Create a Slack App**
   - Go to https://api.slack.com/apps
   - Click **"Create New App"**
   - Select **"From scratch"**
   - Name: `OpenIncident` (or your preference)
   - Choose your workspace
   - Click **"Create App"**

2. **Add Bot Token Scopes**
   - In the left sidebar, click **"OAuth & Permissions"**
   - Scroll to **"Scopes"** → **"Bot Token Scopes"**
   - Click **"Add an OAuth Scope"** and add each of these:
     - `channels:manage` — Create and archive incident channels
     - `channels:read` — List channels for deduplication
     - `chat:write` — Post status updates to channels
     - `chat:write.public` — Post to channels without joining
     - `users:read` — Resolve user display names for timeline sync
     - `channels:history` — Read channel messages for timeline sync

3. **Install App to Workspace**
   - Scroll up to **"OAuth Tokens for Your Workspace"**
   - Click **"Install to Workspace"**
   - Review permissions and click **"Allow"**

4. **Copy Credentials**
   - After installation, you'll see **"Bot User OAuth Token"**
   - It starts with `xoxb-` — copy this value
   - Add to your `.env` file:
     ```env
     SLACK_BOT_TOKEN=xoxb-1234567890-1234567890123-abcdefghijklmnopqrstuvwx
     ```

5. **Get Signing Secret**
   - In the left sidebar, click **"Basic Information"**
   - Scroll to **"App Credentials"**
   - Under **"Signing Secret"**, click **"Show"** and copy
   - Add to your `.env` file:
     ```env
     SLACK_SIGNING_SECRET=a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
     ```

6. **Enable Socket Mode** (for interactive buttons and slash commands)
   - In the left sidebar, click **"Socket Mode"**
   - Toggle **"Enable Socket Mode"** on
   - Under **"App-Level Tokens"**, click **"Generate Token"**
   - Name: `openincident-socket`, scope: `connections:write`
   - Copy the token (starts with `xapp-`) and add to `.env`:
     ```env
     SLACK_APP_TOKEN=xapp-1-...
     ```

7. **Add Slash Command**
   - In the left sidebar, click **"Slash Commands"**
   - Click **"Create New Command"**
   - Command: `/incident`
   - Request URL: `https://your-domain/slack/events` (or any URL — Socket Mode ignores this)
   - Short description: `Manage incidents`
   - Click **"Save"**

8. **Subscribe to Events** (for Slack→timeline sync)
   - In the left sidebar, click **"Event Subscriptions"**
   - Toggle **"Enable Events"** on
   - Under **"Subscribe to bot events"**, add: `message.channels`
   - Click **"Save Changes"**

9. **Enable Interactive Components**
   - In the left sidebar, click **"Interactivity & Shortcuts"**
   - Toggle **"Interactivity"** on
   - Request URL: `https://your-domain/slack/events` (Socket Mode ignores this)
   - Click **"Save Changes"**

10. **Restart OpenIncident**
    ```bash
    docker-compose restart backend
    ```

11. **Verify Integration**
    - Send a test alert (see [Test with a Sample Alert](#5-test-with-a-sample-alert))
    - Check your Slack workspace for a new channel like `#incident-001-high-error-rate`
    - Try `/incident new` in any channel — a modal should appear
    - If the channel was created, Slack integration is working! ✅

#### Required Scopes Summary

| Scope | Purpose |
|-------|---------|
| `channels:manage` | Create and archive incident channels |
| `channels:read` | List channels to prevent duplicates |
| `channels:history` | Read messages for Slack→timeline sync |
| `chat:write` | Post messages and status updates |
| `chat:write.public` | Post to channels without joining them first |
| `users:read` | Resolve user display names in timeline |

#### Slack Features Overview

| Feature | Requires |
|---------|----------|
| Auto-create incident channels | `SLACK_BOT_TOKEN` |
| Post incident details to channel | `SLACK_BOT_TOKEN` |
| Update card on status change | `SLACK_BOT_TOKEN` |
| Acknowledge/Resolve via buttons | `SLACK_APP_TOKEN` (Socket Mode) |
| `/incident new` slash command | `SLACK_APP_TOKEN` (Socket Mode) |
| Slack messages → timeline sync | `SLACK_APP_TOKEN` (Socket Mode) |
| Archive channel on resolution | `SLACK_BOT_TOKEN` |
| Auto-invite users to channel | `SLACK_AUTO_INVITE_USER_IDS` |

---

### Microsoft Teams App Setup

Teams integration lets OpenIncident automatically create a channel for each incident, post an Adaptive Card with live status, and respond to bot commands (`ack`, `resolve`, `status`, `new`).

**Time required**: ~30 minutes for initial Azure setup, ~5 minutes for subsequent deploys.

#### Prerequisites

- Microsoft 365 tenant with Teams
- Azure Active Directory access to create App Registrations
- Teams team where incidents will be managed
- Permission to install custom apps in Teams (or IT admin who can do it)

#### Step 1 — Create an Azure App Registration

1. Go to [Azure Portal](https://portal.azure.com) → **Azure Active Directory** → **App registrations** → **New registration**
2. Name: `openincident-bot` (or any name)
3. Supported account types: **Single tenant**
4. Click **Register**
5. On the Overview page, copy:
   - **Application (client) ID** → `TEAMS_APP_ID`
   - **Directory (tenant) ID** → `TEAMS_TENANT_ID`
6. Go to **Certificates & secrets** → **New client secret**
7. Add a description, choose an expiry, click **Add**
8. Copy the **Value** immediately (it won't be shown again) → `TEAMS_APP_PASSWORD`

#### Step 2 — Add Microsoft Graph API permissions

1. In the App Registration, click **API permissions** → **Add a permission**
2. Select **Microsoft Graph** → **Application permissions**
3. Add each of the following:

   | Permission | Purpose |
   |---|---|
   | `Channel.Create` | Create incident channels |
   | `ChannelMessage.Read.All` | Read thread messages for timeline sync |
   | `Chat.ReadWrite.All` | Send direct messages |
   | `Team.ReadBasic.All` | Validate the team exists on startup |
   | `TeamMember.Read.All` | Read team members |

4. Click **Grant admin consent for \<your org\>** — this requires an admin account

#### Step 3 — Create an Azure Bot Service

> **This step is required.** Without it, the Bot Framework relay rejects all proactive message requests regardless of permissions.

1. In Azure Portal → **Create a resource** → search **"Azure Bot"** → **Create**
2. Fill in:
   - **Bot handle**: `openincident-bot` (any name, must be globally unique)
   - **Resource group**: same as your other resources
   - **App ID**: select **Use existing app registration** → enter the App ID from Step 1
   - **App type**: Single tenant
3. Click **Review + create** → **Create**
4. Once deployed, go to the resource → **Channels** → **Microsoft Teams** → **Save**

5. Now add the Bot Framework API permission:
   - Go back to your App Registration → **API permissions** → **Add a permission**
   - Click **APIs my organization uses** tab (not Microsoft APIs)
   - Search: `BotFramework` → select **Microsoft Bot Framework**
   - Choose **Application permissions** → add whatever scope is available
   - Click **Grant admin consent**

#### Step 4 — Find your Team ID

You need the internal ID of the Teams team where incidents will be posted.

**Option A — From the Teams URL:**
1. Open Teams in a browser
2. Navigate to your team → click any channel
3. Copy the URL: it contains `groupId=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
4. That `groupId` value is your `TEAMS_TEAM_ID`

**Option B — From Graph Explorer:**
1. Go to [Graph Explorer](https://developer.microsoft.com/graph/graph-explorer)
2. Sign in and run: `GET https://graph.microsoft.com/v1.0/me/joinedTeams`
3. Find your team and copy its `id` field

#### Step 5 — Generate and install the bot app package

OpenIncident includes a command to generate the Teams sideload package automatically:

```bash
# Ensure TEAMS_APP_ID is in your .env, then:
make teams-app-package
```

This creates `openincident-teams-app.zip` and prints sideload instructions. To install:

1. Open **Microsoft Teams**
2. Navigate to your incident-management team
3. Click **···** next to the team name → **Manage team** → **Apps** tab
4. Click **Upload a custom app** → select `openincident-teams-app.zip`
5. Click **Add** → **Add to a team** → select your team → **Set up a bot**

> If "Upload a custom app" is not visible, ask your Teams admin to enable custom app uploads in the Teams Admin Center under **Teams apps** → **Setup policies**.

#### Step 6 — Configure environment variables

Add to your `.env`:

```env
TEAMS_APP_ID=4fc877c2-...          # Application (client) ID from Step 1
TEAMS_APP_PASSWORD=your-secret     # Client secret value from Step 1
TEAMS_TENANT_ID=007f9c35-...       # Directory (tenant) ID from Step 1
TEAMS_TEAM_ID=339fa65d-...         # Team ID from Step 4
TEAMS_BOT_USER_ID=                 # Optional — service principal object ID, needed for DMs
TEAMS_SERVICE_URL=https://smba.trafficmanager.net/amer/   # Match your region (see below)
```

**Service URL by region** — pick the one closest to your Microsoft 365 tenant:

| Region | URL |
|---|---|
| Americas (default) | `https://smba.trafficmanager.net/amer/` |
| Europe | `https://smba.trafficmanager.net/emea/` |
| India | `https://smba.trafficmanager.net/in/` |
| Asia Pacific | `https://smba.trafficmanager.net/apac/` |

If you are unsure, check your tenant's region in **Azure Active Directory** → **Properties** → **Country or region**.

#### Step 7 — Restart and verify

```bash
docker-compose restart backend
```

Check the logs:

```bash
docker-compose logs backend | grep -i teams
```

You should see:
```
"Teams integration enabled"
```

Create a test incident to confirm the channel appears:

```bash
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{"title":"Teams integration test","severity":"high"}'
```

Within a few seconds a new channel should appear in your Teams team with an Adaptive Card showing the incident status.

#### Bot commands

Once the bot is installed, mention it in any incident channel:

| Command | Action |
|---|---|
| `@OpenIncident ack` | Acknowledge the incident |
| `@OpenIncident resolve` | Resolve the incident |
| `@OpenIncident status` | Show current incident status |
| `@OpenIncident new <title>` | Create a new incident |

#### Teams Features Overview

| Feature | Requires |
|---|---|
| Auto-create incident channel | `TEAMS_APP_ID`, `TEAMS_APP_PASSWORD`, `TEAMS_TENANT_ID`, `TEAMS_TEAM_ID` |
| Post Adaptive Card on creation | All of the above + bot installed in team |
| Update card on status change | All of the above + `teams_conversation_id` stored |
| Bot commands (ack/resolve/status) | Bot installed in team |
| Create incident via bot (`new`) | Bot installed in team |
| Direct message on escalation | `TEAMS_BOT_USER_ID` additionally |
| Archive channel on resolve | All base vars (best-effort rename) |

---

## Troubleshooting

### Backend Won't Start

**Symptom**: `docker-compose logs backend` shows connection errors

**Possible causes**:

1. **Database not ready**
   ```bash
   # Check if PostgreSQL is running
   docker-compose ps db
   # Expected: Status "Up"

   # Check database logs
   docker-compose logs db
   ```
   **Fix**: Wait 10-15 seconds after `docker-compose up -d`, or restart:
   ```bash
   docker-compose restart backend
   ```

2. **Port 8080 already in use**
   ```bash
   # Check what's using port 8080
   lsof -i :8080
   ```
   **Fix**: Either stop the conflicting service or change `PORT` in `.env`:
   ```env
   PORT=8081
   ```
   Then update `docker-compose.yml` ports mapping.

3. **Invalid DATABASE_URL**
   - Verify `.env` has correct database credentials
   - Default: `postgresql://openincident:secret@db:5432/openincident?sslmode=disable`

### Slack Integration Not Working

**Symptom**: Alerts create incidents but no Slack channel appears

**Troubleshooting steps**:

1. **Verify Slack token is set**
   ```bash
   docker-compose exec backend env | grep SLACK_BOT_TOKEN
   ```
   Should show `SLACK_BOT_TOKEN=xoxb-...`

2. **Check backend logs**
   ```bash
   docker-compose logs backend | grep -i slack
   ```
   Look for:
   - ✅ `"slack service initialized"` — integration working
   - ❌ `"slack auth failed"` — invalid token
   - ❌ `"failed to create channel"` — missing scopes

3. **Verify OAuth scopes**
   - Go to https://api.slack.com/apps → Your App → OAuth & Permissions
   - Confirm all required scopes are added (see [Slack App Setup](#slack-app-setup))
   - If you added scopes after installation, **reinstall the app**:
     - Click "Reinstall to Workspace"
     - Update `SLACK_BOT_TOKEN` in `.env` (it will change)
     - Restart backend: `docker-compose restart backend`

4. **Bot not invited to create channels**
   - The bot should NOT need to be manually invited
   - If channels aren't being created, check for `missing_scope` errors in logs

### /ready Returns 503

**Symptom**: `curl http://localhost:8080/ready` returns HTTP 503

**Possible causes**:

1. **Redis not running**
   ```bash
   docker-compose ps redis
   # Expected: Status "Up"
   ```
   **Fix**: Start Redis:
   ```bash
   docker-compose up -d redis
   docker-compose restart backend
   ```

2. **Database connection pool exhausted**
   - Check database connections in metrics:
     ```bash
     curl http://localhost:8080/metrics | grep db_connections
     ```
   - Increase pool size in `.env`:
     ```env
     DB_MAX_OPEN_CONNS=50
     DB_MAX_IDLE_CONNS=10
     ```

### Incidents Not Created from Alerts

**Symptom**: Webhook returns 200 OK but no incident appears

**Debug steps**:

1. **Check alert severity**
   - By default, only `critical` and `warning` alerts create incidents
   - `info` level alerts are stored but don't auto-create incidents

2. **Check backend logs**
   ```bash
   docker-compose logs backend | tail -50
   ```
   Look for error messages during incident creation

3. **Verify alert deduplication**
   - If an alert with the same external ID already exists, it won't create a new incident
   - Check existing alerts:
     ```bash
     curl http://localhost:8080/api/v1/alerts
     ```

4. **Test with minimal payload**
   ```bash
   curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
     -H "Content-Type: application/json" \
     -d '{
       "receiver": "test",
       "status": "firing",
       "alerts": [{
         "status": "firing",
         "labels": {"alertname": "TestAlert", "severity": "critical"},
         "annotations": {"summary": "Test alert"},
         "startsAt": "2024-01-01T00:00:00Z"
       }]
     }'
   ```

### Microsoft Teams Integration Not Working

**Symptom**: Incidents are created but no Teams channel appears, or channel appears but no message is posted.

**Step 1 — Check the logs:**

```bash
docker-compose logs backend | grep -i teams
```

| Log message | Meaning |
|---|---|
| `"Teams integration enabled"` | Credentials validated, Graph API working |
| `"creating teams channel for incident"` | Channel creation attempt started |
| `"teams channel created for incident"` | Graph API succeeded — channel exists |
| `"failed to post initial teams card"` | Bot Framework step failed (see below) |
| `credential validation failed` | Wrong `TEAMS_APP_ID` / `TEAMS_APP_PASSWORD` / `TEAMS_TENANT_ID` |

**Step 2 — Channel created but no message posted (most common)**

The channel appears in Teams but no Adaptive Card is posted. Check for the error in logs:

- `401 Authorization has been denied` — Bot Framework API permission not granted.
  Go to Azure Portal → App Registration → **API permissions** → **APIs my organization uses**
  → search `BotFramework` → add permission → **Grant admin consent**.

- `400 BadSyntax: Incorrect conversation creation parameters` — The bot is not installed in the team.
  Run `make teams-app-package` and follow the sideload instructions.

- `400 BadSyntax: Activity must include non-empty text` — Outdated version of the service code.
  Pull the latest version and restart.

**Step 3 — Wrong region (silent failures)**

If your Microsoft 365 tenant is outside the Americas, the default service URL will fail with a 400.
Set the correct region in `.env`:

```env
# Europe
TEAMS_SERVICE_URL=https://smba.trafficmanager.net/emea/

# India
TEAMS_SERVICE_URL=https://smba.trafficmanager.net/in/

# Asia Pacific
TEAMS_SERVICE_URL=https://smba.trafficmanager.net/apac/
```

**Step 4 — Bot commands not working**

If `@OpenIncident ack` returns "No incident found for this channel":
- Verify the bot is installed in the team (Step 5 of setup)
- Verify `TEAMS_SERVICE_URL` matches your tenant region
- Check that `teams_conversation_id` was saved: create a fresh incident and check logs for "teams channel created"

**Step 5 — "Upload a custom app" not visible in Teams**

Custom app sideloading is controlled by Teams policy. Ask a Teams admin to enable it:
Teams Admin Center → **Teams apps** → **Setup policies** → allow custom app uploads.

Alternatively, the admin can upload the package centrally via **Manage apps** and assign it to the team.

### Frontend Shows Empty State

**Symptom**: UI loads but shows "No incidents" even though backend has data

**Possible causes**:

1. **Frontend can't reach backend**
   - Check browser console (F12) for CORS errors
   - Verify frontend is configured with correct API URL
   - Default: `VITE_API_URL=http://localhost:8080` in `frontend/.env`

2. **Backend not returning data**
   - Test API directly:
     ```bash
     curl http://localhost:8080/api/v1/incidents
     ```
   - If empty, no incidents exist yet

### Need More Help?

- **Logs**: Always check `docker-compose logs backend` for detailed error messages
- **Metrics**: Check http://localhost:8080/metrics for system health indicators
- **GitHub Issues**: https://github.com/yourusername/openincident/issues
- **Community**: Join our GitHub Discussions for support

---

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) first.

### Development Setup

```bash
# Clone repo
git clone https://github.com/yourusername/openincident.git
cd openincident

# Start dependencies
docker-compose up -d db redis

# Run backend
cd backend && go run ./cmd/openincident

# Run frontend (separate terminal)
cd frontend && npm install && npm run dev
```

### Running Tests

```bash
make test
```

---

## Comparison

| Feature | OpenIncident | incident.io | PagerDuty |
|---------|--------------|-------------|-----------|
| Alert management | ✅ | ❌ | ✅ |
| Incident coordination | ✅ | ✅ | ⚠️ |
| On-call scheduling | ✅ | ❌ | ✅ |
| Self-hosted | ✅ | ❌ | ❌ |
| Open source | ✅ | ❌ | ❌ |
| BYO AI/LLM | ✅ | ❌ | ❌ |
| Pricing | Free / Flat | Per-seat | Per-seat |

---

## License

- **Core**: [AGPLv3](LICENSE)
- **Enterprise**: Proprietary (contact us)

---

## Support

- **Community**: [GitHub Discussions](https://github.com/yourusername/openincident/discussions)
- **Issues**: [GitHub Issues](https://github.com/yourusername/openincident/issues)
- **Enterprise**: enterprise@openincident.io

---

Built with ❤️ for teams who believe incident data belongs to them.