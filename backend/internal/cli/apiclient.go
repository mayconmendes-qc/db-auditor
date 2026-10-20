package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/config"
)

// APIClient talks to the auditor HTTP API with verified TLS and a hard timeout.
type APIClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewAPIClient builds a client. InsecureSkipVerify is never enabled.
func NewAPIClient(baseURL, token string, timeout time.Duration) *APIClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &APIClient{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
	}
}

// Do performs an authenticated request. The token is never logged.
func (c *APIClient) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("URL da API ausente (AUDITOR_API_URL)")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body) //nolint:gosec // G704: BaseURL is operator AUDITOR_API_URL; path is a fixed CLI route.
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req) //nolint:gosec // G704: request URL built from operator-configured BaseURL.
	if err != nil {
		return nil, fmt.Errorf("%s", config.SanitizeError(err))
	}
	return resp, nil
}

// MapHTTPStatusToExit translates common API auth failures to CLI exit codes.
func MapHTTPStatusToExit(status int) int {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ExitUsage
	case http.StatusRequestEntityTooLarge:
		return ExitUsage
	case http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
		return ExitInsufficient
	default:
		if status >= 500 {
			return ExitInsufficient
		}
		return ExitUsage
	}
}
