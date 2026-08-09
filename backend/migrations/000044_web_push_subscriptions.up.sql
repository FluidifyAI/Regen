CREATE TABLE web_push_subscriptions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint     TEXT        NOT NULL,
    p256dh       TEXT        NOT NULL,
    auth         TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_web_push_user_endpoint UNIQUE (user_id, endpoint)
);

CREATE INDEX idx_web_push_user_id   ON web_push_subscriptions (user_id);
CREATE INDEX idx_web_push_last_seen ON web_push_subscriptions (last_seen_at);
