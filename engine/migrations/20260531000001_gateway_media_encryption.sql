-- +goose Up
ALTER TABLE gateways ADD COLUMN media_encryption TEXT NOT NULL DEFAULT 'none'
    CHECK (media_encryption IN ('none','srtp'));

-- +goose Down
ALTER TABLE gateways DROP COLUMN media_encryption;
