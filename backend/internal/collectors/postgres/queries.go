package postgres

import (
	_ "embed"
)

//go:embed ../../../sql/collectors/postgres/server.sql
var serverSQL string

//go:embed ../../../sql/collectors/postgres/databases.sql
var databasesSQL string

//go:embed ../../../sql/collectors/postgres/schemas.sql
var schemasSQL string
