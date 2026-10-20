-- PII scanner (spec 05 RF-01..RF-07, ADR-0001 D4/D14/D11).
-- Never store sampled values — only counts, proportions and classifications.

CREATE TABLE IF NOT EXISTS pii_dictionary_entry (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  name_pattern text NOT NULL CHECK (char_length(btrim(name_pattern)) BETWEEN 1 AND 128),
  category text NOT NULL CHECK (category IN (
    'cpf','cnpj','email','telefone','cep','pix','nome','nascimento','endereco','rg'
  )),
  active boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text NOT NULL DEFAULT '',
  UNIQUE (environment_id, name_pattern)
);

CREATE INDEX IF NOT EXISTS pii_dictionary_entry_env_idx
  ON pii_dictionary_entry (environment_id, active);

CREATE TABLE IF NOT EXISTS pii_environment_setting (
  environment_id uuid PRIMARY KEY REFERENCES audit_environment(id) ON DELETE CASCADE,
  content_scan_enabled boolean NOT NULL DEFAULT false,
  enabled_by text NOT NULL DEFAULT '',
  enabled_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pii_scan (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  audit_run_id uuid REFERENCES audit_run(id) ON DELETE SET NULL,
  stages int[] NOT NULL DEFAULT '{1}',
  status text NOT NULL DEFAULT 'running'
    CHECK (status IN ('running','success','partial_success','failed')),
  coverage_note text NOT NULL DEFAULT '',
  missing_objects text[] NOT NULL DEFAULT '{}',
  columns_analyzed int NOT NULL DEFAULT 0,
  tables_analyzed int NOT NULL DEFAULT 0,
  content_columns int NOT NULL DEFAULT 0,
  actor text NOT NULL DEFAULT '',
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  error text NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS pii_scan_env_idx
  ON pii_scan (environment_id, started_at DESC);

CREATE TABLE IF NOT EXISTS pii_column_result (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scan_id uuid NOT NULL REFERENCES pii_scan(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  column_name text NOT NULL,
  data_type text NOT NULL DEFAULT '',
  category text NOT NULL,
  confidence text NOT NULL CHECK (confidence IN ('alta','media','baixa')),
  method text NOT NULL CHECK (method IN ('nome','conteudo','ambos','nao_verificado')),
  sample_size int NOT NULL DEFAULT 0,
  valid_count int NOT NULL DEFAULT 0,
  non_null_count int NOT NULL DEFAULT 0,
  valid_percent double precision NOT NULL DEFAULT 0,
  null_fraction double precision,
  read_accounts int NOT NULL DEFAULT 0,
  read_account_names text[] NOT NULL DEFAULT '{}',
  has_rls boolean NOT NULL DEFAULT false,
  access_level text NOT NULL DEFAULT 'nivel_tabela'
    CHECK (access_level IN ('nivel_tabela','nivel_coluna')),
  coverage_note text NOT NULL DEFAULT '',
  UNIQUE (scan_id, database_name, schema_name, table_name, column_name)
);

CREATE INDEX IF NOT EXISTS pii_column_result_env_idx
  ON pii_column_result (environment_id, scan_id, category, confidence);

CREATE TABLE IF NOT EXISTS pii_classification_review (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  column_name text NOT NULL,
  decision text NOT NULL CHECK (decision IN ('confirmed','discarded')),
  justification text NOT NULL DEFAULT '',
  evidence_sha256 text NOT NULL DEFAULT '',
  decided_by text NOT NULL,
  decided_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, database_name, schema_name, table_name, column_name)
);

CREATE INDEX IF NOT EXISTS pii_classification_review_env_idx
  ON pii_classification_review (environment_id, decision);
