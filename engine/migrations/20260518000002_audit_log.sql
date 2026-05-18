-- +goose Up
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    ts          TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_id  TEXT,
    actor_type  TEXT NOT NULL CHECK (actor_type IN ('user','system','dialer','migration')),
    actor_id    TEXT,
    tenant_id   BIGINT,
    entity_type TEXT NOT NULL,
    entity_id   TEXT,
    action      TEXT NOT NULL,
    before_data JSONB,
    after_data  JSONB,
    ip          TEXT,
    user_agent  TEXT
);
CREATE INDEX audit_ts_idx ON audit_log (ts DESC);
CREATE INDEX audit_tenant_ts_idx ON audit_log (tenant_id, ts DESC) WHERE tenant_id IS NOT NULL;
CREATE INDEX audit_entity_idx ON audit_log (entity_type, entity_id);

ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY audit_super_admin ON audit_log AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY audit_tenant_read ON audit_log AS PERMISSIVE FOR SELECT
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

CREATE POLICY audit_tenant_insert ON audit_log AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT ON audit_log TO app_user;
GRANT USAGE, SELECT ON SEQUENCE audit_log_id_seq TO app_user;

REVOKE UPDATE, DELETE ON audit_log FROM PUBLIC;
REVOKE UPDATE, DELETE ON audit_log FROM app_user;

-- +goose Down
DROP TABLE audit_log;
