package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
)

type envTokenAuthStore struct {
	fakeAuthStore
	tokens map[string]*repository.EnvTokenPrincipal // plaintext -> principal
	denied []string
}

func (s *envTokenAuthStore) ResolveEnvAPIToken(_ context.Context, plaintext string) (*repository.EnvTokenPrincipal, error) {
	if p, ok := s.tokens[plaintext]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, nil
}

func (s *envTokenAuthStore) LogEnvTokenOperation(_ context.Context, tokenID, action, environmentID, _, result string) error {
	if result == "denied" {
		s.denied = append(s.denied, tokenID+"|"+environmentID+"|"+action)
	}
	return nil
}

func TestEnvTokenCrossEnvironmentForbidden(t *testing.T) {
	envA := "11111111-1111-1111-1111-111111111111"
	envB := "22222222-2222-2222-2222-222222222222"
	plaintext := "dba_abcd1234_" + strings.Repeat("x", 32)
	store := &envTokenAuthStore{
		tokens: map[string]*repository.EnvTokenPrincipal{
			plaintext: {
				ID: "tok-a", EnvironmentID: envA, PublicPrefix: "abcd1234",
				Scopes: []string{"snapshot-context:read"},
			},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/environments/{id}/migration-context", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tables":[]}`))
	})
	h := authMiddleware(mux, store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+envB+"/migration-context", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.denied) == 0 || !strings.Contains(store.denied[0], "tok-a") {
		t.Fatalf("expected denied log with token id: %#v", store.denied)
	}
}

func TestEnvTokenExpiredOrMissingScope(t *testing.T) {
	envA := "11111111-1111-1111-1111-111111111111"
	// Resolve returns nil for expired/revoked — middleware maps to 401.
	store := &envTokenAuthStore{tokens: map[string]*repository.EnvTokenPrincipal{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/environments/{id}/migration-context", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := authMiddleware(mux, store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+envA+"/migration-context", nil)
	req.Header.Set("Authorization", "Bearer dba_deadbeef_"+strings.Repeat("y", 32))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestReportTokenNotAcceptedForMigrationContext(t *testing.T) {
	envA := "11111111-1111-1111-1111-111111111111"
	report := strings.Repeat("r", 32)
	t.Setenv("AUDITOR_REPORT_API_TOKEN", report)
	store := &envTokenAuthStore{tokens: map[string]*repository.EnvTokenPrincipal{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/environments/{id}/migration-context", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := authMiddleware(mux, store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+envA+"/migration-context", nil)
	req.Header.Set("Authorization", "Bearer "+report)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("report token must not authorize migration-context")
	}
}

func TestEnvTokenAllowsMigrationContextWithScope(t *testing.T) {
	envA := "11111111-1111-1111-1111-111111111111"
	plaintext := "dba_okprefix_" + base64.RawURLEncoding.EncodeToString(sha256.New().Sum(nil))[:32]
	if len(plaintext) > 128 {
		plaintext = plaintext[:120]
	}
	store := &envTokenAuthStore{
		tokens: map[string]*repository.EnvTokenPrincipal{
			plaintext: {
				ID: "tok-ok", EnvironmentID: envA, PublicPrefix: "okprefix",
				Scopes: []string{"snapshot-context:read", "migration-analysis:write"},
			},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/environments/{id}/migration-context", func(w http.ResponseWriter, r *http.Request) {
		if requestEnvToken(r) == nil {
			t.Fatal("expected env token principal")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tables":[]}`))
	})
	h := authMiddleware(mux, store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+envA+"/migration-context", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorizeEnvTokenClosedList(t *testing.T) {
	tok := &repository.EnvTokenPrincipal{
		ID: "t", EnvironmentID: "11111111-1111-1111-1111-111111111111",
		Scopes: []string{"snapshot-context:read"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+tok.EnvironmentID+"/databases", nil)
	if _, ok, _ := authorizeEnvToken(req, tok); ok {
		t.Fatal("databases must be deny-by-default for env tokens")
	}
	_ = time.Now()
}
