package report

import "strings"

// RedactMetadata removes selected PDF header fields without mutating stored
// inventory or changing the report's underlying findings.
func (d *Document) RedactMetadata(fields string) {
	for _, raw := range strings.Split(fields, ",") {
		switch strings.TrimSpace(strings.ToLower(raw)) {
		case "environment":
			d.Environment = "[oculto]"
		case "run_id":
			d.RunID = "[oculto]"
		case "requested_by":
			d.RequestedBy = "[oculto]"
		case "service_version":
			d.ServiceVersion = "[oculto]"
		case "rule_version":
			d.RuleVersion = "[oculto]"
		}
	}
}
