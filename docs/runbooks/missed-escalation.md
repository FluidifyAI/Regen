# Runbook: Missed escalation

**Symptom:** An alert fired and an incident was created, but the on-call engineer was not paged. Or the first tier was paged but escalation to the next tier did not fire after the timeout.

---

## Diagnose

**1. Check the incident timeline:**

Open the incident in the UI → **Timeline** tab. Look for:
- `escalation_triggered` entries — confirms the escalation policy fired
- `notification_sent` entries — confirms the page was attempted
- Any error entries related to notifications

**2. Check app logs for escalation worker activity:**

```bash
# Docker Compose
docker logs fluidify-regen --since 1h 2>&1 | grep -i "escalat\|notify\|pag"

# Kubernetes
kubectl logs -n fluidify deploy/fluidify-regen --since=1h | grep -i "escalat\|notify\|pag"
```

**3. Check whether an escalation policy is attached to the incident:**

In the UI: incident detail → **Properties** panel → **Escalation Policy**. If blank, no policy was attached when the incident was created.

**4. Check the escalation policy configuration:**

Go to **On-Call → Escalation Policies** and open the policy. Verify:
- At least one tier has users or a schedule assigned
- Tier timeouts are set (e.g., 5 minutes before escalating to the next tier)
- The schedule referenced in the policy has active members at the time the incident fired

**5. Check Redis (escalation timers run via Redis):**

```bash
curl -s https://your-regen-host/ready | jq .redis
```

If Redis is down, escalation timers cannot fire. See [Redis unavailable runbook](./redis-unavailable.md).

---

## Common causes

| Cause | Check |
|-------|-------|
| No escalation policy on the incident | Incident properties panel — policy field is blank |
| Schedule has no on-call member | On-Call → Schedules → who's on call right now? |
| Redis is down | `/ready` endpoint → `"redis":"error"` |
| Escalation policy has no notification channels configured | Policy tiers show users but Slack/email is not set up |
| Incident was acknowledged before escalation timer fired | Timeline shows `acknowledged` before `escalation_triggered` |
| Alert routing rule created incident without attaching a policy | Check routing rule actions for `escalation_policy_id` |

---

## Mitigate

**Page the on-call engineer manually** while investigating root cause — don't wait for the system to recover first.

**Manually trigger escalation:**

If the incident exists but nobody was paged, manually acknowledge and then page via Slack if that integration is working:

```
/regen escalate INC-123
```

Or add a timeline note tagging the on-call engineer directly.

**Attach a policy to the incident:**

From the incident detail → Properties → Escalation Policy → assign the correct policy. This starts the escalation timer from now.

---

## Resolve

1. Fix the root cause (schedule gap, Redis, policy config)
2. Verify by creating a test incident and confirming the escalation fires at the expected timeout
3. Check the schedule for the next 7 days to ensure there are no other gaps where nobody is on call

---

## Prevention

- Every escalation policy tier should have a **fallback schedule** or a named individual as the final tier with no timeout (always pages)
- Run a monthly "schedule audit" — go to **On-Call → Schedules** and verify every schedule has coverage for the next 30 days
- Monitor Redis availability — escalation timers depend on it
- Set a routing rule default action that attaches your primary escalation policy to all new incidents
