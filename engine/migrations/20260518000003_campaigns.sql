-- +goose Up
CREATE TABLE campaigns (
    id               BIGSERIAL PRIMARY KEY,
    tenant_id        BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name             TEXT NOT NULL,
    mode             TEXT NOT NULL CHECK (mode IN ('press1','broadcast','predictive','preview')),
    status           TEXT NOT NULL DEFAULT 'paused' CHECK (status IN ('paused','active','completed','archived')),
    dial_ratio       NUMERIC(5,2) NOT NULL DEFAULT 1.0 CHECK (dial_ratio > 0 AND dial_ratio <= 10),
    max_abandon_pct  NUMERIC(4,2) NOT NULL DEFAULT 3.0 CHECK (max_abandon_pct >= 0 AND max_abandon_pct <= 100),
    prompt_audio     TEXT,
    transfer_dest    TEXT,
    caller_id_pool   JSONB NOT NULL DEFAULT '[]'::jsonb,
    retry_policy     JSONB NOT NULL DEFAULT '{}'::jsonb,
    calling_hours    JSONB NOT NULL DEFAULT '{}'::jsonb,
    tz_strategy      TEXT NOT NULL DEFAULT 'lead_local' CHECK (tz_strategy IN ('lead_local','campaign_local')),
    dnc_list_ids     BIGINT[] NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
CREATE INDEX campaigns_tenant_status_idx ON campaigns (tenant_id, status);

ALTER TABLE campaigns ENABLE ROW LEVEL SECURITY;

CREATE POLICY campaigns_super_admin ON campaigns AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY campaigns_tenant ON campaigns AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON campaigns TO app_user;
GRANT USAGE, SELECT ON SEQUENCE campaigns_id_seq TO app_user;

-- +goose Down
DROP TABLE campaigns;
