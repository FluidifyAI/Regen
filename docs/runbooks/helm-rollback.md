# Runbook: Helm rollback

**Symptom:** A Helm upgrade introduced a regression — the app is returning errors, the UI is broken, or the migration failed. You need to revert to the previous working version quickly.

---

## Diagnose

**1. Check the current release status:**

```bash
helm status fluidify-regen -n fluidify
```

**2. Check the revision history:**

```bash
helm history fluidify-regen -n fluidify
```

Example output:

```
REVISION  UPDATED        STATUS      CHART                  APP VERSION  DESCRIPTION
1         Jun 01 10:00   superseded  fluidify-regen-0.11.0  0.11.0       Install complete
2         Jul 12 14:00   deployed    fluidify-regen-1.0.0   1.0.0        Upgrade complete
```

**3. Identify the last good revision:**

Look for the most recent revision with `STATUS = superseded` — that was the last working state.

**4. Check pod logs for the reason:**

```bash
kubectl logs -n fluidify deploy/fluidify-regen --since=15m
```

---

## Rollback

**Roll back to the previous revision:**

```bash
helm rollback fluidify-regen -n fluidify
```

This rolls back to the immediately preceding revision. To target a specific revision:

```bash
helm rollback fluidify-regen 1 -n fluidify
```

Where `1` is the revision number from `helm history`.

**Verify the rollback:**

```bash
helm status fluidify-regen -n fluidify
kubectl rollout status deploy/fluidify-regen -n fluidify
curl -s https://your-regen-host/ready | jq .
```

---

## Database migrations after rollback

If the failed upgrade ran database migrations before crashing, the schema may now be ahead of the rolled-back binary. This is the most dangerous case.

**Check the current migration version:**

```bash
kubectl exec -n fluidify deploy/fluidify-regen -- /app/regen migrate version
```

Or directly in PostgreSQL:

```bash
kubectl exec -n fluidify deploy/postgres -- psql -U regen -c \
  "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 5;"
```

**If the schema is ahead of the rolled-back binary:**

The rolled-back binary will refuse to start if it detects an unknown migration version. Options:

1. **Preferred:** Roll forward — fix the issue in a new version and deploy forward. Do not roll back migrations in production unless absolutely necessary.
2. **Last resort — down migration:** Run the down migration for the version that was applied:
   ```bash
   kubectl exec -n fluidify deploy/fluidify-regen -- /app/regen migrate down 1
   ```
   This reverts the last migration. Only do this if you have a recent database backup and have tested the down migration.

**Always take a database backup before rolling back migrations:**

```bash
kubectl exec -n fluidify deploy/postgres -- \
  pg_dump -U regen regen > regen-backup-before-rollback-$(date +%Y%m%d-%H%M).sql
```

---

## Resolve

1. Confirm `/ready` returns all-green
2. Smoke test critical paths: incidents list, alert ingestion, Slack channel creation
3. Open an incident report documenting what went wrong in the failed upgrade
4. Fix the root cause before attempting the upgrade again

---

## Prevention

- Always take a database backup before a Helm upgrade: `pg_dump` before `helm upgrade`
- Use `helm upgrade --dry-run` to render templates and catch obvious errors before applying
- Deploy to a staging environment first and run smoke tests there
- Keep `helm history` — Helm retains 10 revisions by default; rollback is always one command away
