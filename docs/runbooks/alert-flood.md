# Runbook: Alert flood

**Symptom:** Dozens or hundreds of incidents are created in minutes. Slack is flooded with new channels. On-call engineers are paged repeatedly.

---

## Diagnose

**1. Identify the source:**

```bash
# Docker Compose
docker logs fluidify-regen --since 15m 2>&1 | grep "alert received\|incident created" | head -50

# Kubernetes
kubectl logs -n fluidify deploy/fluidify-regen --since=15m | grep "alert received\|incident created" | head -50
```

**2. Check the alerts list in the UI:**

Go to **Alerts** — sort by `received_at` descending. Look for:
- All alerts from the same source (Prometheus, Grafana, etc.)
- All alerts with the same `alertname` label
- All alerts from the same service

**3. Check if deduplication is working:**

If the same alert is firing repeatedly (e.g., a flapping alert), deduplication should prevent duplicate incidents. If it's not:
- Check that the alert source is sending a consistent `fingerprint` or `external_id`
- Generic webhook alerts require an `external_id` field for deduplication; without it, every POST creates a new alert

---

## Mitigate

**Immediate: suppress the source at the alerting level**

The fastest mitigation is silencing at the source, not in Regen:

- **Prometheus Alertmanager:** Create a silence via the Alertmanager UI or API
  ```bash
  amtool silence add alertname=<AlertName> --duration=2h --comment="Flood mitigation"
  ```
- **Grafana:** Pause the alert rule in the Grafana UI
- **CloudWatch:** Disable the alarm in the AWS console

**Suppress in Regen via a routing rule:**

1. Go to **Settings → Routing Rules**
2. Add a high-priority rule (low priority number):
   - `match_criteria`: target the noisy alert by label or source
   - `actions`: `{"suppress": true}`
3. This stops new incidents being created; existing ones are unaffected

**Resolve the flood of open incidents:**

If hundreds of incidents were created, bulk-resolve them via the API:

```bash
# Get all triggered incidents
curl -s "https://your-regen-host/api/v1/incidents?status=triggered&limit=500" \
  -H "Authorization: Bearer <token>" | jq -r '.incidents[].id' | while read id; do
  curl -s -X PATCH "https://your-regen-host/api/v1/incidents/$id" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer <token>" \
    -d '{"status":"resolved"}'
done
```

---

## Resolve

1. Remove or adjust the suppression routing rule once the source is fixed
2. Archive or clean up noise Slack channels (Regen prefixes resolved channels with `[RESOLVED]`)
3. Review deduplication and grouping rules to prevent recurrence — see [deduplication docs](../alerts/deduplication.md)

---

## Prevention

- **Grouping rules:** Group alerts from the same service into one incident within a time window
- **Routing rules:** Suppress low-severity or known-noisy alerts by default; only page on `critical` and `warning`
- **Alert source hygiene:** Fix flapping alerts at the source — alerts that resolve and re-fire within seconds indicate a threshold that needs tuning
- **Rate limit webhook callers:** Regen's API has rate limiting enabled (`RATE_LIMIT_REQUESTS_PER_MINUTE`); tune it to absorb bursts without creating incidents for every single request
