-- +goose Up
ALTER TABLE lead_lists ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE scripts    ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE sounds     ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX lead_lists_tags_idx ON lead_lists USING GIN (tags);
CREATE INDEX scripts_tags_idx    ON scripts    USING GIN (tags);
CREATE INDEX sounds_tags_idx     ON sounds     USING GIN (tags);

COMMENT ON COLUMN lead_lists.tags IS 'free-form labels for grouping/filtering; campaign builder uses these as selectors later';
COMMENT ON COLUMN scripts.tags    IS 'free-form labels for grouping/filtering';
COMMENT ON COLUMN sounds.tags     IS 'free-form labels for grouping/filtering';

-- +goose Down
DROP INDEX IF EXISTS sounds_tags_idx;
DROP INDEX IF EXISTS scripts_tags_idx;
DROP INDEX IF EXISTS lead_lists_tags_idx;
ALTER TABLE sounds     DROP COLUMN tags;
ALTER TABLE scripts    DROP COLUMN tags;
ALTER TABLE lead_lists DROP COLUMN tags;
