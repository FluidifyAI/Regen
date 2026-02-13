-- Migration 000009: Routing Rules Table
--
-- Routing rules determine what happens when an alert is processed:
-- - Which alerts create incidents (vs. suppressed)
-- - Severity overrides (e.g., downgrade info alerts to not create incidents)
-- - Channel overrides (e.g., route DB alerts to #db-oncall instead of auto-named channel)
--
-- Rules are evaluated in priority order (lowest number = highest priority).
-- First matching rule wins. Unmatched alerts use default behavior (create incident).

CREATE TABLE routing_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL,
    match_criteria JSONB NOT NULL DEFAULT '{}',
    actions JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT routing_rules_priority_unique UNIQUE (priority)
);

-- Partial index for the hot-path query: GetEnabled() called on every alert
CREATE INDEX idx_routing_rules_enabled_priority
ON routing_rules (priority)
WHERE enabled = true;

-- GIN index for JSONB containment queries on match_criteria
CREATE INDEX idx_routing_rules_match_criteria
ON routing_rules USING GIN (match_criteria);

-- Default rule: create incidents for critical and warning alerts (preserves v0.2 behavior)
INSERT INTO routing_rules (name, description, priority, match_criteria, actions)
VALUES (
    'Default: create incidents for critical and warning',
    'Preserves existing behavior: critical and warning severity alerts auto-create incidents. This rule should not be deleted.',
    1000,
    '{"severity": ["critical", "warning"]}',
    '{"create_incident": true}'
);
