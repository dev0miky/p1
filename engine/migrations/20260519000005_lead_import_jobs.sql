-- +goose Up
CREATE TABLE lead_import_jobs (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    list_id         BIGINT REFERENCES lead_lists(id) ON DELETE SET NULL,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'aborted')),
    csv_filename    TEXT NOT NULL,
    file_key        TEXT NOT NULL,
    column_map      JSONB NOT NULL DEFAULT '{}'::jsonb,
    total_rows      INT NOT NULL DEFAULT 0 CHECK (total_rows >= 0),
    processed_rows  INT NOT NULL DEFAULT 0 CHECK (processed_rows >= 0),
    error_rows      INT NOT NULL DEFAULT 0 CHECK (error_rows >= 0),
    last_error      TEXT,
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX lead_import_jobs_tenant_created_idx ON lead_import_jobs (tenant_id, created_at DESC);
CREATE INDEX lead_import_jobs_running_idx ON lead_import_jobs (status) WHERE status IN ('pending', 'running');

ALTER TABLE lead_import_jobs ENABLE ROW LEVEL SECURITY;

CREATE POLICY lead_import_jobs_super_admin ON lead_import_jobs AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY lead_import_jobs_tenant ON lead_import_jobs AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON lead_import_jobs TO app_user;
GRANT USAGE, SELECT ON SEQUENCE lead_import_jobs_id_seq TO app_user;

-- +goose Down
DROP TABLE IF EXISTS lead_import_jobs;
