# Grouped Alerts UI (OI-105)

## Overview

The Grouped Alerts UI provides visual indicators and enhanced display for incidents that have multiple alerts grouped together via grouping rules. This makes it easy to understand why alerts were grouped and identify cross-source correlation.

---

## Features

### 1. Grouping Header

When an incident has a `group_key` (indicating it was created via grouping rules), a prominent header is shown:

```
┌──────────────────────────────────────────────┐
│ 🔗 Grouped Incident                          │
│                                               │
│ 3 alerts from 2 different sources grouped    │
│ together using alert grouping rules.         │
│                                               │
│ Sources: prometheus  grafana                  │
└──────────────────────────────────────────────┘
```

**Key Information:**
- **Alert count**: How many alerts are in this group
- **Source diversity**: Highlights when alerts come from different monitoring systems
- **Source badges**: Visual chips showing all unique sources

---

### 2. Visual Connectors

Grouped alerts are visually connected with vertical lines to show they're part of the same group:

```
┌─ Alert 1 (Prometheus) ─────────────┐
│                                     │
│ HighCPU - service=api, env=prod    │
└─────────────────────────────────────┘
         │ (connector line)
┌─ Alert 2 (Grafana) ────────────────┐
│                                     │
│ HighLatency - service=api, env=prod│
└─────────────────────────────────────┘
         │
┌─ Alert 3 (CloudWatch) ─────────────┐
│                                     │
│ HighErrors - service=api, env=prod │
└─────────────────────────────────────┘
```

---

### 3. Cross-Source Highlighting

When alerts come from different monitoring sources, the **source name** is highlighted in the brand color:

- Regular (single source): `Source: prometheus`
- Cross-source: `Source: prometheus` (in blue/brand color)

This makes it immediately obvious when cross-source correlation is happening.

---

### 4. Enhanced Alert Cards

Each alert card shows:

**Header:**
- 🔗 Link icon (for grouped alerts)
- Alert title
- Severity badge
- Resolved badge (if applicable)

**Content:**
- Description
- **Source** (highlighted if cross-source)
- Start time
- End time (if resolved)

**Labels (Expandable):**
- Click "Show labels (N)" to see alert labels
- Displays up to 10 labels in a grid
- Shows "+N more labels" if there are additional labels

---

### 5. Group Key Display (Advanced)

For debugging or advanced users, an expandable section shows the group key:

```
Advanced: Group Key ▼

This incident was grouped using the following key
(SHA256 hash of alert labels):

67cc029136ef93ee8d95c02cf600dc4491d1b54af3330d7fb13145a25afe12fb

All alerts with the same group key within the configured
time window are grouped into this incident.
```

---

## Visual Design

### Color Scheme

**Grouped Incidents:**
- Border: Brand primary with low opacity (`border-brand-primary/30`)
- Background: Brand light with low opacity (`bg-brand-light/30`)
- Hover: Brand light with higher opacity (`bg-brand-light/50`)

**Regular Incidents:**
- Border: Standard border color (`border-border`)
- Background: White (`bg-white`)
- Hover: Surface secondary (`bg-surface-secondary`)

### Icons

- **🔗 Link2**: Indicates this alert is part of a group
- **📊 Layers**: Shows grouping header icon
- **🔔 AlertCircle**: Alert severity indicator

---

## User Flows

### Flow 1: Viewing a Grouped Incident

1. User opens incident detail page (e.g., INC-123)
2. Clicks "Alerts" tab
3. **Sees grouping header** explaining why alerts were grouped
4. **Sees visual connectors** between related alerts
5. **Sees source diversity** if alerts come from multiple sources
6. Can expand "Show labels" to see why alerts matched the grouping rule

### Flow 2: Understanding Cross-Source Correlation

1. User notices **"2 different sources"** in the grouping header
2. **Source badges** show `prometheus` and `grafana`
3. **Each alert card** has the source name highlighted in blue
4. User can expand alert labels to see:
   - Prometheus alert: `{service: "api", env: "production", alertname: "HighCPU"}`
   - Grafana alert: `{service: "api", env: "production", alertname: "HighLatency"}`
5. User understands both alerts are for the same service/environment

### Flow 3: Debugging Grouping Issues

1. User wonders why specific alerts were grouped
2. Expands **"Advanced: Group Key"** section
3. Sees the SHA256 hash used for grouping
4. Can compare this against grouping rule configuration
5. Can verify alerts with same group key are grouped together

---

## Example Screenshots (Text Representation)

### Single-Source Grouped Incident

```
╔══════════════════════════════════════════════════╗
║ 🔗 Grouped Incident                              ║
║                                                  ║
║ 3 alerts from prometheus grouped together using ║
║ alert grouping rules.                            ║
╚══════════════════════════════════════════════════╝

┌──────────────────────────────────────────────────┐
│ 🔗 HighCPU                           [CRITICAL]  │
│ CPU usage above 90% on web-01                    │
│ Source: prometheus  •  Feb 12, 3:30 PM          │
└──────────────────────────────────────────────────┘
    │
┌──────────────────────────────────────────────────┐
│ 🔗 HighCPU                           [CRITICAL]  │
│ CPU usage above 90% on web-02                    │
│ Source: prometheus  •  Feb 12, 3:31 PM          │
└──────────────────────────────────────────────────┘
    │
┌──────────────────────────────────────────────────┐
│ 🔗 HighCPU                           [CRITICAL]  │
│ CPU usage above 90% on web-03                    │
│ Source: prometheus  •  Feb 12, 3:32 PM          │
└──────────────────────────────────────────────────┘
```

### Cross-Source Grouped Incident

```
╔══════════════════════════════════════════════════╗
║ 🔗 Grouped Incident                              ║
║                                                  ║
║ 3 alerts from 3 different sources grouped       ║
║ together using alert grouping rules.            ║
║                                                  ║
║ Sources:  [prometheus]  [grafana]  [cloudwatch] ║
╚══════════════════════════════════════════════════╝

┌──────────────────────────────────────────────────┐
│ 🔗 HighCPU                           [CRITICAL]  │
│ CPU usage above 90%                              │
│ Source: prometheus  •  Feb 12, 3:30 PM          │
└──────────────────────────────────────────────────┘
    │
┌──────────────────────────────────────────────────┐
│ 🔗 HighLatency                       [CRITICAL]  │
│ API latency above 2s                             │
│ Source: grafana  •  Feb 12, 3:31 PM             │
└──────────────────────────────────────────────────┘
    │
┌──────────────────────────────────────────────────┐
│ 🔗 HighErrorRate                     [CRITICAL]  │
│ Error rate above 5%                              │
│ Source: cloudwatch  •  Feb 12, 3:32 PM          │
└──────────────────────────────────────────────────┘
```

---

## Implementation Details

### Component Structure

```tsx
<GroupedAlerts>
  {isGrouped && (
    <GroupingHeader>
      - Alert count
      - Source diversity indicator
      - Source badges
    </GroupingHeader>
  )}

  <AlertsList>
    {alerts.map((alert) => (
      <AlertCard>
        {isGrouped && <VisualConnector />}
        <AlertContent>
          {isGrouped && <LinkIcon />}
          <AlertHeader />
          <AlertDescription />
          <AlertMetadata>
            - Source (highlighted if cross-source)
            - Timestamps
          </AlertMetadata>
          <ExpandableLabels />
        </AlertContent>
      </AlertCard>
    ))}
  </AlertsList>

  {isGrouped && (
    <AdvancedSection>
      - Group key display
      - Explanation
    </AdvancedSection>
  )}
</GroupedAlerts>
```

### TypeScript Types

```typescript
interface GroupedAlertsProps {
  alerts: Alert[]
  incident: Incident  // Must include group_key field
}

interface Incident {
  // ... existing fields
  group_key?: string  // SHA256 hash for grouped incidents
}
```

### Styling

**Grouped Alert Cards:**
- Border: `border-brand-primary/30`
- Background: `bg-brand-light/30`
- Hover: `bg-brand-light/50`

**Visual Connectors:**
- Color: `bg-brand-primary/30`
- Width: `w-0.5`
- Height: `h-4`

**Source Highlighting (Cross-Source):**
- Color: `text-brand-primary`
- Weight: `font-medium`

---

## Benefits

### 1. Reduced Alert Fatigue

Instead of seeing 10 separate alert cards, users see:
- **1 grouped incident** with clear explanation
- Visual indication of why alerts are related
- Easy-to-understand grouping logic

### 2. Cross-Source Visibility

Users can immediately see when:
- Alerts come from multiple monitoring systems
- Different tools are detecting the same underlying issue
- Correlation is happening across infrastructure

### 3. Debugging Support

Advanced users can:
- Inspect the group key
- Understand which labels were used for grouping
- Verify grouping rule behavior

### 4. Better Incident Understanding

At a glance, users know:
- How many alerts are in this incident
- Where the alerts came from
- If this is a multi-source issue

---

## Future Enhancements

### Potential Improvements:

1. **Show Matching Rule Name**
   - Display which grouping rule caused the grouping
   - Link to rule configuration

2. **Interactive Group Key**
   - Click group key to see all incidents with same key
   - Search/filter by group key

3. **Timeline Visualization**
   - Show when each alert was added to the group
   - Visualize alert arrival pattern over time

4. **Un-Group Action**
   - Allow users to manually un-link alerts if grouped incorrectly
   - Feedback mechanism to improve grouping rules

---

## Related Documentation

- **[cross-source-correlation.md](cross-source-correlation.md)** - How cross-source grouping works
- **[grouping-rules-api.md](grouping-rules-api.md)** - API for managing grouping rules
- **[EPIC-013-PROGRESS.md](../EPIC-013-PROGRESS.md)** - Implementation details

---

**Status:** OI-105 ✅ Complete (v0.3)
