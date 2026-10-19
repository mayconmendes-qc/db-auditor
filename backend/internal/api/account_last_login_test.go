package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
)

type lastLoginStubStore struct {
	stubStore
	envTokenAuthStore
	upserted   []repository.AccountLastLoginInput
	envSeen    string
	tokenSeen  *string
	list       []repository.AccountLastLoginObservation
	failUpsert error
}

func (s *lastLoginStubStore) UpsertAccountLastLogins(_ context.Context, env string, tokenID *string, entries []repository.AccountLastLoginInput) (int, error) {
	if s.failUpsert != nil {
		return 0, s.failUpsert
	}
	s.envSeen = env
	s.tokenSeen = tokenID
	s.upserted = append([]repository.AccountLastLoginInput(nil), entries...)
	return len(entries), nil
}

func (s *lastLoginStubStore) ListAccountLastLogins(_ context.Context, env string) ([]repository.AccountLastLoginObservation, error) {
	out := make([]repository.AccountLastLoginObservation, 0)
	for _, o := range s.list {
		if o.EnvironmentID == env {
			out = append(out, o)
		}
	}
	return out, nil
}

func lastLoginPlaintext() string {
	p := "dba_llogin01_" + base64.RawURLEncoding.EncodeToString(sha256.New().Sum(nil))[:32]
	if len(p) > 128 {
		p = p[:120]
	}
	return p
}

func TestLastLoginIngestOnlyRoleAndTime(t *testing.T) {
	env := "11111111-1111-1111-1111-111111111111"
	plaintext := lastLoginPlaintext()
	store := &lastLoginStubStore{
		envTokenAuthStore: envTokenAuthStore{
			tokens: map[string]*repository.EnvTokenPrincipal{
				plaintext: {
					ID: "tok-ll", EnvironmentID: env, PublicPrefix: "llogin01",
					Scopes: []string{"last-login:write"},
				},
			},
		},
	}
	mux := http.NewServeMux()
	registerAccountLastLoginRoutes(mux, store)
	h := authMiddleware(mux, store)

	// Extra field on entry → 400, nothing stored.
	bodyExtra := `{"entries":[{"role":"app_ro","last_login_at":"2026-10-19T12:00:00Z","ip":"1.2.3.4"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/account-last-logins", strings.NewReader(bodyExtra))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("extra field status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.upserted) != 0 {
		t.Fatalf("must not store on extra field: %+v", store.upserted)
	}

	// Top-level extra field → 400.
	bodyTop := `{"entries":[{"role":"app_ro","last_login_at":"2026-10-19T12:00:00Z"}],"database":"db1"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/account-last-logins", strings.NewReader(bodyTop))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("top extra status=%d", rec.Code)
	}

	// Valid body.
	bodyOK := `{"schema_version":"1","entries":[{"role":"app_ro","last_login_at":"2026-10-19T12:00:00Z"}]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/account-last-logins", strings.NewReader(bodyOK))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ok status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.upserted) != 1 || store.upserted[0].Role != "app_ro" {
		t.Fatalf("upserted=%+v", store.upserted)
	}
	if store.tokenSeen == nil || *store.tokenSeen != "tok-ll" {
		t.Fatalf("token id=%v", store.tokenSeen)
	}
	raw, _ := json.Marshal(store.upserted[0])
	var probe map[string]json.RawMessage
	_ = json.Unmarshal(raw, &probe)
	for k := range probe {
		if k != "role" && k != "last_login_at" {
			t.Fatalf("stored extra field %s", k)
		}
	}
}

func TestLastLoginTokenOtherEnvironment403(t *testing.T) {
	envA := "11111111-1111-1111-1111-111111111111"
	envB := "22222222-2222-2222-2222-222222222222"
	plaintext := lastLoginPlaintext()
	store := &lastLoginStubStore{
		envTokenAuthStore: envTokenAuthStore{
			tokens: map[string]*repository.EnvTokenPrincipal{
				plaintext: {
					ID: "tok-a", EnvironmentID: envA, PublicPrefix: "llogin01",
					Scopes: []string{"last-login:write"},
				},
			},
		},
	}
	mux := http.NewServeMux()
	registerAccountLastLoginRoutes(mux, store)
	h := authMiddleware(mux, store)
	body := `{"entries":[{"role":"r","last_login_at":"2026-10-19T12:00:00Z"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+envB+"/account-last-logins", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.upserted) != 0 {
		t.Fatal("must not upsert on cross-env")
	}
}

func TestLastLoginMissingScope403(t *testing.T) {
	env := "11111111-1111-1111-1111-111111111111"
	plaintext := lastLoginPlaintext()
	store := &lastLoginStubStore{
		envTokenAuthStore: envTokenAuthStore{
			tokens: map[string]*repository.EnvTokenPrincipal{
				plaintext: {
					ID: "tok-ns", EnvironmentID: env, PublicPrefix: "llogin01",
					Scopes: []string{"snapshot-context:read"},
				},
			},
		},
	}
	mux := http.NewServeMux()
	registerAccountLastLoginRoutes(mux, store)
	h := authMiddleware(mux, store)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/account-last-logins",
		bytes.NewReader([]byte(`{"entries":[{"role":"r","last_login_at":"2026-10-19T12:00:00Z"}]}`)))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthorizeEnvTokenLastLoginWrite(t *testing.T) {
	tok := &repository.EnvTokenPrincipal{
		ID: "t", EnvironmentID: "11111111-1111-1111-1111-111111111111",
		Scopes: []string{"last-login:write"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+tok.EnvironmentID+"/account-last-logins", nil)
	env, ok, scope := authorizeEnvToken(req, tok)
	if !ok || env != tok.EnvironmentID || scope != "last-login:write" {
		t.Fatalf("ok=%v env=%s scope=%s", ok, env, scope)
	}
	_ = time.Now()
}

func TestDecodeAccountLastLoginRejectsApplicationField(t *testing.T) {
	_, err := decodeAccountLastLoginBody([]byte(`{"entries":[{"role":"a","last_login_at":"2026-01-01T00:00:00Z","application":"x"}]}`))
	if err == nil || !strings.Contains(err.Error(), "campo não permitido") {
		t.Fatalf("err=%v", err)
	}
}
