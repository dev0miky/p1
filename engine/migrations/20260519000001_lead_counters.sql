-- +goose Up
ALTER TABLE leads
    ADD COLUMN n_calls               INT NOT NULL DEFAULT 0 CHECK (n_calls >= 0),
    ADD COLUMN n_answered            INT NOT NULL DEFAULT 0 CHECK (n_answered >= 0),
    ADD COLUMN n_ringed              INT NOT NULL DEFAULT 0 CHECK (n_ringed >= 0),
    ADD COLUMN n_voicemail           INT NOT NULL DEFAULT 0 CHECK (n_voicemail >= 0),
    ADD COLUMN n_transferred         INT NOT NULL DEFAULT 0 CHECK (n_transferred >= 0),
    ADD COLUMN n_transfer_completed  INT NOT NULL DEFAULT 0 CHECK (n_transfer_completed >= 0),
    ADD COLUMN n_error               INT NOT NULL DEFAULT 0 CHECK (n_error >= 0),
    ADD COLUMN n_went_to_dnc         INT NOT NULL DEFAULT 0 CHECK (n_went_to_dnc >= 0),
    ADD COLUMN first_call_time       TIMESTAMPTZ,
    ADD COLUMN last_call_time        TIMESTAMPTZ;

COMMENT ON COLUMN leads.n_calls IS 'incremented per originate (queued -> originating transition)';
COMMENT ON COLUMN leads.n_answered IS 'incremented on channel_answer';
COMMENT ON COLUMN leads.n_ringed IS 'incremented on channel_create';
COMMENT ON COLUMN leads.n_voicemail IS 'incremented on terminal voicemail state';
COMMENT ON COLUMN leads.n_transferred IS 'incremented on bridging state (transfer initiated)';
COMMENT ON COLUMN leads.n_transfer_completed IS 'incremented on bridged state (transfer connected)';
COMMENT ON COLUMN leads.n_error IS 'incremented on terminal failed state';
COMMENT ON COLUMN leads.n_went_to_dnc IS 'incremented when lead moves to opt_out / DNC';

-- +goose Down
ALTER TABLE leads
    DROP COLUMN n_calls,
    DROP COLUMN n_answered,
    DROP COLUMN n_ringed,
    DROP COLUMN n_voicemail,
    DROP COLUMN n_transferred,
    DROP COLUMN n_transfer_completed,
    DROP COLUMN n_error,
    DROP COLUMN n_went_to_dnc,
    DROP COLUMN first_call_time,
    DROP COLUMN last_call_time;
