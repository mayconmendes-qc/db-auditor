// Package postgres implements read-only PostgreSQL inventory collectors.
// Collectors emit facts only; analyzers interpret facts elsewhere.
package postgres

import "time"

const CollectorVersion = "1.0.0"

// ServerFacts is the output of the server collector.
type ServerFacts struct {
	ServerVersion      string    `json:"server_version"`
	StartedAt          time.Time `json:"started_at"`
	Uptime             string    `json:"uptime"`
	Timezone           string    `json:"timezone"`
	ServerEncoding     string    `json:"server_encoding"`
	MaxConnections     int       `json:"max_connections"`
	CurrentConnections int       `json:"current_connections"`
	Autovacuum         string    `json:"autovacuum"`
}

// DatabaseFacts is one row from the database collector.
type DatabaseFacts struct {
	Name             string `json:"database_name"`
	Owner            string `json:"owner_name"`
	Encoding         string `json:"encoding"`
	Collate          string `json:"collate_name"`
	CType            string `json:"ctype_name"`
	AllowConnections bool   `json:"allow_connections"`
	IsTemplate       bool   `json:"is_template"`
	SizeBytes        int64  `json:"size_bytes"`
	ConnectionCount  int    `json:"connection_count"`
}

// SchemaFacts is one row from the schema collector (current database).
type SchemaFacts struct {
	DatabaseName          string `json:"database_name"`
	SchemaName            string `json:"schema_name"`
	Owner                 string `json:"owner_name"`
	TableCount            int    `json:"table_count"`
	ViewCount             int    `json:"view_count"`
	MaterializedViewCount int    `json:"materialized_view_count"`
	SequenceCount         int    `json:"sequence_count"`
	FunctionCount         int    `json:"function_count"`
	SizeBytes             int64  `json:"size_bytes"`
}

// PartialError records a non-fatal failure for one database during discovery.
type PartialError struct {
	Database string `json:"database"`
	Op       string `json:"op"`
	Message  string `json:"error"`
}
