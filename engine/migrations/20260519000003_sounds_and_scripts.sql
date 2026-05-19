-- +goose Up
CREATE TABLE sounds (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT,
    file_key     TEXT NOT NULL,
    mime_type    TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL CHECK (size_bytes >= 0),
    duration_ms  INT,
    sha256       TEXT,
    status       TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('pending', 'ready', 'failed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX sounds_tenant_idx ON sounds (tenant_id);

ALTER TABLE sounds ENABLE ROW LEVEL SECURITY;

CREATE POLICY sounds_super_admin ON sounds AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY sounds_tenant ON sounds AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON sounds TO app_user;
GRANT USAGE, SELECT ON SEQUENCE sounds_id_seq TO app_user;

CREATE TABLE scripts (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    type        TEXT NOT NULL CHECK (type IN ('press1', 'broadcast', 'survey', 'custom')),
    body        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX scripts_tenant_idx ON scripts (tenant_id);

ALTER TABLE scripts ENABLE ROW LEVEL SECURITY;

CREATE POLICY scripts_super_admin ON scripts AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY scripts_tenant ON scripts AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON scripts TO app_user;
GRANT USAGE, SELECT ON SEQUENCE scripts_id_seq TO app_user;

-- +goose Down
DROP TABLE IF EXISTS scripts;
DROP TABLE IF EXISTS sounds;
