-- Preserve provenance of the later, comparable diagnostic used to validate an action.
ALTER TABLE quality_issue ADD COLUMN IF NOT EXISTS validation_scan_id uuid REFERENCES quality_scan(id);
ALTER TABLE quality_issue_event ADD COLUMN IF NOT EXISTS validation_scan_id uuid REFERENCES quality_scan(id);
