# Environment Variables

All configuration is done through environment variables. Set them in a `.env` file in the project root or pass them directly to Docker.

## Core

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `PORT` | `8080` | No | Port the application listens on |
| `APP_ENV` | `development` | No | Set to `production` for production deployments |
| `LOG_LEVEL` | `info` | No | Log verbosity: `debug`, `info`, `warn`, `error` |

## Database

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `DATABASE_URL` | `postgresql://regen:secret@localhost:5432/regen?sslmode=disable` | Yes (production) | PostgreSQL connection string |
| `DB_MAX_OPEN_CONNS` | `25` | No | Maximum open database connections |
| `DB_MAX_IDLE_CONNS` | `5` | No | Maximum idle database connections |
| `DB_CONN_MAX_LIFE` | `5m` | No | Maximum connection lifetime |

Migrations run automatically on startup. No manual migration step required.

## Redis

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `REDIS_URL` | `redis://localhost:6379` | No | Redis connection URL (ignored when Sentinel is configured) |
| `REDIS_PASSWORD` | — | No | Redis password (used in both single and Sentinel mode) |
| `REDIS_SENTINEL_ADDRS` | — | No | Comma-separated Sentinel addresses for HA: `sentinel1:26379,sentinel2:26379` |
| `REDIS_SENTINEL_MASTER` | `mymaster` | No | Sentinel master name (only used when `REDIS_SENTINEL_ADDRS` is set) |

## Slack

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SLACK_BOT_TOKEN` | — | Yes (Slack) | Bot token starting with `xoxb-` |
| `SLACK_SIGNING_SECRET` | — | Yes (Slack) | Signing secret from your Slack app |
| `SLACK_APP_TOKEN` | — | Yes (Slack) | App-level token starting with `xapp-` — required for interactive buttons (Make me Lead, Add Note) |

See [Connecting Slack](../getting-started/connecting-slack.md) for how to obtain these values.

## Microsoft Teams

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `TEAMS_APP_ID` | — | Yes (Teams) | Azure App Registration Application ID |
| `TEAMS_APP_PASSWORD` | — | Yes (Teams) | Azure App Registration client secret |
| `TEAMS_TENANT_ID` | — | Yes (Teams) | Azure AD Tenant ID |
| `TEAMS_TEAM_ID` | — | Yes (Teams) | ID of the Team where incident channels are created |
| `TEAMS_BOT_USER_ID` | — | Yes (Teams) | AAD object ID of the bot user (required for DMs) |
| `TEAMS_SERVICE_URL` | `https://smba.trafficmanager.net/amer/` | No | Bot Framework relay URL — change for non-US tenants: `/emea/`, `/apac/`, `/in/` |

Teams integration is disabled when `TEAMS_APP_ID` is not set.

## AI (OpenAI)

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `OPENAI_API_KEY` | — | No | Your OpenAI API key — AI features are disabled when not set |
| `OPENAI_MODEL` | `gpt-4o-mini` | No | Model to use for summaries and post-mortem generation |
| `OPENAI_MAX_TOKENS` | `1000` | No | Token limit for summaries |
| `OPENAI_POSTMORTEM_MAX_TOKENS` | `3000` | No | Token limit for post-mortem generation |

The API key can also be set through **Settings → System** in the UI without a container restart.

## SAML SSO

SAML SSO is free and included in the Community edition.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SAML_IDP_METADATA_URL` | — | Yes (SSO) | Your IdP metadata URL — SSO is disabled when not set |
| `SAML_BASE_URL` | `http://localhost:8080` | Yes (SSO) | Externally reachable base URL of this instance (e.g. `https://incidents.yourcompany.com`) |
| `SAML_ENTITY_ID` | — | No | SP EntityID — defaults to `<SAML_BASE_URL>/saml/metadata` |
| `SAML_CERT_FILE` | — | No | Path to SP certificate PEM — auto-generated self-signed cert used if not set |
| `SAML_KEY_FILE` | — | No | Path to SP private key PEM — auto-generated if not set |
| `SAML_ALLOW_IDP_INITIATED` | `false` | No | Allow IdP-initiated flows (e.g. Okta tile clicks) |

See [SAML SSO](./saml-sso.md) for setup guides for Okta, Azure AD, and Google Workspace.

## CORS

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `CORS_ALLOWED_ORIGINS` | — | No | Comma-separated list of allowed origins for API requests (e.g. `https://incidents.yourcompany.com`). Leave empty when the frontend and API are on the same origin (default with `make start`). |

## Example `.env` file

```env
# Required
DATABASE_URL=postgresql://regen:strongpassword@db:5432/regen?sslmode=disable
REDIS_URL=redis://redis:6379

# Slack
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...
SLACK_APP_TOKEN=xapp-...

# AI (optional)
OPENAI_API_KEY=sk-...

# Production
APP_ENV=production
PORT=8080
```
