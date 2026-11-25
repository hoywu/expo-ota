-- +goose Up
ALTER TABLE updates DROP CONSTRAINT updates_platform_check,
    ADD CONSTRAINT updates_platform_check CHECK (platform IN ('ios', 'android', 'windows'));

-- +goose Down
ALTER TABLE updates DROP CONSTRAINT updates_platform_check,
    ADD CONSTRAINT updates_platform_check CHECK (platform IN ('ios', 'android'));
