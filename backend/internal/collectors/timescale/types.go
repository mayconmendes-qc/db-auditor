// Package timescale implements read-only TimescaleDB inventory collectors.
// Collectors emit facts only; analyzers interpret facts elsewhere.
package timescale

import "time"

const CollectorVersion = "1.0.0"

// CollectionStatus describes whether a Timescale collector ran or was skipped.
type CollectionStatus string

const (
	StatusOK                 CollectionStatus = "OK"
	StatusSkippedUnsupported CollectionStatus = "SKIPPED_UNSUPPORTED"
	StatusSkippedMissing     CollectionStatus = "SKIPPED_MISSING"
	StatusError              CollectionStatus = "ERROR"
)

// VersionFacts is the output of the Timescale version collector.
type VersionFacts struct {
	DatabaseName      string `json:"database_name"`
	ExtensionName     string `json:"extension_name"`
	ExtensionVersion  string `json:"extension_version"`
	SchemaName        string `json:"schema_name"`
	Major             int    `json:"major"`
	Minor             int    `json:"minor"`
	Patch             int    `json:"patch"`
	Compatible        bool   `json:"compatible"`
	CompatibilityNote string `json:"compatibility_note,omitempty"`
}

// HypertableFacts is one row from the hypertable collector.
type HypertableFacts struct {
	DatabaseName       string `json:"database_name"`
	SchemaName         string `json:"schema_name"`
	HypertableName     string `json:"hypertable_name"`
	Owner              string `json:"owner_name"`
	NumDimensions      int    `json:"num_dimensions"`
	NumChunks          int    `json:"num_chunks"`
	CompressionEnabled bool   `json:"compression_enabled"`
	IsDistributed      bool   `json:"is_distributed"`
	TotalSizeBytes     int64  `json:"total_size_bytes"`
	DataSizeBytes      int64  `json:"data_size_bytes"`
	IndexSizeBytes     int64  `json:"index_size_bytes"`
}

// DimensionFacts is one row from the dimension collector.
type DimensionFacts struct {
	DatabaseName     string  `json:"database_name"`
	SchemaName       string  `json:"schema_name"`
	HypertableName   string  `json:"hypertable_name"`
	DimensionNumber  int     `json:"dimension_number"`
	ColumnName       string  `json:"column_name"`
	ColumnType       string  `json:"column_type"`
	DimensionType    string  `json:"dimension_type"`
	TimeInterval     *string `json:"time_interval,omitempty"`
	IntegerInterval  *string `json:"integer_interval,omitempty"`
	IntegerNowFunc   *string `json:"integer_now_func,omitempty"`
	NumSlices        *int    `json:"num_slices,omitempty"`
	PartitioningFunc *string `json:"partitioning_func,omitempty"`
}

// ChunkFacts is one row from the chunk collector.
type ChunkFacts struct {
	DatabaseName      string     `json:"database_name"`
	SchemaName        string     `json:"schema_name"`
	HypertableName    string     `json:"hypertable_name"`
	ChunkSchema       string     `json:"chunk_schema"`
	ChunkName         string     `json:"chunk_name"`
	RangeStart        *time.Time `json:"range_start,omitempty"`
	RangeEnd          *time.Time `json:"range_end,omitempty"`
	RangeStartInteger *int64     `json:"range_start_integer,omitempty"`
	RangeEndInteger   *int64     `json:"range_end_integer,omitempty"`
	IsCompressed      bool       `json:"is_compressed"`
	ChunkTablespace   string     `json:"chunk_tablespace"`
	TotalSizeBytes    int64      `json:"total_size_bytes"`
	DataSizeBytes     int64      `json:"data_size_bytes"`
	IndexSizeBytes    int64      `json:"index_size_bytes"`
}

// InventoryResult bundles Timescale facts for one database collection pass.
type InventoryResult struct {
	Status      CollectionStatus  `json:"status"`
	Message     string            `json:"message,omitempty"`
	Version     *VersionFacts     `json:"version,omitempty"`
	Hypertables []HypertableFacts `json:"hypertables,omitempty"`
	Dimensions  []DimensionFacts  `json:"dimensions,omitempty"`
	Chunks      []ChunkFacts      `json:"chunks,omitempty"`
}
