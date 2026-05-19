-- +goose Up
ALTER TABLE scripts ADD COLUMN transfer_to TEXT;

-- +goose Down
ALTER TABLE scripts DROP COLUMN transfer_to;
