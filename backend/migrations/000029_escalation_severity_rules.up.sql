CREATE TABLE IF NOT EXISTS escalation_severity_rules (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    severity              VARCHAR(50)  NOT NULL
                              CHECK (severity IN ('critical', 'high', 'medium', 'low')),
    escalation_policy_id  UUID         NOT NULL
                              REFERENCES escalation_policies(id) ON DELETE CASCADE,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_escalation_severity_rules_severity UNIQUE (severity)
);
