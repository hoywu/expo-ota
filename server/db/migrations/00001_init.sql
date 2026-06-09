-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    username      citext UNIQUE NOT NULL,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz,
    disabled_at   timestamptz
);

CREATE TABLE apps (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    app_slug            text NOT NULL UNIQUE
                        CHECK (app_slug ~ '^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$'),
    name                text NOT NULL,
    description         text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE TABLE code_signing_keys (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    app_id                uuid NOT NULL,
    key_id                text NOT NULL,
    algorithm             text NOT NULL DEFAULT 'rsa-v1_5-sha256',
    public_key_pem        text NOT NULL,
    encrypted_private_key bytea NOT NULL,
    encryption_key_id     text NOT NULL,
    enabled               boolean NOT NULL DEFAULT true,
    created_at            timestamptz NOT NULL DEFAULT now(),
    disabled_at           timestamptz,
    UNIQUE (app_id, key_id)
);

ALTER TABLE code_signing_keys
    ADD CONSTRAINT code_signing_keys_app_fk
    FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE;
CREATE INDEX code_signing_keys_app_id_idx ON code_signing_keys(app_id);

CREATE TABLE runtime_versions (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    app_id     uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (app_id, version)
);
CREATE INDEX runtime_versions_app_idx ON runtime_versions (app_id);

CREATE TABLE assets (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    app_id        uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    sha256        bytea NOT NULL CHECK (octet_length(sha256) = 32),
    sha256_b64url text NOT NULL,                       -- 协议字段
    size_bytes    bigint NOT NULL CHECK (size_bytes >= 0),
    content_type  text NOT NULL,
    file_ext      text,
    storage_key   text NOT NULL,                       -- apps/{slug}/assets/{sha256_b64url}
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (app_id, sha256)
);
CREATE INDEX assets_app_sha_b64url_idx ON assets (app_id, sha256_b64url);

CREATE TABLE updates (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    app_id             uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    runtime_version_id uuid NOT NULL REFERENCES runtime_versions(id),
    platform           text NOT NULL CHECK (platform IN ('ios','android')),
    manifest_uuid      uuid NOT NULL,                  -- 协议字段 manifest.id
    launch_asset_id    uuid NOT NULL REFERENCES assets(id),
    status             text NOT NULL DEFAULT 'published'
                       CHECK (status IN ('published')),
    message            text,
    git_commit_hash    text,
    manifest_metadata  jsonb NOT NULL DEFAULT '{}',
    extra              jsonb NOT NULL DEFAULT '{}',
    expo_config        jsonb,                          -- expoConfig.json 原文
    manifest_snapshot  jsonb NOT NULL,                 -- 缓存的 manifest JSON 字节
    rolled_back_from   uuid REFERENCES updates(id),
    created_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    UNIQUE (app_id, manifest_uuid)
);
CREATE INDEX updates_latest_idx
    ON updates (app_id, runtime_version_id, platform, created_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX updates_app_created_idx
    ON updates (app_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE update_assets (
    update_id  uuid NOT NULL REFERENCES updates(id) ON DELETE CASCADE,
    asset_id   uuid NOT NULL REFERENCES assets(id),
    asset_key  text NOT NULL,                          -- manifest.assets[].key (md5 hex)
    file_ext   text,
    sort_order int NOT NULL DEFAULT 0,
    PRIMARY KEY (update_id, asset_key)
);
CREATE INDEX update_assets_asset_idx ON update_assets (asset_id);

CREATE TABLE api_tokens (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    app_id       uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    created_by   uuid NOT NULL REFERENCES users(id),
    name         text NOT NULL,
    token_hash   bytea NOT NULL UNIQUE,                -- sha256(token)
    scopes       text[] NOT NULL DEFAULT ARRAY['publish']::text[],
    last_used_at timestamptz,
    expires_at   timestamptz,                          -- 可空
    created_at   timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz
);
CREATE INDEX api_tokens_app_active_idx
    ON api_tokens (app_id) WHERE revoked_at IS NULL;

CREATE TABLE manifest_requests (
    id               bigint GENERATED ALWAYS AS IDENTITY,
    app_id           uuid NOT NULL,
    occurred_at      timestamptz NOT NULL DEFAULT now(),
    runtime_version  text NOT NULL,
    platform         text NOT NULL,
    device_id        text,                             -- expo-device-id header / X-Forwarded-For
    served_update_id uuid,
    result           text NOT NULL
                     CHECK (result IN ('update','no_update','rollback','not_found','bad_request','not_acceptable','error')),
    http_status      smallint NOT NULL,
    PRIMARY KEY (occurred_at, id)
) PARTITION BY RANGE (occurred_at);

CREATE TABLE manifest_requests_default PARTITION OF manifest_requests DEFAULT;
-- CREATE TABLE manifest_requests_y2026m06 PARTITION OF manifest_requests
--     FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
-- CREATE TABLE manifest_requests_y2026m07 PARTITION OF manifest_requests
--     FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE TABLE client_events (
    id              bigint GENERATED ALWAYS AS IDENTITY,
    app_id          uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    occurred_at     timestamptz NOT NULL,              -- 客户端声称的时间
    received_at     timestamptz NOT NULL DEFAULT now(),
    event_id        uuid NOT NULL,
    event_type      text NOT NULL
                    CHECK (event_type IN ('update_succeeded','update_failed')),
    update_id       uuid,
    manifest_uuid   uuid,
    runtime_version text,
    platform        text,
    device_id       text NOT NULL,
    app_version     text,
    os_version      text,
    duration_ms     int,
    error_code      text,
    error_message   text,
    PRIMARY KEY (received_at, id)
) PARTITION BY RANGE (received_at);

CREATE INDEX client_events_app_event_idx
    ON client_events (app_id, event_id);

CREATE TABLE client_events_default PARTITION OF client_events DEFAULT;
-- CREATE TABLE client_events_y2026m06 PARTITION OF client_events
--     FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
-- CREATE TABLE client_events_y2026m07 PARTITION OF client_events
--     FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE TABLE audit_logs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    app_id        uuid REFERENCES apps(id) ON DELETE SET NULL,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action        text NOT NULL,                       -- publish_update / rollback_update / delete_update / cleanup_updates / create_app / create_user / login_failed / ...
    target_type   text,
    target_id     text,
    request_id    text,
    ip            inet,
    user_agent    text,
    payload       jsonb NOT NULL DEFAULT '{}',
    occurred_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_app_time_idx ON audit_logs (app_id, occurred_at DESC);
CREATE INDEX audit_logs_actor_time_idx ON audit_logs (actor_user_id, occurred_at DESC);
CREATE INDEX audit_logs_action_time_idx ON audit_logs (action, occurred_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;

DROP TABLE IF EXISTS client_events;
DROP TABLE IF EXISTS manifest_requests;

DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS update_assets;
DROP TABLE IF EXISTS updates;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS runtime_versions;

ALTER TABLE IF EXISTS apps DROP CONSTRAINT IF EXISTS apps_signing_key_fk;
DROP TABLE IF EXISTS code_signing_keys;
DROP TABLE IF EXISTS apps;

DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS citext;
