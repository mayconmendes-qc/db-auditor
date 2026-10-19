package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SchemaVersion tags CLI JSON/Markdown payloads so consumers can evolve safely.
const SchemaVersion = "1"

// WriteJSON encodes value as JSON with a trailing newline.
func WriteJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return err
	}
	return nil
}

// WriteMarkdown writes a Markdown document already assembled by the caller.
func WriteMarkdown(w io.Writer, doc string) error {
	if !strings.HasSuffix(doc, "\n") {
		doc += "\n"
	}
	_, err := io.WriteString(w, doc)
	return err
}

// FormatFlag validates --format md|json.
func FormatFlag(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "md", "markdown":
		return "md", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("formato inválido %q (use md ou json)", raw)
	}
}
