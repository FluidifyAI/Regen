# Runbook: Schedule with no members

**Symptom:** The on-call query for a schedule returns an empty result. Escalation policies that reference the schedule have nobody to page. The UI shows "Nobody on call" for the schedule.

---

## Diagnose

**1. Check who is currently on call:**

```bash
curl -s "https://your-regen-host/api/v1/schedules/<schedule-id>/oncall" \
  -H "Authorization: Bearer <token>" | jq .
```

An empty `on_call` array means the schedule has no coverage at this time.

**2. Check the schedule configuration:**

Go to **On-Call → Schedules → [schedule name]**:
- Are there any rotation layers? An empty schedule has no layers.
- Do the layers have participants? A layer with no users produces no on-call.
- Is the rotation start date in the future? A layer that hasn't started yet produces no coverage.
- Is there a gap in the rotation (e.g., the schedule only runs Mon–Fri and it's Saturday)?

**3. Check for overrides:**

An override that removes the on-call person without a replacement creates a gap. Go to **Schedule → Overrides** and check the current time window.

**4. Check user accounts:**

If a participant's Regen account was deleted after being added to the schedule, the slot is empty. The schedule calendar will show the deleted user's name but they can no longer be paged.

---

## Mitigate

**Add a temporary override:**

1. Go to **On-Call → Schedules → [schedule name] → Overrides**
2. Create an override for the current gap, assigning an available engineer
3. This takes effect immediately — the next escalation check will find the override

**Add a user to the rotation layer:**

1. Go to the schedule → edit the rotation layer
2. Add the missing participant
3. The rotation recalculates immediately for future shifts

**Add a fallback tier to the escalation policy:**

If the schedule will have gaps regularly (e.g., weekends only covered by a secondary team):

1. Go to **On-Call → Escalation Policies → [policy name]**
2. Add a final tier with a named individual (not a schedule) as a backstop
3. This tier fires if all earlier tiers produce no on-call result

---

## Resolve

1. Verify `GET /api/v1/schedules/<id>/oncall` returns a non-empty result
2. Create a test incident and confirm the escalation policy pages correctly
3. Review the schedule for the next 30 days to identify any other coverage gaps

---

## Prevention

- Every schedule should have at least two participants in each rotation layer — a one-person rotation creates a single point of failure
- Every escalation policy should have a final tier with a named individual (not just a schedule) as a fallback
- Review schedule coverage monthly — look especially at holiday periods and timezone gaps
- Set up a `/api/v1/schedules/<id>/oncall` monitor that alerts if the result is empty for more than 5 minutes
