-- +goose Up
CREATE INDEX client_events_app_manifest_occurred_idx
    ON client_events (app_id, manifest_uuid, occurred_at DESC);

-- +goose Down
DROP INDEX IF EXISTS client_events_app_manifest_occurred_idx;
