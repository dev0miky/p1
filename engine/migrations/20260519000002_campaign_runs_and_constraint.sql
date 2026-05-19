-- +goose Up
ALTER TABLE campaigns
    ADD COLUMN run_no INT NOT NULL DEFAULT 0 CHECK (run_no >= 0),
    ADD COLUMN call_constraint TEXT NOT NULL DEFAULT 'no_constraint' CHECK (call_constraint IN (
        'no_constraint',
        'only_answered',
        'only_human_answered',
        'only_machine_answered',
        'only_failed_transfers',
        'only_transfers',
        'only_successful_transfers',
        'only_errors',
        'skip_answered',
        'skip_human_answered',
        'skip_machine_answered',
        'skip_successful_transfers',
        'skip_errors'
    ));

COMMENT ON COLUMN campaigns.run_no IS 'incremented every time the campaign transitions paused -> active. call_state.campaign_run_no records which run a given call belongs to.';
COMMENT ON COLUMN campaigns.call_constraint IS 'filter applied at lead claim time to skip leads based on their counter state (e.g. only redial machine-answered).';

ALTER TABLE call_state
    ADD COLUMN campaign_run_no INT NOT NULL DEFAULT 0 CHECK (campaign_run_no >= 0);

CREATE INDEX call_state_campaign_run_idx ON call_state (campaign_id, campaign_run_no DESC) WHERE campaign_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS call_state_campaign_run_idx;
ALTER TABLE call_state DROP COLUMN campaign_run_no;
ALTER TABLE campaigns DROP COLUMN run_no, DROP COLUMN call_constraint;
