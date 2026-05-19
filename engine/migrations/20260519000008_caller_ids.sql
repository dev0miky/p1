-- +goose Up
CREATE TABLE caller_ids (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    e164_number   TEXT NOT NULL CHECK (e164_number ~ '^\+[1-9][0-9]{1,14}$'),
    display_name  TEXT,
    attestation   TEXT NOT NULL DEFAULT 'none' CHECK (attestation IN ('a', 'b', 'c', 'none')),
    description   TEXT,
    tags          TEXT[] NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, e164_number),
    UNIQUE (tenant_id, name)
);

CREATE INDEX caller_ids_tenant_idx ON caller_ids (tenant_id);
CREATE INDEX caller_ids_tags_idx ON caller_ids USING GIN (tags);

ALTER TABLE caller_ids ENABLE ROW LEVEL SECURITY;

CREATE POLICY caller_ids_super_admin ON caller_ids AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY caller_ids_tenant ON caller_ids AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON caller_ids TO app_user;
GRANT USAGE, SELECT ON SEQUENCE caller_ids_id_seq TO app_user;

-- Join table: which caller_ids each campaign rotates through.
CREATE TABLE campaign_caller_ids (
    campaign_id  BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    caller_id_id BIGINT NOT NULL REFERENCES caller_ids(id) ON DELETE RESTRICT,
    attached_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, caller_id_id)
);

CREATE INDEX campaign_caller_ids_caller_idx ON campaign_caller_ids (caller_id_id);

ALTER TABLE campaign_caller_ids ENABLE ROW LEVEL SECURITY;

CREATE POLICY campaign_caller_ids_super_admin ON campaign_caller_ids AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY campaign_caller_ids_tenant ON campaign_caller_ids AS PERMISSIVE FOR ALL
    USING (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    )
    WITH CHECK (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    );

GRANT SELECT, INSERT, UPDATE, DELETE ON campaign_caller_ids TO app_user;

-- +goose Down
DROP TABLE IF EXISTS campaign_caller_ids;
DROP TABLE IF EXISTS caller_ids;
