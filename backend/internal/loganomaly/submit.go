package loganomaly

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

// LastLoginSubmitBody is the closed POST body (ADR-0001 D10): only entries of role + last_login_at.
type LastLoginSubmitBody struct {
	SchemaVersion string           `json:"schema_version"`
	Entries       []LastLoginEntry `json:"entries"`
}

// SubmitLastLoginPOSTer abstracts the HTTP client (cli.APIClient or tests).
type SubmitLastLoginPOSTer interface {
	Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error)
}

// BuildLastLoginSubmitBody copies only role and last_login_at into a closed struct.
func BuildLastLoginSubmitBody(f LastLoginFile) LastLoginSubmitBody {
	entries := make([]LastLoginEntry, 0, len(f.Entries))
	for _, e := range f.Entries {
		entries = append(entries, LastLoginEntry{
			Role:        strings.TrimSpace(e.Role),
			LastLoginAt: strings.TrimSpace(e.LastLoginAt),
		})
	}
	sv := f.SchemaVersion
	if sv == "" {
		sv = SchemaVersion
	}
	return LastLoginSubmitBody{SchemaVersion: sv, Entries: entries}
}

// SubmitLastLogin posts the aggregate to POST .../account-last-logins.
// Never includes templates, counts, IP, database, or application.
func SubmitLastLogin(ctx context.Context, client SubmitLastLoginPOSTer, environmentID string, f LastLoginFile) error {
	if client == nil {
		return fmt.Errorf("cliente HTTP ausente")
	}
	environmentID = strings.TrimSpace(environmentID)
	if environmentID == "" {
		return fmt.Errorf("environment obrigatório")
	}
	body := BuildLastLoginSubmitBody(f)
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	// Defence in depth: refuse if marshalled JSON gained unexpected keys.
	if err := ValidateLastLoginOnlyRoleAndTime(raw); err != nil {
		return err
	}
	path := "/api/v1/environments/" + environmentID + "/account-last-logins"
	resp, err := client.Do(ctx, http.MethodPost, path, bytes.NewReader(raw))
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

// SubmitLastLoginHTTP is a thin http.Client adapter for tests without cli import cycles.
type SubmitLastLoginHTTP struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func (c *SubmitLastLoginHTTP) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 30 * time.Second}
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.Client.Do(req)
}
