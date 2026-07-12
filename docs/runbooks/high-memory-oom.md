# Runbook: High memory / OOM

**Symptom:** The Regen container is OOMKilled and restarting repeatedly. Memory usage climbs steadily over hours. Kubernetes shows `OOMKilled` in `kubectl describe pod`.

---

## Diagnose

**1. Check if the container was OOMKilled:**

```bash
kubectl describe pod -n fluidify -l app=fluidify-regen | grep -A5 "Last State\|OOMKilled\|Reason"
```

**2. Check current memory usage:**

```bash
kubectl top pods -n fluidify
```

**3. Check memory limits in the Helm values:**

```bash
helm get values fluidify-regen -n fluidify | grep -A5 resources
```

Default limits: `memory: 512Mi`. If the app is growing beyond this, either the limit is too low or there is a memory leak.

**4. Check for alert flood or large result sets:**

High memory is often caused by:
- Processing a very large batch of alerts (alert flood — see [alert-flood runbook](./alert-flood.md))
- A query returning a very large result set (check for unusual traffic on the API)
- An AI generation job holding a large prompt in memory

Check logs for the time memory started climbing:

```bash
kubectl logs -n fluidify deploy/fluidify-regen --since=1h | grep -i "error\|panic\|oom\|large\|limit"
```

**5. Check PostgreSQL connection pool:**

Each idle PostgreSQL connection holds ~5–10 MB in the Go runtime. If `DB_MAX_OPEN_CONNS` is too high relative to available memory, connections alone can exhaust the limit.

---

## Mitigate

**Immediate: increase memory limit if you're under-provisioned:**

```bash
helm upgrade fluidify-regen ./deploy/helm/fluidify-regen -n fluidify \
  --reuse-values \
  --set resources.limits.memory=1Gi \
  --set resources.requests.memory=256Mi
```

**Restart the pod to recover from OOMKill:**

Kubernetes restarts OOMKilled pods automatically, but if the restart loop is causing service disruption:

```bash
kubectl rollout restart deploy/fluidify-regen -n fluidify
```

**Reduce connection pool size:**

```env
DB_MAX_OPEN_CONNS=10
DB_MAX_IDLE_CONNS=5
```

Restart the app after changing.

---

## Sizing reference

| Deployment scale | Recommended memory limit |
|---|---|
| <10 incidents/day | 256 Mi |
| 10–100 incidents/day | 512 Mi (default) |
| 100–1000 incidents/day | 1 Gi |
| 1000+ incidents/day | 2 Gi; consider horizontal scaling |

The Go runtime's GC typically keeps live heap at ~2× the working set. If you see steady growth without levelling off, that indicates a leak, not just working set growth.

---

## Resolve

1. Confirm pods are stable with `kubectl get pods -n fluidify -w`
2. Check `/ready` returns all-green
3. Monitor memory usage with `kubectl top pods -n fluidify` over the next 30 minutes to confirm it's stable

---

## Prevention

- Set memory **requests** and **limits** in the Helm values — without limits, the container can consume all node memory and impact other workloads
- Enable Kubernetes HPA (`autoscaling.enabled: true` in values) so the app scales out under load instead of running out of memory on a single pod
- Keep `DB_MAX_OPEN_CONNS` at ≤25 (default) — each connection holds memory in both the app and PostgreSQL
- Alert on `container_memory_usage_bytes` approaching 80% of the limit — this gives time to act before OOMKill
