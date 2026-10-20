package restoreverify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type DrillContext struct {
	SourceMajor   int                 `json:"source_major"`
	AuditRunID    string              `json:"audit_run_id,omitempty"`
	Fingerprints  []string            `json:"target_fingerprints"`
	Inventory     []InventoryObject   `json:"inventory"`
	EstimatedRows []TableEstimate     `json:"estimated_rows"`
	WriteCounters []TableWriteCounter `json:"write_counters"`
	Partial       bool                `json:"partial"`
}

func SubmitEvidence(ctx context.Context, apiURL, token, environmentID string, ev Evidence, client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/v1/environments/"+environmentID+"/restore-drills", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("API status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func FetchDrillContext(ctx context.Context, apiURL, token, environmentID string, client *http.Client) (*DrillContext, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/api/v1/environments/"+environmentID+"/restore-drill-context", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("contexto status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out DrillContext
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func RenderSummary(ev Evidence) string {
	rto := "n/d"
	if ev.RTOSeconds != nil {
		rto = fmt.Sprintf("%d s", *ev.RTOSeconds)
	}
	status := "sucesso"
	if !ev.Succeeded {
		status = "falha"
	}
	return fmt.Sprintf("ensaio %s: RTO %s, %d verificação(ões), %d divergência(s)", status, rto, len(ev.Checks), len(ev.Divergences))
}

func RefuseRowValues(raw json.RawMessage) error {
	s := string(raw)
	for _, banned := range []string{`"row_values"`, `"sample_rows"`, `"literal_values"`} {
		if strings.Contains(s, banned) {
			return fmt.Errorf("evidência contém campo de valor proibido")
		}
	}
	return nil
}
