CREATE TABLE device_tokens (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token        TEXT        NOT NULL,
    platform     VARCHAR(10) NOT NULL DEFAULT 'web',
    app_version  VARCHAR(50) NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_device_tokens_user_token UNIQUE (user_id, token)
);

CREATE INDEX idx_device_tokens_user_id   ON device_tokens (user_id);
CREATE INDEX idx_device_tokens_last_seen ON device_tokens (last_seen_at);
