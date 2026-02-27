# UI Improvements Backlog

> Issues and polish items found during manual testing.
> Add entries here as you go through `MANUAL-TESTING.md`.
> Each entry should have: the page/component, what's wrong, and the desired behaviour.

---

## How to add an entry

```
### [Page] Short description of the issue
**Found during:** MANUAL-TESTING.md section X.Y
**Current behaviour:** What happens today
**Expected behaviour:** What it should do
**Priority:** High / Medium / Low
```

---

## Known Issues (pre-populated)

### [Login] Setup form toggle text is small and easy to miss
**Found during:** Section 1.1
**Current behaviour:** "First time here? Create admin account →" is a small grey text link between dividers
**Expected behaviour:** When no accounts exist (open mode), the setup form should be the primary CTA — bold and prominent
**Priority:** Medium

### [Sidebar] No visual indicator for active section when collapsed
**Found during:** General navigation
**Current behaviour:** In collapsed mode, the active item has a blue left border but no icon colour change
**Expected behaviour:** Active icon should be blue/white in collapsed mode for clarity
**Priority:** Low

### [Incidents] No empty state on the incidents list
**Found during:** Section 3.1
**Current behaviour:** Blank page if no incidents exist
**Expected behaviour:** "No incidents yet" illustration with a CTA to create one or set up a webhook
**Priority:** Medium

### [Incidents] Pagination missing
**Found during:** Section 3.1
**Current behaviour:** All incidents loaded at once (capped at 500 on backend)
**Expected behaviour:** Pagination or infinite scroll
**Priority:** Medium

### [Incident Detail] Timeline auto-refresh
**Found during:** Section 3.4
**Current behaviour:** Timeline requires a manual page refresh to show new entries posted from Slack/Teams
**Expected behaviour:** Live updates (polling or websocket)
**Priority:** High

### [Incident Detail] No confirmation before status changes
**Found during:** Section 3.3
**Current behaviour:** Clicking Resolve immediately changes status
**Expected behaviour:** Confirmation dialog ("Resolve this incident?") to prevent accidental clicks
**Priority:** Medium

### [On-Call] Calendar view is basic
**Found during:** Section 6.2
**Current behaviour:** Simple list/table of shift blocks
**Expected behaviour:** Visual calendar (week/month) with colour-coded shifts per user
**Priority:** Medium

### [Post-Mortems] No rich text editor
**Found during:** Section 9.3
**Current behaviour:** Plain textarea for post-mortem content
**Expected behaviour:** Markdown editor with preview (or rich text)
**Priority:** Medium

### [Settings/Users] No email invite flow
**Found during:** Section 10.2
**Current behaviour:** Setup token shown on screen — must be manually shared
**Expected behaviour:** Send invite email directly from the UI
**Priority:** High

### [Global] No toast notification system
**Found during:** Throughout
**Current behaviour:** `alert()` used in some places (e.g. Sidebar search); errors shown inline
**Expected behaviour:** Consistent toast/snackbar component for success and error feedback
**Priority:** High

### [Global] No mobile layout
**Found during:** General
**Current behaviour:** Mobile overlay sidebar exists but content pages are not responsive
**Expected behaviour:** Responsive layouts for core pages (incidents, on-call)
**Priority:** Low

---

## Add your findings below as you test

<!-- Testing notes go here -->
