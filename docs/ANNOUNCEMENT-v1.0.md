# Fluidify Regen v1.0.0 — Announcement Draft

> 300–400 word post for HackerNews / Reddit (r/devops, r/sysadmin).
> Hand off to GTM doc before publishing — do not post directly from this file.

---

## HackerNews / Reddit

**Title:** Show HN: Fluidify Regen v1.0 — open-source PagerDuty/incident.io alternative, self-hosted, free forever

---

We're releasing v1.0 of Fluidify Regen today — an open-source incident management and on-call platform you can run on a $20/month VPS.

**Why this exists:**

PagerDuty and incident.io charge $30–50/user/month. A 200-person engineering team pays ~$120k/year to be told who's on call. Grafana OnCall was the self-hosted alternative until Grafana archived it in March 2026, leaving ~50,000 users without a migration path.

Regen is the replacement: full feature parity, one Docker image, your data stays on your servers.

**What it does:**

- Alert ingestion from Prometheus, Grafana, CloudWatch, and any webhook source
- On-call rotations with public holidays, individual leave, schedule overrides
- Multi-step escalation policies with configurable timeouts
- Slack and Microsoft Teams integration — channels auto-created, bot commands, timeline sync
- AI-generated incident summaries and post-mortems (BYO key — OpenAI, Anthropic, or self-hosted Ollama)
- 1-click migration from PagerDuty (including EU), Opsgenie, and Grafana OnCall
- SSO/SAML — free, not gated behind an enterprise tier

**SSO is free.** We've seen too many tools charge $X/month just to let you log in with your IdP. That's not a power feature, it's a security requirement. We stay off [sso.tax](https://sso.tax).

**What's in v1.0:**

The biggest additions since v0.11 are RE2 regex in alert routing rules, multi-provider AI (OpenAI/Anthropic/Ollama in one UI), incident file attachments, a public status page, and a full set of production runbooks. Full changelog: [CHANGELOG.md](https://github.com/FluidifyAI/Regen/blob/main/CHANGELOG.md)

**Try it:**

```bash
git clone https://github.com/FluidifyAI/Regen.git && cd Regen
cp .env.example .env && make start
```

Opens at `http://localhost:8080`. Helm chart available for Kubernetes. Single binary with embedded frontend — no separate asset server.

**What's next:**

v1.1 adds an MCP server so external agents (Claude, GPT, custom bots) can call Regen to query incidents, triage context, and on-call state over the Model Context Protocol. The long-term goal is making AI agents and humans first-class actors on the same operational layer — not AI bolted on top of a human-centric tool.

GitHub: https://github.com/FluidifyAI/Regen  
Discord: https://discord.gg/b6PSdhzDa

---

*Self-host it. Keep your data. Never pay per seat.*
