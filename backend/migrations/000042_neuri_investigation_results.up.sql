CREATE TABLE neuri_investigation_results (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id          UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    investigation_run_id UUID NOT NULL,
    top_hypothesis       TEXT NOT NULL,
    confidence           DOUBLE PRECISION NOT NULL,
    summary              TEXT NOT NULL,
    ranked_hypotheses    JSONB NOT NULL DEFAULT '[]',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_neuri_results_incident_id ON neuri_investigation_results (incident_id);
