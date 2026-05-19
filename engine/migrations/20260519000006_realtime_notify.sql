-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_tenant_event() RETURNS TRIGGER AS $$
DECLARE
  payload JSONB;
BEGIN
  IF TG_TABLE_NAME = 'call_events' THEN
    payload := jsonb_build_object(
      'tenant_id', NEW.tenant_id,
      'type', 'call.event',
      'call_uuid', NEW.call_uuid::text,
      'from_state', NEW.from_state,
      'to_state', NEW.to_state,
      'reason', NEW.reason,
      'at', NEW.at
    );
  ELSIF TG_TABLE_NAME = 'lead_import_jobs' THEN
    payload := jsonb_build_object(
      'tenant_id', NEW.tenant_id,
      'type', 'import.progress',
      'job_id', NEW.id,
      'list_id', NEW.list_id,
      'status', NEW.status,
      'total_rows', NEW.total_rows,
      'processed_rows', NEW.processed_rows,
      'error_rows', NEW.error_rows,
      'csv_filename', NEW.csv_filename
    );
  ELSIF TG_TABLE_NAME = 'campaigns' THEN
    payload := jsonb_build_object(
      'tenant_id', NEW.tenant_id,
      'type', 'campaign.status',
      'campaign_id', NEW.id,
      'name', NEW.name,
      'status', NEW.status,
      'run_no', NEW.run_no
    );
  ELSE
    RETURN NEW;
  END IF;
  -- pg_notify payloads are capped at 8 kB. Our payloads are tiny so we're fine.
  PERFORM pg_notify('tenant_events', payload::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER call_events_notify_trg
  AFTER INSERT ON call_events
  FOR EACH ROW EXECUTE FUNCTION notify_tenant_event();

CREATE TRIGGER lead_import_jobs_notify_trg
  AFTER INSERT OR UPDATE OF status, processed_rows, error_rows, total_rows ON lead_import_jobs
  FOR EACH ROW EXECUTE FUNCTION notify_tenant_event();

CREATE TRIGGER campaigns_notify_trg
  AFTER UPDATE OF status ON campaigns
  FOR EACH ROW
  WHEN (OLD.status IS DISTINCT FROM NEW.status)
  EXECUTE FUNCTION notify_tenant_event();

-- +goose Down
DROP TRIGGER IF EXISTS campaigns_notify_trg ON campaigns;
DROP TRIGGER IF EXISTS lead_import_jobs_notify_trg ON lead_import_jobs;
DROP TRIGGER IF EXISTS call_events_notify_trg ON call_events;
DROP FUNCTION IF EXISTS notify_tenant_event();
