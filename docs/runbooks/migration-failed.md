# Runbook: Migration failed

**Symptom:** The app refuses to start. Logs show a database migration error. The container exits immediately or enters a crash loop.

---

## Diagnose

**1. Check the logs for the error:**

```bash
# Docker Compose
docker logs fluidify-regen 2>&1 | grep -i "migrat\|error\|fail"

# Kubernetes
kubectl logs -n fluidify deploy/fluidify-regen | grep -i "migrat\|error\|fail"
# Or check the migration Job (if using Helm):
kubectl logs -n fluidify job/fluidify-regen-migrate
```

Common patterns:
- `migration failed: pq: column "X" of relation "Y" already exists` — migration was partially applied (crashed midway)
- `migration failed: pq: relation "X" does not exist` — earlier migration was skipped or rolled back
- `migration failed: dirty database version N; fix and force version` — the migration table is marked dirty from a previous crash
- `permission denied for schema public` — database user lacks DDL privileges

**2. Check the current migration state:**

```bash
# Docker Compose
docker exec fluidify-regen-db psql -U regen -c \
  "SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# Kubernetes
kubectl exec -n fluidify deploy/postgres -- psql -U regen -c \
  "SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 5;"
```

A `dirty = true` row means the migration at that version was interrupted.

---

## Fix: dirty migration state

This is the most common failure. The migration tool marks the version as dirty when a migration crashes partway through.

**Step 1 — Take a database backup first:**

```bash
# Docker Compose
docker exec fluidify-regen-db pg_dump -U regen regen > regen-backup-$(date +%Y%m%d-%H%M).sql

# Kubernetes
kubectl exec -n fluidify deploy/postgres -- pg_dump -U regen regen > regen-backup-$(date +%Y%m%d-%H%M).sql
```

**Step 2 — Check what the failed migration was trying to do:**

```bash
ls backend/migrations/ | grep <version>
```

Review the `.up.sql` file for that version to understand what state the database may be in.

**Step 3 — Force the migration version:**

If the migration was partially applied and the schema is now in an unknown state, you may need to manually clean up the partial change and then force the version:

```bash
# Run the app with the force flag (resets dirty state to the version below the failed one)
docker run --rm --env-file .env ghcr.io/fluidifyai/regen:latest /app/regen migrate force <version-1>
```

Where `<version-1>` is the version number *before* the failed migration (i.e., the last known-good migration).

Then restart the app normally — it will re-apply the failed migration from scratch.

**Step 4 — If the partial migration left schema artifacts:**

Connect directly to PostgreSQL and manually clean up. For example, if a migration to add a column failed halfway:

```sql
-- Check if the column exists
SELECT column_name FROM information_schema.columns
WHERE table_name='incidents' AND column_name='new_column';

-- If it does and shouldn't yet, drop it
ALTER TABLE incidents DROP COLUMN IF EXISTS new_column;
```

Then re-run `migrate force` and restart.

---

## Fix: permission denied

The database user needs DDL privileges to run migrations. Grant them:

```sql
GRANT ALL PRIVILEGES ON SCHEMA public TO regen;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO regen;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO regen;
```

---

## Fix: migration already applied (column exists)

If a migration tries to add a column that already exists, it's because a previous partial run added it before crashing. The fix is the same as the dirty state fix above — force the version back and let it retry cleanly.

---

## Resolve

1. Confirm the app starts successfully and logs show `running database migrations... done` or `no migrations to run`
2. Check `GET /ready` returns `"database":"ok"`
3. Do a quick smoke test: open the UI, check the incidents list

---

## Prevention

- Always take a database backup before deploying a new version (`pg_dump` before `docker pull` / `helm upgrade`)
- Test migrations on a staging database with a copy of production data before deploying to production
- Regen migrations are written to be idempotent where possible — but complex schema changes (type changes, constraint additions) are not always safely re-runnable
- In Kubernetes, the Helm chart runs migrations as a Job that must complete before the Deployment starts — this ensures the app never runs against an un-migrated schema
