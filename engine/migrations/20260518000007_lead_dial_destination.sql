-- +goose Up
ALTER TABLE leads ADD COLUMN dial_destination TEXT;
COMMENT ON COLUMN leads.dial_destination IS 'optional sip dest (extension or sip user) used at originate time; phone_e164 stays as the compliance/audit identifier';

-- +goose Down
ALTER TABLE leads DROP COLUMN dial_destination;
