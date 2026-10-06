-- name: CreateEnvironment :one
INSERT INTO audit_environment (name, type, discovery_mode, active)
VALUES ($1, $2, $3, $4)
RETURNING id, name, type, discovery_mode, active, created_at, updated_at, expects_replica, engine;

-- name: GetEnvironmentByID :one
SELECT id, name, type, discovery_mode, active, created_at, updated_at, expects_replica, engine
FROM audit_environment
WHERE id = $1;

-- name: ListEnvironments :many
SELECT id, name, type, discovery_mode, active, created_at, updated_at, expects_replica, engine
FROM audit_environment
ORDER BY name;

-- name: ListActiveEnvironments :many
SELECT id, name, type, discovery_mode, active, created_at, updated_at, expects_replica, engine
FROM audit_environment
WHERE active = true
ORDER BY name;
