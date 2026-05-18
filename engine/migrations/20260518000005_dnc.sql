-- +goose Up
CREATE TABLE dnc_entries (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    scope       TEXT NOT NULL CHECK (scope IN ('internal','federal','state','wireless','rnd','custom')),
    state_code  CHAR(2),
    phone_e164  TEXT NOT NULL,
    source      TEXT,
    reason      TEXT,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,
    CHECK ((scope = 'internal') = (tenant_id IS NOT NULL)),
    CHECK ((scope = 'state') = (state_code IS NOT NULL))
);

CREATE UNIQUE INDEX dnc_internal_unique ON dnc_entries (tenant_id, phone_e164) WHERE scope = 'internal';
CREATE UNIQUE INDEX dnc_federal_unique ON dnc_entries (phone_e164) WHERE scope = 'federal';
CREATE UNIQUE INDEX dnc_state_unique ON dnc_entries (state_code, phone_e164) WHERE scope = 'state';
CREATE INDEX dnc_phone_idx ON dnc_entries (phone_e164);
CREATE INDEX dnc_expires_idx ON dnc_entries (expires_at) WHERE expires_at IS NOT NULL;

ALTER TABLE dnc_entries ENABLE ROW LEVEL SECURITY;

CREATE POLICY dnc_super_admin ON dnc_entries AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY dnc_tenant_internal ON dnc_entries AS PERMISSIVE FOR ALL
    USING (scope = 'internal' AND tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (scope = 'internal' AND tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

CREATE POLICY dnc_tenant_read_global ON dnc_entries AS PERMISSIVE FOR SELECT
    USING (scope IN ('federal','state','wireless','rnd'));

CREATE TABLE opt_outs (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    campaign_id   BIGINT REFERENCES campaigns(id) ON DELETE SET NULL,
    phone_e164    TEXT NOT NULL,
    channel       TEXT NOT NULL CHECK (channel IN ('ivr_dtmf','agent','sms','web','api')),
    evidence_ref  TEXT,
    captured_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX opt_outs_tenant_idx ON opt_outs (tenant_id, captured_at DESC);
CREATE INDEX opt_outs_phone_idx ON opt_outs (phone_e164);

ALTER TABLE opt_outs ENABLE ROW LEVEL SECURITY;

CREATE POLICY opt_outs_super_admin ON opt_outs AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY opt_outs_tenant ON opt_outs AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON dnc_entries, opt_outs TO app_user;
GRANT USAGE, SELECT ON SEQUENCE dnc_entries_id_seq, opt_outs_id_seq TO app_user;

-- +goose Down
DROP TABLE opt_outs;
DROP TABLE dnc_entries;
