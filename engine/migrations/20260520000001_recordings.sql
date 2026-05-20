-- +goose Up
CREATE TABLE recordings (
    id             BIGSERIAL PRIMARY KEY,
    tenant_id      BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    call_uuid      TEXT NOT NULL,
    campaign_id    BIGINT REFERENCES campaigns(id) ON DELETE SET NULL,
    lead_id        BIGINT REFERENCES leads(id) ON DELETE SET NULL,
    file_key       TEXT NOT NULL,
    sha256         TEXT NOT NULL,
    size_bytes     BIGINT NOT NULL CHECK (size_bytes >= 0),
    duration_ms    INT,
    retention_until TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, call_uuid)
);

CREATE INDEX recordings_tenant_idx ON recordings (tenant_id);
CREATE INDEX recordings_campaign_idx ON recordings (campaign_id);
CREATE INDEX recordings_lead_idx ON recordings (lead_id);

ALTER TABLE recordings ENABLE ROW LEVEL SECURITY;

CREATE POLICY recordings_super_admin ON recordings AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY recordings_tenant ON recordings AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON recordings TO app_user;
GRANT USAGE, SELECT ON SEQUENCE recordings_id_seq TO app_user;

-- +goose Down
DROP TABLE IF EXISTS recordings;
