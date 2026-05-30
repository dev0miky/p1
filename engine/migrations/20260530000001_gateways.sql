-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE gateways (
    id                 BIGSERIAL PRIMARY KEY,
    name               TEXT NOT NULL UNIQUE CHECK (name ~ '^[a-z0-9_-]{1,64}$'),
    description        TEXT,
    proxy              TEXT NOT NULL CHECK (length(trim(proxy)) > 0),
    register           BOOLEAN NOT NULL DEFAULT true,
    username           TEXT,
    password_enc       BYTEA,
    realm              TEXT,
    from_user          TEXT,
    from_domain        TEXT,
    transport          TEXT NOT NULL DEFAULT 'udp' CHECK (transport IN ('udp','tcp','tls')),
    expire_seconds     INT NOT NULL DEFAULT 3600 CHECK (expire_seconds > 0),
    retry_seconds      INT NOT NULL DEFAULT 30 CHECK (retry_seconds > 0),
    caller_id_in_from  BOOLEAN NOT NULL DEFAULT true,
    extra_params       JSONB NOT NULL DEFAULT '{}',
    enabled            BOOLEAN NOT NULL DEFAULT true,
    is_active          BOOLEAN NOT NULL DEFAULT false,
    register_status    TEXT NOT NULL DEFAULT 'unknown' CHECK (register_status IN ('unknown','registered','trying','failed','noreg','down')),
    register_status_at TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT gateways_active_requires_enabled CHECK (NOT is_active OR enabled)
);

CREATE UNIQUE INDEX gateways_one_active ON gateways ((is_active)) WHERE is_active;

ALTER TABLE gateways ENABLE ROW LEVEL SECURITY;

CREATE POLICY gateways_super_admin ON gateways AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin')
    WITH CHECK (current_setting('app.role', true) = 'super_admin');

GRANT SELECT, INSERT, UPDATE, DELETE ON gateways TO app_user;
GRANT USAGE, SELECT ON SEQUENCE gateways_id_seq TO app_user;

-- +goose Down
DROP TABLE IF EXISTS gateways;
