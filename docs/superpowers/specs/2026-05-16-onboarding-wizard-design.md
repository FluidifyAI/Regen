# Design Spec — Onboarding Wizard (OPE-108)

**Date:** 2026-05-16  
**Linear:** https://linear.app/openincident/issue/OPE-108  
**Status:** Approved

---

## Section 1 — What we're building

A full-page `/setup` route that auto-redirects new users on first login. Guides through 5 sequential steps with a progress bar. Every step is skippable (except schedule if no schedule exists). Once all steps are completed or skipped the wizard marks itself dismissed and never shows again.

**The 5 steps:**

| Step | What happens |
|---|---|
| 1 — Connect Slack | Inline version of SlackSetupModal flow (bot token setup, not login OAuth) |
| 2 — Set up schedule | Grafana OnCall import (working) / PD / OG (Coming Soon tiles) / Create manually. Required if no schedule exists; skippable if one already does. |
| 3 — Invite teammates | Simplified InviteModal — email + name, auto-generated password, shows copyable setup link. Skippable. |
| 4 — Send a test alert | Generic webhook URL + ready-to-paste curl. Polls for incoming alert, auto-completes on detection. Skippable. |
| 5 — Done | Celebration screen. Links to incident, schedule, docs. "Go to dashboard" closes wizard permanently. |

**What it is NOT:**
- Not a modal — dedicated full page at `/setup`
- Not a hard blocker — every step has Skip (except schedule when none exists)
- Not shown again after completion or full skip
- Not dependent on PD/Opsgenie imports (Coming Soon tiles only)

**Success criteria:**
- Fresh-install user lands on `/setup` immediately after first login
- All 5 steps completable in under 15 minutes
- Completing or skipping all steps → redirect to `/`, wizard never re-triggers
- Slack step failure shows a human-readable error

---

## Section 2 — Technical approach

### Setup completion detection

Extend existing `GET /api/v1/setup/status` (`backend/internal/api/handlers/setup.go`) to return:

```json
{
  "slack_connected": true,
  "has_schedule": true,
  "demo_data_available": false
}
```

Wizard dismissal stored in `localStorage` key `regen_setup_wizard_v1: { dismissed: true, currentStep: N }`.

### AuthGate redirect logic

On login, call `GET /api/v1/setup/status`. Redirect to `/setup` if:
- `slack_connected === false` AND `localStorage` key `regen_setup_wizard_v1` not set

Users who configured Slack before the wizard existed are skipped automatically.

### New frontend route

`/setup` → `frontend/src/pages/SetupWizardPage.tsx`

**Step components (each receive `onComplete` / `onSkip` callbacks):**

| Component | Reuses |
|---|---|
| `frontend/src/components/onboarding/WizardStepSlack.tsx` | Logic from `SlackSetupModal.tsx` inline |
| `frontend/src/components/onboarding/WizardStepSchedule.tsx` | `POST /api/v1/migrations/oncall/import` + `POST /api/v1/schedules` |
| `frontend/src/components/onboarding/WizardStepInvite.tsx` | `POST /api/v1/settings/users` (simplified form) |
| `frontend/src/components/onboarding/WizardStepTestAlert.tsx` | Polls `GET /api/v1/alerts?limit=5` every 3s for 2 min |
| `frontend/src/components/onboarding/WizardStepDone.tsx` | Links to incident, schedule, docs |

**Test alert polling:** shows generic webhook URL + curl. Polls every 3s for 2 minutes. On alert detected → step auto-completes. On timeout → shows "mark as done manually" button.

**Schedule step gate:** if `has_schedule: false` → Skip button hidden. If `has_schedule: true` → step pre-ticked, skippable.

### Backend changes (minimal)

1. Extend `GET /api/v1/setup/status` — add `slack_connected` and `has_schedule` fields
2. No new migrations, no new tables

### Frontend changes

1. `frontend/src/pages/SetupWizardPage.tsx` — new page, progress bar, step state machine
2. 5 step components in `frontend/src/components/onboarding/`
3. `AuthGate` — add setup status check + redirect logic
4. `frontend/src/App.tsx` — add `/setup` route

---

## Section 3 — Test strategy

### Backend unit tests (`backend/internal/api/handlers/setup_test.go`)

| Test | Covers |
|---|---|
| `TestSetupStatus_SlackConnected` | `slack_connected: true` when slack_config row exists |
| `TestSetupStatus_NoSlack` | `slack_connected: false` when table empty |
| `TestSetupStatus_HasSchedule` | `has_schedule: true` when schedule exists |
| `TestSetupStatus_NoSchedule` | `has_schedule: false` when schedules empty |
| `TestSetupStatus_BothMissing` | Both false on fresh install |

### Frontend unit tests (Vitest + React Testing Library)

| Test | Covers |
|---|---|
| `WizardStepSlack` — error on bad token | Inline validation, human-readable error |
| `WizardStepSchedule` — Skip hidden when no schedule | `has_schedule: false` hides Skip |
| `WizardStepSchedule` — Skip visible when schedule exists | `has_schedule: true` shows Skip |
| `WizardStepInvite` — shows setup link after creation | Copyable link renders on success |
| `WizardStepTestAlert` — auto-completes on alert | Poll mock returns alert → step done |
| `SetupWizardPage` — progress bar advances | Step completion increments indicator |
| `AuthGate` — redirects when slack not connected | `slack_connected: false` + no key → redirect |
| `AuthGate` — no redirect when dismissed | localStorage key set → no redirect |

### Edge cases

- Browser closed mid-step → wizard resumes at correct step (localStorage `currentStep`)
- All 5 steps skipped → wizard marks dismissed, redirects to `/`
- `has_schedule: true` on arrival → step 2 pre-ticked, advance immediately
- Test alert times out (2 min) → shows manual "mark done" button

---

## Section 4 — Security considerations

| Concern | Handling |
|---|---|
| Slack token storage | Validated server-side before saving; never stored unvalidated |
| Invite password | Auto-generated 16-char random; bcrypt 12 rounds (existing handler) |
| Setup token exposure | Displayed once, not logged, expires 7 days (existing behaviour) |
| `/setup` auth | Inside `AuthGate` — unauthenticated users cannot reach it |
| Admin-only steps | Invite + schedule import hit existing `RequireAdmin()` endpoints — non-admins get clear error |
| localStorage contents | Only `{ dismissed: true, currentStep: N }` — no tokens, no secrets |
| Webhook URL (step 4) | Read-only, derived from `window.location.origin`; already public by design |

No new attack surface — the wizard is a frontend orchestration layer over existing secured endpoints.
