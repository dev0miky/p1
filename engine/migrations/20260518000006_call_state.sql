-- +goose Up
CREATE TABLE call_state (
    uuid           UUID PRIMARY KEY,
    tenant_id      BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    campaign_id    BIGINT REFERENCES campaigns(id) ON DELETE SET NULL,
    lead_id        BIGINT REFERENCES leads(id) ON DELETE SET NULL,
    state          TEXT NOT NULL,
    version        INT NOT NULL DEFAULT 1 CHECK (version > 0),
    dialed_number  TEXT NOT NULL,
    caller_id      TEXT,
    amd_result     TEXT,
    dtmf           TEXT,
    hangup_cause   TEXT,
    locked_by      TEXT,
    metadata       JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at    TIMESTAMPTZ,
    bridged_at     TIMESTAMPTZ,
    ended_at       TIMESTAMPTZ,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX call_state_tenant_started_idx ON call_state (tenant_id, started_at DESC);
CREATE INDEX call_state_campaign_started_idx ON call_state (campaign_id, started_at DESC) WHERE campaign_id IS NOT NULL;
CREATE INDEX call_state_active_idx ON call_state (state)
    WHERE state NOT IN ('completed','failed','no_answer','busy','voicemail','opt_out');

CREATE TABLE call_events (
    id          BIGSERIAL PRIMARY KEY,
    call_uuid   UUID NOT NULL REFERENCES call_state(uuid) ON DELETE CASCADE,
    tenant_id   BIGINT NOT NULL,
    from_state  TEXT,
    to_state    TEXT NOT NULL,
    reason      TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX call_events_uuid_idx ON call_events (call_uuid, at);
CREATE INDEX call_events_tenant_at_idx ON call_events (tenant_id, at DESC);

ALTER TABLE call_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE call_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY call_state_super_admin ON call_state AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');
CREATE POLICY call_state_tenant ON call_state AS PERMISSIVE FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

CREATE POLICY call_events_super_admin ON call_events AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');
CREATE POLICY call_events_tenant_read ON call_events AS PERMISSIVE FOR SELECT
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);
CREATE POLICY call_events_tenant_insert ON call_events AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE ON call_state TO app_user;
GRANT SELECT, INSERT ON call_events TO app_user;
GRANT USAGE, SELECT ON SEQUENCE call_events_id_seq TO app_user;

REVOKE DELETE ON call_state, call_events FROM app_user;
REVOKE UPDATE ON call_events FROM app_user;

-- +goose Down
DROP TABLE call_events;
DROP TABLE call_state;
