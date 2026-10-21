package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSendToDestinationsWebhookFailureThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		for _, bad := range []string{"SELECT ", "INSERT INTO", "password", "postgres://"} {
			if strings.Contains(string(body), bad) {
				t.Errorf("payload leaked %q: %s", bad, body)
			}
		}
		if n == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	event := Event{
		Kind: "query_digest", EnvironmentID: "00000000-0000-0000-0000-000000000001",
		DedupKey: "digest-1", Title: "Digest de consultas (ok)", Severity: "info",
	}
	payload := DigestPayload{
		Event: event, Link: "/digest?env=e&id=d", RegressionCount: 2, State: "ok",
	}
	dests := []Destination{{
		ID: "dest-1", Channel: "webhook", Target: srv.URL, Format: "generic",
	}}

	errs := SendToDestinations(context.Background(), dests, event, payload)
	if len(errs) != 1 || errs[0] == nil {
		t.Fatalf("first attempt want error, got %v", errs)
	}

	errs = SendToDestinations(context.Background(), dests, event, payload)
	if len(errs) != 1 || errs[0] != nil {
		t.Fatalf("retry want success, got %v", errs)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}

	raw, _ := json.Marshal(payload)
	if strings.Contains(string(raw), "SELECT ") {
		t.Fatal("DigestPayload must not carry SQL")
	}
}

func TestGlobalFallbackChannels(t *testing.T) {
	cfg := Config{WebhookURL: "https://hooks.example.com/x", EmailTo: "ops@example.com", SMTPHost: "smtp.example.com"}
	ch := cfg.GlobalFallbackChannels()
	if len(ch) != 2 {
		t.Fatalf("channels=%v", ch)
	}
	empty := Config{}
	if len(empty.GlobalFallbackChannels()) != 0 {
		t.Fatal("disabled config must report no global channels")
	}
}
