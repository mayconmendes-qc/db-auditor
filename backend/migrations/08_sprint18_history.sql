-- Sprint 18: explicit baselines and immutable per-run finding observations.
ALTER TABLE analysis_run ADD COLUMN rule_manifest_hash text NOT NULL DEFAULT '';

CREATE TABLE audit_baseline (
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  table_name text NOT NULL DEFAULT '',
  audit_run_id uuid NOT NULL REFERENCES audit_run(id),
  selected_at timestamptz NOT NULL DEFAULT now(),
  selected_by text NOT NULL DEFAULT 'local',
  PRIMARY KEY (environment_id, database_name, schema_name, table_name)
);

CREATE TABLE baseline_selection_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  table_name text NOT NULL DEFAULT '',
  previous_run_id uuid REFERENCES audit_run(id),
  selected_run_id uuid NOT NULL REFERENCES audit_run(id),
  selected_by text NOT NULL DEFAULT 'local',
  selected_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE baseline_comparison (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  table_name text NOT NULL DEFAULT '',
  baseline_run_id uuid NOT NULL REFERENCES audit_run(id),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('complete','partial','incompatible')),
  added_tables integer NOT NULL DEFAULT 0,
  removed_tables integer NOT NULL DEFAULT 0,
  changed_tables integer NOT NULL DEFAULT 0,
  compared_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name)
);

ALTER TABLE finding ADD COLUMN recurrence_count integer NOT NULL DEFAULT 0;
ALTER TABLE finding ADD COLUMN suppression_reason text;
ALTER TABLE finding ADD COLUMN suppressed_until timestamptz;
ALTER TABLE finding ADD COLUMN superseded_by uuid REFERENCES finding(id);

CREATE TABLE finding_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  finding_id uuid NOT NULL REFERENCES finding(id) ON DELETE CASCADE,
  audit_run_id uuid REFERENCES audit_run(id) ON DELETE CASCADE,
  event_type text NOT NULL CHECK (event_type IN ('observed','resolved','reopened','suppressed','acknowledged','superseded')),
  reason text NOT NULL DEFAULT '',
  category text NOT NULL DEFAULT '',
  severity text NOT NULL DEFAULT '',
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  object_name text NOT NULL DEFAULT '',
  rule_version text NOT NULL DEFAULT '',
  title text NOT NULL DEFAULT '',
  summary text NOT NULL DEFAULT '',
  recommendation text NOT NULL DEFAULT '',
  finding_status text NOT NULL DEFAULT 'open',
  impact text NOT NULL DEFAULT '',
  risk text NOT NULL DEFAULT '',
  validation text NOT NULL DEFAULT '',
  reference_urls jsonb NOT NULL DEFAULT '[]'::jsonb,
  rule_parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  confidence numeric(4,3) NOT NULL DEFAULT 0,
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (finding_id, audit_run_id, event_type)
);
CREATE INDEX finding_event_timeline_idx ON finding_event (finding_id, recorded_at, id);

-- Existing installations only know the most recent observation of each
-- deduplicated finding. Preserve that one without inventing older history.
INSERT INTO finding_event(finding_id,audit_run_id,event_type,reason,category,severity,database_name,schema_name,object_name,rule_version,title,summary,recommendation,confidence,evidence,finding_status,impact,risk,validation,reference_urls,rule_parameters,recorded_at)
SELECT id,audit_run_id,'observed','migrated latest observation; older history unavailable',category,severity,database_name,schema_name,object_name,rule_version,title,summary,recommendation,confidence,evidence,status,impact,risk,validation,reference_urls,rule_parameters,last_seen_at
FROM finding WHERE audit_run_id IS NOT NULL;

-- The old unique key mixed different versions of one rule. Keep each version
-- independently so a new rule can supersede, not silently rewrite, history.
DROP INDEX finding_dedup_idx;
CREATE UNIQUE INDEX finding_dedup_version_idx ON finding (environment_id, dedup_key, rule_version);
