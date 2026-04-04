# Fluidify Regen Documentation

Open-source incident management — self-hosted, agent-native, free forever.

## Getting Started

| | |
|--|--|
| [Installation](./getting-started/installation.md) | Run Regen with a single command |
| [Connecting Slack](./getting-started/connecting-slack.md) | Bot token, signing secret, interactive buttons |
| [Connecting Teams](./getting-started/connecting-teams.md) | Azure App Registration, Bot Service, sideloading |
| [Connecting Telegram](./getting-started/connecting-telegram.md) | Notification-only — bot setup, chat ID, limitations |

## Alert Sources

Connect your monitoring tools to start receiving alerts.

| | |
|--|--|
| [Prometheus / Alertmanager](./alerts/sources/prometheus.md) | Webhook URL and alertmanager.yml config |
| [Grafana](./alerts/sources/grafana.md) | Unified Alerting contact point setup |
| [AWS CloudWatch](./alerts/sources/cloudwatch.md) | SNS subscription setup |
| [Generic Webhook](./alerts/sources/generic.md) | For any other tool or custom script |
| [Deduplication & Grouping](./alerts/deduplication.md) | Prevent alert storms from flooding incidents |

## Incidents

| | |
|--|--|
| [Incident Lifecycle](./incidents/lifecycle.md) | Triggered → Acknowledged → Resolved |
| [Post-Mortems](./incidents/post-mortems.md) | AI-generated drafts, templates, action items |

## On-Call

| | |
|--|--|
| [Schedules](./on-call/schedules.md) | Rotations, layers, overrides |
| [Escalation Policies](./on-call/escalation-paths.md) | Auto-escalate when no one responds |
| [Being On-Call](./on-call/being-on-call.md) | How to respond, acknowledge, and hand off |

## Self-Hosting

| | |
|--|--|
| [Docker Compose](./self-hosting/docker-compose.md) | Production deployment, backups, reverse proxy |
| [Kubernetes](./self-hosting/kubernetes.md) | Helm chart, HA, secrets |
| [Environment Variables](./self-hosting/environment-variables.md) | Full reference for every config option |
| [SAML SSO](./self-hosting/saml-sso.md) | Okta, Azure AD, Google Workspace |

---

## Need help?

- [GitHub Issues](https://github.com/fluidifyai/regen/issues) — bug reports and feature requests
- [GitHub Discussions](https://github.com/fluidifyai/regen/discussions) — questions and community
