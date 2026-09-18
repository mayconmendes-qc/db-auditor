-- Demo environments for local make up (fixed UUIDs used by UI analyzer demos).
-- Applied only on empty volume via docker-entrypoint-initdb.d.

INSERT INTO audit_environment (id, name, type, discovery_mode, active)
VALUES
  (
    '00000000-0000-0000-0000-000000000001',
    'Demo Tiger Cloud',
    'tiger_cloud',
    'single_database',
    true
  ),
  (
    '00000000-0000-0000-0000-000000000002',
    'Demo Datacenter',
    'self_hosted',
    'multi_database',
    true
  )
ON CONFLICT (id) DO NOTHING;
