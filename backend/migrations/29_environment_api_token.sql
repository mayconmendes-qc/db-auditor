-- Environment-scoped API tokens for CLI principals (ADR-0001 D1).
-- Stores only sha256 of the secret; plaintext is returned once at creation.

CREATE TABLE IF NOT EXISTS environment_api_token (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 128),
  public_prefix text NOT NULL CHECK (public_prefix ~ '^[a-z0-9]{6,16}$'),
  token_sha256 bytea NOT NULL UNIQUE,
  scopes text[] NOT NULL,
  created_by uuid REFERENCES auditor_user(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  last_used_at timestamptz,
  CONSTRAINT environment_api_token_scopes_known CHECK (
    scopes <> '{}' AND scopes <@ ARRAY[
      'snapshot-context:read',
      'migration-analysis:write',
      'deploy-annotation:write',
      'quality-contract:run',
      'restore-evidence:write',
      'last-login:write'
    ]::text[]
  ),
  CONSTRAINT environment_api_token_expires_after_created CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS environment_api_token_env_idx
  ON environment_api_token (environment_id, created_at DESC);

CREATE INDEX IF NOT EXISTS environment_api_token_prefix_idx
  ON environment_api_token (public_prefix);
