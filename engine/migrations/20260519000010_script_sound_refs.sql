-- +goose Up
ALTER TABLE scripts
    ADD COLUMN greeting_sound_id   BIGINT REFERENCES sounds(id) ON DELETE SET NULL,
    ADD COLUMN pre_bridge_sound_id BIGINT REFERENCES sounds(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE scripts
    DROP COLUMN pre_bridge_sound_id,
    DROP COLUMN greeting_sound_id;
