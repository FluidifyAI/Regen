-- Create grouping_rules table for alert grouping configuration
--
-- Grouping rules define how alerts should be combined into a single incident
-- based on label matching and time windows.
--
-- Design decisions:
--   - JSONB for flexible label matching (match_labels) and cross-source correlation (cross_source_labels)
--   - UNIQUE constraint on priority to enforce explicit ordering
--   - Default time window of 300 seconds (5 minutes)
--   - Timestamps for audit trail

CREATE TABLE grouping_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Human-readable identification
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',

    -- Rule activation and priority
    enabled BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL,

    -- Label matching configuration
    -- Example: {"alertname": "*"} matches all alerts with an alertname label
    -- Example: {"service": "api", "env": "prod"} matches specific service in prod
    match_labels JSONB NOT NULL DEFAULT '{}',

    -- Grouping time window in seconds
    -- Alerts within this window are grouped together
    time_window_seconds INTEGER NOT NULL DEFAULT 300,

    -- Cross-source correlation labels
    -- Example: ["service", "env"] enables grouping Prometheus and Grafana alerts
    -- for the same service+environment combination
    cross_source_labels JSONB DEFAULT '[]',

    -- Audit timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure unique priority values (explicit ordering)
    CONSTRAINT grouping_rules_priority_unique UNIQUE (priority)
);

-- Index for efficient enabled rules lookup (most common query)
CREATE INDEX idx_grouping_rules_enabled_priority ON grouping_rules (enabled, priority)
    WHERE enabled = true;

-- Index for JSONB label matching queries
CREATE INDEX idx_grouping_rules_match_labels ON grouping_rules USING GIN (match_labels);

-- Seed default grouping rule
-- This preserves existing behavior: group alerts with same alertname within 5 minutes
INSERT INTO grouping_rules (name, description, priority, match_labels, time_window_seconds)
VALUES (
    'Default: group by alertname',
    'Groups alerts with the same alertname label within a 5-minute window into a single incident. This prevents alert fatigue when the same alert fires multiple times in quick succession.',
    100,
    '{"alertname": "*"}',
    300
);
