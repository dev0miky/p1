-- +goose Up

-- Drop the embedded reference columns. None are read by the dialer today;
-- the audit (PLAN.md §1) confirmed prompt_audio is unused, caller_id_pool is
-- hardcoded as '0000000000' at originate time, and dnc_list_ids never read.
ALTER TABLE campaigns
    DROP COLUMN prompt_audio,
    DROP COLUMN transfer_dest,
    DROP COLUMN caller_id_pool,
    DROP COLUMN dnc_list_ids;

-- campaign_sounds: a campaign attaches sounds for specific roles. A single
-- sound can play in multiple roles within the same campaign (e.g. a tone
-- used both as hold music and whisper). Roles enumerated in the CHECK.
CREATE TABLE campaign_sounds (
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    sound_id    BIGINT NOT NULL REFERENCES sounds(id) ON DELETE RESTRICT,
    role        TEXT NOT NULL CHECK (role IN ('greeting', 'voicemail', 'hold', 'whisper', 'opt_out_confirm')),
    attached_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, sound_id, role)
);

CREATE INDEX campaign_sounds_sound_idx ON campaign_sounds (sound_id);

ALTER TABLE campaign_sounds ENABLE ROW LEVEL SECURITY;

CREATE POLICY campaign_sounds_super_admin ON campaign_sounds AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY campaign_sounds_tenant ON campaign_sounds AS PERMISSIVE FOR ALL
    USING (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    )
    WITH CHECK (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    );

GRANT SELECT, INSERT, UPDATE, DELETE ON campaign_sounds TO app_user;

-- campaign_scripts: usually 1:1 with a campaign but the table allows N:1
-- in case future variants want to A/B between scripts.
CREATE TABLE campaign_scripts (
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    script_id   BIGINT NOT NULL REFERENCES scripts(id) ON DELETE RESTRICT,
    attached_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, script_id)
);

CREATE INDEX campaign_scripts_script_idx ON campaign_scripts (script_id);

ALTER TABLE campaign_scripts ENABLE ROW LEVEL SECURITY;

CREATE POLICY campaign_scripts_super_admin ON campaign_scripts AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY campaign_scripts_tenant ON campaign_scripts AS PERMISSIVE FOR ALL
    USING (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    )
    WITH CHECK (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    );

GRANT SELECT, INSERT, UPDATE, DELETE ON campaign_scripts TO app_user;

-- campaign_lists: many-to-many between campaigns and lead_lists.
-- Attaching a list to a campaign also updates the leads.campaign_id of
-- every lead in that list — handled in the repo, not via trigger, so the
-- update stays inside the same tenant tx context.
CREATE TABLE campaign_lists (
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    list_id     BIGINT NOT NULL REFERENCES lead_lists(id) ON DELETE CASCADE,
    attached_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, list_id)
);

CREATE INDEX campaign_lists_list_idx ON campaign_lists (list_id);

ALTER TABLE campaign_lists ENABLE ROW LEVEL SECURITY;

CREATE POLICY campaign_lists_super_admin ON campaign_lists AS PERMISSIVE FOR ALL
    USING (current_setting('app.role', true) = 'super_admin');

CREATE POLICY campaign_lists_tenant ON campaign_lists AS PERMISSIVE FOR ALL
    USING (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    )
    WITH CHECK (
        EXISTS (SELECT 1 FROM campaigns c WHERE c.id = campaign_id
                AND c.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint)
    );

GRANT SELECT, INSERT, UPDATE, DELETE ON campaign_lists TO app_user;

-- +goose Down
DROP TABLE IF EXISTS campaign_lists;
DROP TABLE IF EXISTS campaign_scripts;
DROP TABLE IF EXISTS campaign_sounds;

ALTER TABLE campaigns
    ADD COLUMN prompt_audio    TEXT,
    ADD COLUMN transfer_dest   TEXT,
    ADD COLUMN caller_id_pool  JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN dnc_list_ids    BIGINT[] NOT NULL DEFAULT '{}';
