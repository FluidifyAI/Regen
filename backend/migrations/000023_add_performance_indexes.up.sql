-- Performance indexes identified by audit (2026-02-24).
--
-- All indexes use IF NOT EXISTS so the migration is safe to re-run.
-- Partial indexes are used where a column is mostly NULL to keep the index small.

-- incidents.commander_id: foreign key with no index; used in ownership/assignment lookups.
CREATE INDEX IF NOT EXISTS idx_incidents_commander_id
    ON incidents (commander_id)
    WHERE commander_id IS NOT NULL;

-- incidents.created_by_id: no index; used in audit/ownership queries.
CREATE INDEX IF NOT EXISTS idx_incidents_created_by_id
    ON incidents (created_by_id);

-- alerts.fingerprint: used for deduplication in the alert processing pipeline.
CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint
    ON alerts (fingerprint);

-- alerts.labels (GIN): enables future label-based alert filtering without full table scans.
CREATE INDEX IF NOT EXISTS idx_alerts_labels
    ON alerts USING GIN (labels);

-- post_mortems.created_by_id: supports "my post-mortems" list views.
CREATE INDEX IF NOT EXISTS idx_post_mortems_created_by_id
    ON post_mortems (created_by_id);

-- post_mortem_templates.name: supports template name lookups and search.
CREATE INDEX IF NOT EXISTS idx_post_mortem_templates_name
    ON post_mortem_templates (name);

-- action_items.owner: supports "my action items" list views.
CREATE INDEX IF NOT EXISTS idx_action_items_owner
    ON action_items (owner);

-- action_items.due_date + status: supports overdue/upcoming action item queries.
CREATE INDEX IF NOT EXISTS idx_action_items_due_date_status
    ON action_items (due_date, status)
    WHERE status != 'closed';
