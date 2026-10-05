-- Inventory filters use empty-string sentinels. Callers never concatenate SQL.

-- name: CountTableSnapshots :one
SELECT COUNT(*)::bigint
FROM table_snapshot
WHERE environment_id = $1::uuid
  AND audit_run_id = COALESCE(
    NULLIF($2, '')::uuid,
    (SELECT id FROM audit_run
     WHERE environment_id = $1::uuid
       AND status IN ('success', 'partial_success')
     ORDER BY started_at DESC, id DESC
     LIMIT 1)
  )
  AND ($3 = '' OR database_name = $3)
  AND ($4 = '' OR schema_name = $4)
  AND ($5 = '' OR table_name = $5)
  AND ($6 = '' OR relation_class = $6)
  AND (
    $7 = ''
    OR table_name ILIKE $7
    OR schema_name ILIKE $7
    OR database_name ILIKE $7
  );

-- name: ListTableSnapshotsFiltered :many
SELECT id::text, audit_run_id::text, environment_id::text, database_name, schema_name, table_name,
  owner_name, COALESCE(relkind, ''), COALESCE(relation_class, ''), COALESCE(is_partition, false),
  total_size_bytes, data_size_bytes, index_size_bytes,
  COALESCE(row_estimate, 0), COALESCE(column_count, 0), COALESCE(has_primary_key, false), collected_at
FROM table_snapshot
WHERE environment_id = $1::uuid
  AND audit_run_id = COALESCE(
    NULLIF($2, '')::uuid,
    (SELECT id FROM audit_run
     WHERE environment_id = $1::uuid
       AND status IN ('success', 'partial_success')
     ORDER BY started_at DESC, id DESC
     LIMIT 1)
  )
  AND ($3 = '' OR database_name = $3)
  AND ($4 = '' OR schema_name = $4)
  AND ($5 = '' OR table_name = $5)
  AND ($6 = '' OR relation_class = $6)
  AND (
    $7 = ''
    OR table_name ILIKE $7
    OR schema_name ILIKE $7
    OR database_name ILIKE $7
  )
ORDER BY database_name, schema_name, table_name, id
LIMIT $8 OFFSET $9;
