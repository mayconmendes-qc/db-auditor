package report

import "testing"

func TestRedactMetadata(t *testing.T) {
	d := Document{Environment: "production", RunID: "run-123", RequestedBy: "alice", ServiceVersion: "1.2", RuleVersion: "3.4"}
	d.RedactMetadata("environment,requested_by")
	if d.Environment != "[oculto]" || d.RequestedBy != "[oculto]" {
		t.Fatal("selected metadata was not redacted")
	}
	if d.RunID != "run-123" || d.ServiceVersion != "1.2" {
		t.Fatal("unselected metadata was changed")
	}
}
