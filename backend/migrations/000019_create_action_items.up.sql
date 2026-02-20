CREATE TABLE action_items (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    post_mortem_id UUID         NOT NULL REFERENCES post_mortems(id) ON DELETE CASCADE,
    title          VARCHAR(500) NOT NULL,
    owner          VARCHAR(255),
    due_date       TIMESTAMPTZ,
    status         VARCHAR(20)  NOT NULL DEFAULT 'open'
                                CHECK (status IN ('open', 'in_progress', 'closed')),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_action_items_post_mortem_id ON action_items (post_mortem_id);
CREATE INDEX idx_action_items_open ON action_items (post_mortem_id) WHERE status != 'closed';

COMMENT ON TABLE action_items IS 'Action items linked to a post-mortem. Cascade-deleted when the post-mortem is deleted. (v0.7)';
