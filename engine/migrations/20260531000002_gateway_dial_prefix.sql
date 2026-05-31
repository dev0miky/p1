-- +goose Up
ALTER TABLE gateways ADD COLUMN dial_prefix TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE gateways DROP COLUMN dial_prefix;
