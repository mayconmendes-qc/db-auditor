package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// HashPayload returns sha256 hex of a deterministic JSON encoding of v.
func HashPayload(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("fingerprint marshal: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// TableFingerprintInput is the structural material for a table fingerprint.
type TableFingerprintInput struct {
	Schema      string                   `json:"schema"`
	Name        string                   `json:"name"`
	Columns     []ColumnFingerprintPart  `json:"columns"`
	Constraints []string                 `json:"constraints"`
	Indexes     []string                 `json:"indexes"`
	Hypertable  *HypertableFingerprintPart `json:"hypertable,omitempty"`
}

// ColumnFingerprintPart is one column in a table fingerprint.
type ColumnFingerprintPart struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
}

// HypertableFingerprintPart captures Timescale-specific table config.
type HypertableFingerprintPart struct {
	NumDimensions       int      `json:"num_dimensions"`
	CompressionEnabled  bool     `json:"compression_enabled"`
	DimensionColumns    []string `json:"dimension_columns"`
	DimensionIntervals  []string `json:"dimension_intervals"`
}

// FunctionFingerprintInput is the structural material for functions/procedures.
type FunctionFingerprintInput struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	Args      string `json:"args"`
	Language  string `json:"language"`
	Kind      string `json:"kind"`
	Volatility string `json:"volatility"`
	SecurityDefiner bool `json:"security_definer"`
	Definition string `json:"definition"`
}

// ViewFingerprintInput is the structural material for views/matviews.
type ViewFingerprintInput struct {
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	Relkind    string `json:"relkind"`
	Definition string `json:"definition"`
}

// FingerprintResult is the hash plus algorithm metadata.
type FingerprintResult struct {
	Hash              string `json:"hash"`
	AlgorithmVersion  string `json:"algorithm_version"`
}

// TableFingerprint builds a stable hash for a table structure.
func TableFingerprint(in TableFingerprintInput) (FingerprintResult, error) {
	in.Schema = NormalizeName(in.Schema)
	in.Name = NormalizeName(in.Name)
	for i := range in.Columns {
		in.Columns[i].Name = NormalizeName(in.Columns[i].Name)
		in.Columns[i].Type = NormalizeType(in.Columns[i].Type)
		if in.Columns[i].Default != "" {
			in.Columns[i].Default = NormalizeSQL(in.Columns[i].Default)
		}
	}
	sort.Slice(in.Columns, func(i, j int) bool {
		return in.Columns[i].Position < in.Columns[j].Position
	})
	for i := range in.Constraints {
		in.Constraints[i] = NormalizeSQL(in.Constraints[i])
	}
	sort.Strings(in.Constraints)
	for i := range in.Indexes {
		in.Indexes[i] = NormalizeSQL(in.Indexes[i])
	}
	sort.Strings(in.Indexes)
	if in.Hypertable != nil {
		for i := range in.Hypertable.DimensionColumns {
			in.Hypertable.DimensionColumns[i] = NormalizeName(in.Hypertable.DimensionColumns[i])
		}
		for i := range in.Hypertable.DimensionIntervals {
			in.Hypertable.DimensionIntervals[i] = NormalizeInterval(in.Hypertable.DimensionIntervals[i])
		}
	}
	h, err := HashPayload(in)
	if err != nil {
		return FingerprintResult{}, err
	}
	return FingerprintResult{Hash: h, AlgorithmVersion: AlgorithmVersion}, nil
}

// FunctionFingerprint builds a stable hash for a function/procedure.
func FunctionFingerprint(in FunctionFingerprintInput) (FingerprintResult, error) {
	in.Schema = NormalizeName(in.Schema)
	in.Name = NormalizeName(in.Name)
	in.Args = strings.TrimSpace(strings.ToLower(in.Args))
	in.Language = NormalizeName(in.Language)
	in.Kind = NormalizeName(in.Kind)
	in.Volatility = NormalizeName(in.Volatility)
	in.Definition = NormalizeSQL(in.Definition)
	h, err := HashPayload(in)
	if err != nil {
		return FingerprintResult{}, err
	}
	return FingerprintResult{Hash: h, AlgorithmVersion: AlgorithmVersion}, nil
}

// ViewFingerprint builds a stable hash for a view or matview.
func ViewFingerprint(in ViewFingerprintInput) (FingerprintResult, error) {
	in.Schema = NormalizeName(in.Schema)
	in.Name = NormalizeName(in.Name)
	in.Relkind = NormalizeName(in.Relkind)
	in.Definition = NormalizeSQL(in.Definition)
	h, err := HashPayload(in)
	if err != nil {
		return FingerprintResult{}, err
	}
	return FingerprintResult{Hash: h, AlgorithmVersion: AlgorithmVersion}, nil
}

// HypertableFingerprint is a convenience wrapper around table + hypertable parts.
func HypertableFingerprint(table TableFingerprintInput, ht HypertableFingerprintPart) (FingerprintResult, error) {
	table.Hypertable = &ht
	return TableFingerprint(table)
}
