# Failure Runbooks

Step-by-step guides for the most common Regen failure modes. Each runbook covers diagnosis, immediate mitigation, and root cause resolution.

| # | Runbook | Symptom |
|---|---------|---------|
| 1 | [Slack webhook down](./slack-webhook-down.md) | Incidents created but no Slack channels appear |
| 2 | [Database connection lost](./database-connection-lost.md) | API returns 500, `/ready` shows `"database":"error"` |
| 3 | [Alert flood](./alert-flood.md) | Hundreds of incidents created in minutes |
| 4 | [Missed escalation](./missed-escalation.md) | Alert fired, incident created, on-call not paged |
| 5 | [Redis unavailable](./redis-unavailable.md) | Background jobs stall, `/ready` shows `"redis":"error"` |
| 6 | [SAML misconfigured](./saml-misconfigured.md) | SSO login redirects to error or loops |
| 7 | [Schedule with no members](./schedule-no-members.md) | On-call query returns empty, escalation has nobody to page |
| 8 | [Helm rollback](./helm-rollback.md) | Bad deploy — need to revert to the previous version |
| 9 | [Migration failed](./migration-failed.md) | App won't start, logs show migration error |
| 10 | [High memory / OOM](./high-memory-oom.md) | Container OOMKilled, pod restart loop |
