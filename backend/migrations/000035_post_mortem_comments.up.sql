CREATE TABLE post_mortem_comments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_mortem_id UUID NOT NULL REFERENCES post_mortems(id) ON DELETE CASCADE,
    author_id      VARCHAR(255) NOT NULL DEFAULT 'anonymous',
    author_name    VARCHAR(255) NOT NULL DEFAULT 'Unknown',
    content        TEXT NOT NULL CHECK (char_length(content) >= 1 AND char_length(content) <= 5000),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pm_comments_post_mortem_id ON post_mortem_comments(post_mortem_id);
