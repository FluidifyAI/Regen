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

## Testing Findings

### [Login] No logo on login page
**Found during:** Section 1.1
**Current behaviour:** Login page has no product logo — just text "OpenIncident"
**Expected behaviour:** A solid/icon logo displayed prominently on the login page. Placeholder acceptable for now; final logo asset to be designed separately.
**Priority:** Low

---

### [Sidebar] Users link should sit near the bottom, above the user profile bar
**Found during:** Manual navigation
**Current behaviour:** The Users link appears inside the "Settings" nav section in the middle of the sidebar, making it easy to miss
**Expected behaviour:** Move the Users / user management link to the bottom of the nav, directly above the user profile/avatar row — consistent with how tools like Linear and incident.io position admin-level items
**Priority:** Low

---

### [Global] Sidebar search is a placeholder — define what it should search
**Found during:** General navigation
**Current behaviour:** Clicking Search shows an `alert("Search coming soon")` — no search functionality exists
**Expected behaviour:** Global search across incidents by title and incident number (e.g. "INC-42", "database latency"). Could also search on-call schedules and escalation policies in future. Minimum viable: incident title + number search with keyboard shortcut (⌘K / Ctrl+K).
**Priority:** Medium

---

### [Incident Detail] Timeline entries display raw JSON instead of human-readable content
**Found during:** Section 3.4
**Current behaviour:** Timeline entries show raw JSON blobs (e.g. `{"channel_id":"19:abc...","channel_name":"...","channel_url":"..."}`) inline as plain text, which is unreadable and clutters the timeline
**Expected behaviour:** Each timeline entry type should be rendered appropriately:
- Channel-created entries → show a formatted "Channel created" card with a clickable "Open in Teams/Slack" button
- System messages → plain readable sentence, e.g. "Incident channel created in Microsoft Teams"
- JSON should never be surfaced raw to the user
**Priority:** Medium

---

### [Incident Detail] Slack/Teams user IDs shown instead of display names in timeline
**Found during:** Section 3.4
**Current behaviour:** Timeline entries show raw Slack user IDs (e.g. `<@UMBHZD4LT> has joined the channel`) instead of the user's actual name
**Expected behaviour:** Resolve Slack/Teams user IDs to display names at render time. Fall back to the user's email address if name resolution fails. The integration (Slack/Teams) already has the user info — it should be stored with the timeline entry or resolved on the fly.
**Priority:** Medium

---

### [Incident Detail] Properties panel is missing key fields and commands are non-functional
**Found during:** Section 3.3
**Current behaviour:**
1. "Incident commands" option in the Properties panel is shown but does nothing (not implemented)
2. "Last modified" timestamp is not displayed anywhere on the incident
3. The Properties panel only shows a limited set of fields
**Expected behaviour:**
1. Incident commands (acknowledge, resolve, reassign commander) should be functional inline actions
2. "Last modified" should be visible in the Properties panel, formatted as relative time (e.g. "3 minutes ago")
3. Consider adding: linked alert count, duration since triggered, Slack/Teams channel link, post-mortem status
**Priority:** Medium

---

### [Incident Detail] Severity selector has no descriptions — users don't know what each level means
**Found during:** Section 3.3
**Current behaviour:** The severity/priority dropdown shows labels only (Critical, High, Medium, Low) with no guidance on what each means or when to use them
**Expected behaviour:** Each severity level should have a short description shown on hover or in a tooltip, similar to how incident.io handles it:
- **Critical** — Major customer impact, all hands on deck
- **High** — Significant impact, core team engaged
- **Medium** — Partial impact or degraded performance
- **Low** — Minor issue, low customer impact
This helps responders make faster, more consistent severity decisions.
**Priority:** Medium

---

### [Routing Rules / List pages] Table UI needs a revamp — rows should be easier to read and edit inline
**Found during:** Section 5.x (Routing Rules, Escalation list pages)
**Current behaviour:** Table rows display data in a flat list with no visual hierarchy. Editing requires navigating to a separate detail page or a cramped inline form.
**Expected behaviour:** Tables should feel more interactive and modern:
- Rows should have clear hover states and visible action buttons on hover
- Edit/delete actions should be immediately accessible without a page navigation
- Consider a slide-out panel or inline expand for editing, similar to incident.io's routing rules editor
**Priority:** Medium

---

### [Global] Analytics section is missing — add a "Coming soon" placeholder
**Found during:** General navigation
**Current behaviour:** No analytics or reporting section exists anywhere in the product
**Expected behaviour:** Add an "Analytics" entry to the sidebar (under Your Organization) that navigates to a coming-soon placeholder page. The page should describe what will be available: incident frequency, MTTD/MTTR, alert volume trends, on-call load. This sets expectations for users evaluating the product.
**Priority:** Medium

---

### [Escalation Policies] Page flow is not fluid and toggle button style is inconsistent
**Found during:** Section 7.x
**Current behaviour:**
1. The escalation policy creation/editing flow feels disconnected — steps are not clearly sequenced
2. The "Enabled" toggle button uses blue inconsistently with other enabled states in the app
**Expected behaviour:**
1. The escalation flow should guide the user step-by-step: select policy → add steps → set timeouts → assign to schedule. Consider a wizard or clearly numbered step UI.
2. Standardise the enabled/active state colour to blue across all toggles, badges, and status indicators in the app
**Priority:** Medium
