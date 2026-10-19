-- Access review campaigns (spec 07 RF-01..RF-09, ADR-0001 D7/D11).
-- Idempotent additive migration. Contact fields are PII: never log them.

CREATE TABLE IF NOT EXISTS access_review_owner (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  -- Target role or LOGIN account on the audited database.
  target_role text NOT NULL,
  owner_user_id uuid NOT NULL REFERENCES auditor_user(id),
  contact_name text NOT NULL DEFAULT '' CHECK (char_length(contact_name) <= 200),
  contact_email text NOT NULL DEFAULT '' CHECK (char_length(contact_email) <= 254),
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text NOT NULL DEFAULT '',
  UNIQUE (environment_id, target_role)
);

CREATE INDEX IF NOT EXISTS access_review_owner_env_idx
  ON access_review_owner (environment_id, owner_user_id);

CREATE TABLE IF NOT EXISTS access_review_campaign (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
  -- Scope: databases and schemas included when the matrix was frozen.
  scope jsonb NOT NULL DEFAULT '{}'::jsonb,
  reference_run_id uuid NOT NULL REFERENCES audit_run(id),
  due_at timestamptz NOT NULL,
  status text NOT NULL DEFAULT 'open'
    CHECK (status IN ('open', 'in_review', 'closed', 'overdue')),
  created_by text NOT NULL,
  created_by_user_id uuid REFERENCES auditor_user(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz,
  closed_by text NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS access_review_campaign_env_idx
  ON access_review_campaign (environment_id, created_at DESC);

CREATE TABLE IF NOT EXISTS access_review_item (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id uuid NOT NULL REFERENCES access_review_campaign(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  account_name text NOT NULL,
  object_type text NOT NULL,
  object_name text NOT NULL,
  column_name text NOT NULL DEFAULT '',
  privilege text NOT NULL,
  origin_kind text NOT NULL,
  via_chain text[] NOT NULL DEFAULT '{}',
  origin_label text NOT NULL DEFAULT '',
  -- RF-04 highlights (frozen at campaign create).
  highlights text[] NOT NULL DEFAULT '{}',
  owner_user_id uuid REFERENCES auditor_user(id) ON DELETE SET NULL,
  UNIQUE (campaign_id, database_name, account_name, object_type, object_name, column_name, privilege)
);

CREATE INDEX IF NOT EXISTS access_review_item_campaign_idx
  ON access_review_item (campaign_id, account_name, object_name);
CREATE INDEX IF NOT EXISTS access_review_item_owner_idx
  ON access_review_item (campaign_id, owner_user_id);
CREATE INDEX IF NOT EXISTS access_review_item_env_idx
  ON access_review_item (environment_id, campaign_id);

CREATE TABLE IF NOT EXISTS access_review_decision (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id uuid NOT NULL REFERENCES access_review_campaign(id) ON DELETE CASCADE,
  item_id uuid NOT NULL REFERENCES access_review_item(id) ON DELETE CASCADE,
  decision text NOT NULL CHECK (decision IN ('approve', 'revoke', 'keep')),
  revoke_due_at timestamptz,
  justification text NOT NULL DEFAULT '',
  decided_by text NOT NULL,
  decided_by_user_id uuid REFERENCES auditor_user(id) ON DELETE SET NULL,
  decided_at timestamptz NOT NULL DEFAULT now(),
  supersedes_id uuid REFERENCES access_review_decision(id),
  finding_action_id uuid,
  -- Latest decision per item is the one without a newer superseding row.
  CONSTRAINT access_review_decision_keep_needs_justification
    CHECK (decision <> 'keep' OR char_length(btrim(justification)) > 0)
);

CREATE INDEX IF NOT EXISTS access_review_decision_item_idx
  ON access_review_decision (item_id, decided_at DESC);
CREATE INDEX IF NOT EXISTS access_review_decision_campaign_idx
  ON access_review_decision (campaign_id, decided_at DESC);

ALTER TABLE report_job DROP CONSTRAINT IF EXISTS report_job_report_type_check;
ALTER TABLE report_job ADD CONSTRAINT report_job_report_type_check
  CHECK (report_type IN ('executive','technical','table','architecture','access_review'));
