package dqcontract

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// YAMLError carries line/field for API 400 responses.
type YAMLError struct {
	Line  int
	Field string
	Msg   string
}

func (e *YAMLError) Error() string {
	if e.Line > 0 && e.Field != "" {
		return fmt.Sprintf("linha %d, campo %s: %s", e.Line, e.Field, e.Msg)
	}
	if e.Line > 0 {
		return fmt.Sprintf("linha %d: %s", e.Line, e.Msg)
	}
	if e.Field != "" {
		return fmt.Sprintf("campo %s: %s", e.Field, e.Msg)
	}
	return e.Msg
}

const maxYAMLBytes = 1 << 20 // 1 MiB

// ParseYAML decodes a contract document from YAML with line/field errors.
func ParseYAML(raw []byte, processMaxSample int) (Document, string, error) {
	if len(raw) == 0 {
		return Document{}, "", &YAMLError{Msg: "YAML vazio"}
	}
	if len(raw) > maxYAMLBytes {
		return Document{}, "", &YAMLError{Msg: "YAML excede 1 MiB"}
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return Document{}, "", mapYAMLErr(err, "")
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return Document{}, "", &YAMLError{Msg: "documento YAML inválido"}
	}
	docNode := root.Content[0]
	var doc Document
	if err := docNode.Decode(&doc); err != nil {
		return Document{}, "", mapYAMLErr(err, "")
	}
	NormalizeSampleLimit(&doc)
	if err := ValidateDocument(doc, processMaxSample); err != nil {
		return Document{}, "", &YAMLError{Field: guessField(err.Error()), Msg: err.Error()}
	}
	normalized, err := MarshalYAML(doc)
	if err != nil {
		return Document{}, "", err
	}
	return doc, normalized, nil
}

// MarshalYAML emits a stable YAML representation.
func MarshalYAML(doc Document) (string, error) {
	NormalizeSampleLimit(&doc)
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("falha ao serializar YAML: %w", err)
	}
	return string(out), nil
}

func mapYAMLErr(err error, field string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	line := 0
	// yaml.v3 TypeError / SyntaxError often embed "line N:"
	if i := strings.Index(msg, "line "); i >= 0 {
		var n int
		if _, scanErr := fmt.Sscanf(msg[i:], "line %d", &n); scanErr == nil {
			line = n
		}
	}
	return &YAMLError{Line: line, Field: field, Msg: msg}
}

func guessField(msg string) string {
	for _, f := range []string{
		"sample_limit", "schedule_profile", "database", "schema", "table",
		"max_null_pct", "max_age_hours", "max_change_pct", "tolerance_pct",
		"checks", "column", "columns", "allowed", "type",
	} {
		if strings.Contains(msg, f) {
			return f
		}
	}
	return ""
}
