-- +goose Up
CREATE TABLE tenants (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    sip_domain  TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    email         TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('super_admin','tenant_owner','tenant_admin','campaign_manager','agent','viewer')),
    password_hash TEXT NOT NULL,
    totp_secret   TEXT,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended')),
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email),
    CHECK ((role = 'super_admin') = (tenant_id IS NULL))
);
CREATE INDEX users_tenant_idx ON users (tenant_id);

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE users   ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenants_super_admin ON tenants AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');
CREATE POLICY tenants_self ON tenants AS PERMISSIVE FOR ALL
    USING (id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

CREATE POLICY users_super_admin ON users AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');
CREATE POLICY users_tenant ON users AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON tenants, users TO app_user;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user;

-- +goose Down
DROP TABLE users;
DROP TABLE tenants;
