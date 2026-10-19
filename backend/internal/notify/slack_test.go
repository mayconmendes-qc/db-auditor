package notify

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatSlackHasNoSQL(t *testing.T) {
	event := Event{
		Kind: "drift.out_of_pipeline", EnvironmentID: "env-1",
		DedupKey: "k", Title: "Índice criado fora do pipeline", Severity: "medium",
	}
	payload := FormatSlack(event, "index", "db.public.idx_manual", "added", "https://auditor.example/schema-timeline")
	raw, _ := json.Marshal(payload)
	body := string(raw)
	for _, bad := range []string{"CREATE INDEX", "SELECT ", "password", "postgres://"} {
		if strings.Contains(body, bad) {
			t.Fatalf("leaked %q in %s", bad, body)
		}
	}
	if !strings.Contains(body, "idx_manual") || !strings.Contains(payload.Text, "Índice") {
		t.Fatalf("missing expected fields: %s", body)
	}
}

func TestBodyForDestinationPerFormat(t *testing.T) {
	event := Event{Kind: "drift.out_of_pipeline", EnvironmentID: "e", Title: "t", DedupKey: "d"}
	drift := DriftPayload{Event: event, ObjectType: "index", ObjectKey: "public.i", ChangeKind: "added"}
	generic := BodyForDestination(Destination{Channel: "webhook", Format: "generic"}, event, &drift)
	if _, ok := generic.(DriftPayload); !ok {
		t.Fatalf("generic want DriftPayload got %T", generic)
	}
	slack := BodyForDestination(Destination{Channel: "webhook", Format: "slack"}, event, &drift)
	if _, ok := slack.(SlackPayload); !ok {
		t.Fatalf("slack want SlackPayload got %T", slack)
	}
}

func TestExistingEventJSONUnchangedShape(t *testing.T) {
	event := Event{Kind: "job.unhealthy", EnvironmentID: "e", DedupKey: "d", Title: "job", Severity: "high"}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"kind", "environment_id", "dedup_key", "title", "severity"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing %s", k)
		}
	}
	if len(m) != 5 {
		t.Fatalf("unexpected keys %v", m)
	}
}
