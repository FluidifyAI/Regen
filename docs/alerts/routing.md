# Alert Routing Rules

Routing rules control what Regen does with an alert after deduplication and grouping. Rules are evaluated in priority order — the first matching rule wins.

Configure under **Settings → Routing Rules**.

---

## Match Criteria

The `match_criteria` object specifies which alerts a rule applies to. All fields are optional; omitting a field means "match anything". All specified fields must match (AND logic).

```json
{
  "source":      ["prometheus", "grafana"],
  "severity":    ["critical", "warning"],
  "labels": {
    "env":       "prod",
    "alertname": "DiskUsage.*",
    "svc":       "*"
  },
  "annotations": {
    "summary": ".*connection refused.*"
  },
  "title":       ".*disk.*",
  "description": ".*OOM.*"
}
```

### Fields

| Field | Type | Matching |
|-------|------|---------|
| `source` | `string[]` | Exact list — alert source must be one of the values |
| `severity` | `string[]` | Exact list — alert severity must be one of the values |
| `labels` | `object` | Per-key match against alert labels (see below) |
| `annotations` | `object` | Per-key match against alert annotations (see below) |
| `title` | `string` | RE2 regex matched against the alert title |
| `description` | `string` | RE2 regex matched against the alert description |

### Label and annotation value matching

Each value in `labels` and `annotations` is matched independently:

| Value | Meaning |
|-------|---------|
| `"prod"` | Exact string match |
| `"prod-.*"` | RE2 regex (matches `prod-eu`, `prod-us`, etc.) |
| `"*"` | Key present with any non-empty value |

Regex uses Go's [RE2 syntax](https://pkg.go.dev/regexp/syntax) — no lookaheads or backreferences.

### How `title` and `description` are derived

| Source | `title` | `description` |
|--------|---------|---------------|
| Prometheus | `labels.alertname` | `annotations.summary` |
| Grafana | Rule name | `annotations.summary` |
| CloudWatch | Alarm name | Alarm description |
| Generic | `title` field | `description` field |

---

## Actions

| Action | Key | Description |
|--------|-----|-------------|
| Create incident | `"create_incident": true` | Automatically open an incident (default) |
| Suppress | `"suppress": true` | Store the alert but create no incident |
| Override severity | `"severity_override": "critical"` | Change the alert's severity before incident creation |
| Override channel | `"channel_override": "db-oncall"` | Use a different Slack/Teams channel name suffix |

```json
{
  "create_incident":   true,
  "severity_override": "critical",
  "channel_override":  "db-oncall"
}
```

---

## Examples

### Suppress noisy disk-space alerts in staging

Drop alerts from the `staging` namespace that match a disk usage pattern — they fire constantly during CI and are not worth paging anyone.

```json
{
  "match_criteria": {
    "labels": {
      "namespace": "staging",
      "alertname": "DiskUsage.*"
    }
  },
  "actions": {
    "suppress": true
  }
}
```

### Escalate any production database error to critical

Alerts from the `payments-db` service are automatically upgraded to `critical` severity regardless of what Prometheus reports.

```json
{
  "match_criteria": {
    "labels": {
      "service": "payments-db",
      "env":     "prod"
    }
  },
  "actions": {
    "create_incident":   true,
    "severity_override": "critical",
    "channel_override":  "db-oncall"
  }
}
```

### Suppress alerts whose annotation mentions a known maintenance window

Use an annotation regex to catch alerts that already self-describe as expected.

```json
{
  "match_criteria": {
    "annotations": {
      "summary": ".*scheduled maintenance.*"
    }
  },
  "actions": {
    "suppress": true
  }
}
```

### Catch-all: create an incident for everything else

Set a low-priority rule (high priority number) with an empty `match_criteria` as a fallback.

```json
{
  "match_criteria": {},
  "actions": {
    "create_incident": true
  }
}
```

---

## Evaluation order

Rules are sorted by `priority` (ascending — lower number = higher priority). The **first** rule whose `match_criteria` matches the alert determines the action. Subsequent rules are not evaluated.

A catch-all rule (`match_criteria: {}`) at the end of the list acts as a default.
