-- +goose Up
ALTER TABLE scripts
    ADD COLUMN bridge_digit    TEXT    NOT NULL DEFAULT '1'
        CHECK (bridge_digit ~ '^[0-9*#]$'),
    ADD COLUMN wait_timeout_ms INT     NOT NULL DEFAULT 8000
        CHECK (wait_timeout_ms BETWEEN 1000 AND 60000),
    ADD COLUMN opt_out_digit   TEXT
        CHECK (opt_out_digit IS NULL OR opt_out_digit ~ '^[0-9*#]$'),
    ADD CONSTRAINT scripts_distinct_digits
        CHECK (opt_out_digit IS NULL OR bridge_digit <> opt_out_digit);

UPDATE scripts SET opt_out_digit = '9' WHERE type = 'press1';

-- +goose Down
ALTER TABLE scripts
    DROP CONSTRAINT scripts_distinct_digits,
    DROP COLUMN opt_out_digit,
    DROP COLUMN wait_timeout_ms,
    DROP COLUMN bridge_digit;
