-- +goose Up
CREATE TABLE external_agents (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    dial_string TEXT NOT NULL CHECK (length(trim(dial_string)) > 0),
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX external_agents_tenant_idx ON external_agents (tenant_id);
CREATE INDEX external_agents_tags_idx ON external_agents USING GIN (tags);

ALTER TABLE external_agents ENABLE ROW LEVEL SECURITY;

CREATE POLICY external_agents_super_admin ON external_agents AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY external_agents_tenant ON external_agents AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON external_agents TO app_user;
GRANT USAGE, SELECT ON SEQUENCE external_agents_id_seq TO app_user;

ALTER TABLE scripts
    ADD COLUMN external_agent_id BIGINT REFERENCES external_agents(id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE scripts DROP COLUMN external_agent_id;
DROP TABLE IF EXISTS external_agents;
