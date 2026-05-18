-- +goose Up
CREATE TABLE lead_lists (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    source      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE leads (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    list_id           BIGINT REFERENCES lead_lists(id) ON DELETE SET NULL,
    campaign_id       BIGINT REFERENCES campaigns(id) ON DELETE SET NULL,
    phone_e164        TEXT NOT NULL,
    first_name        TEXT,
    last_name         TEXT,
    email             TEXT,
    timezone          TEXT,
    state_code        CHAR(2),
    status            TEXT NOT NULL DEFAULT 'new'
                       CHECK (status IN ('new','queued','in_flight','done','dnc','max_attempts','failed','opt_out')),
    attempts          INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_attempt_at   TIMESTAMPTZ,
    next_eligible_at  TIMESTAMPTZ,
    locked_by         TEXT,
    locked_until      TIMESTAMPTZ,
    custom_fields     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, campaign_id, phone_e164)
);

CREATE INDEX leads_tenant_status_idx ON leads (tenant_id, status);
CREATE INDEX leads_dispatch_idx ON leads (campaign_id, status, next_eligible_at)
    WHERE status IN ('new', 'queued');
CREATE INDEX leads_phone_idx ON leads (phone_e164);

ALTER TABLE lead_lists ENABLE ROW LEVEL SECURITY;
ALTER TABLE leads ENABLE ROW LEVEL SECURITY;

CREATE POLICY lead_lists_super_admin ON lead_lists AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');
CREATE POLICY lead_lists_tenant ON lead_lists AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

CREATE POLICY leads_super_admin ON leads AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');
CREATE POLICY leads_tenant ON leads AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON lead_lists, leads TO app_user;
GRANT USAGE, SELECT ON SEQUENCE lead_lists_id_seq, leads_id_seq TO app_user;

-- +goose Down
DROP TABLE leads;
DROP TABLE lead_lists;
